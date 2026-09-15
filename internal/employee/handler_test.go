package employee

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

type multipartFilePart struct {
	name        string
	filename    string
	contentType string
	content     string
}

func newEmployeeMultipartRequest(t *testing.T, fields map[string]string, files []multipartFilePart) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}

	for _, file := range files {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", `form-data; name="`+file.name+`"; filename="`+file.filename+`"`)
		header.Set("Content-Type", file.contentType)
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(file.content)); err != nil {
			t.Fatal(err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/employees", bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func newTestHandler(t *testing.T) (*EmployeeHandler, *fakeEmployeeService) {
	t.Helper()
	fake := &fakeEmployeeService{}
	validate := validator.New()
	return NewHandler(fake, validate), fake
}

func makeToken(t *testing.T, secret string, userID int32, role user.UserRole) string {
	t.Helper()
	tok, err := auth.MakeJWT(userID, role, secret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func makeEmployeeToken(t *testing.T, secret string, userID int32) string {
	t.Helper()
	return makeToken(t, secret, userID, user.UserRoleEmployee)
}

func makeEmployerToken(t *testing.T, secret string, userID int32) string {
	t.Helper()
	return makeToken(t, secret, userID, user.UserRoleEmployer)
}

func TestCreateEmployee(t *testing.T) {
	secret := "test-secret"

	tests := []struct {
		name          string
		authorization string
		fields        map[string]string
		files         []multipartFilePart
		expectedCode  int
		expectCall    bool
		expectedUser  int32
		expectedRole  user.UserRole
		expectedFile  bool
		serviceErr    error
	}{
		{
			name:         "missing authentication",
			fields:       map[string]string{"position": "Dev", "role": "Backend", "years_of_experience": string(Years2To5Y)},
			expectedCode: http.StatusUnauthorized,
			expectCall:   false,
		},
		{
			name:          "employer is forbidden",
			authorization: "Bearer " + makeEmployerToken(t, secret, 7),
			fields:        map[string]string{"position": "Dev", "role": "Backend", "years_of_experience": string(Years2To5Y)},
			expectedCode:  http.StatusForbidden,
			expectCall:    true,
			expectedUser:  7,
			expectedRole:  user.UserRoleEmployer,
			serviceErr:    user.ErrProfileRoleForbidden,
		},
		{
			name:          "valid multipart without file",
			authorization: "Bearer " + makeEmployeeToken(t, secret, 7),
			fields:        map[string]string{"position": "Dev", "role": "Backend", "years_of_experience": string(Years2To5Y), "certifications": `["aws"]`, "user_id": "999"},
			expectedCode:  http.StatusCreated,
			expectCall:    true,
			expectedUser:  7,
			expectedRole:  user.UserRoleEmployee,
			expectedFile:  false,
		},
		{
			name:          "valid multipart with singular certifications_file",
			authorization: "Bearer " + makeEmployeeToken(t, secret, 7),
			fields:        map[string]string{"position": "Dev", "role": "Backend", "years_of_experience": string(Years2To5Y)},
			files: []multipartFilePart{
				{name: "certifications_file", filename: "cert.pdf", contentType: "application/pdf", content: "%PDF-1.4"},
			},
			expectedCode: http.StatusCreated,
			expectCall:   true,
			expectedUser: 7,
			expectedRole: user.UserRoleEmployee,
			expectedFile: true,
		},
		{
			name:          "rejects multiple certifications_file parts",
			authorization: "Bearer " + makeEmployeeToken(t, secret, 7),
			fields:        map[string]string{"position": "Dev", "role": "Backend", "years_of_experience": string(Years2To5Y)},
			files: []multipartFilePart{
				{name: "certifications_file", filename: "one.pdf", contentType: "application/pdf", content: "%PDF"},
				{name: "certifications_file", filename: "two.pdf", contentType: "application/pdf", content: "%PDF"},
			},
			expectedCode: http.StatusBadRequest,
			expectCall:   false,
		},
		{
			name:          "rejects non-pdf file",
			authorization: "Bearer " + makeEmployeeToken(t, secret, 7),
			fields:        map[string]string{"position": "Dev", "role": "Backend", "years_of_experience": string(Years2To5Y)},
			files: []multipartFilePart{
				{name: "certifications_file", filename: "cert.txt", contentType: "text/plain", content: "text"},
			},
			expectedCode: http.StatusBadRequest,
			expectCall:   false,
		},
		{
			name:          "rejects PDF larger than five megabytes",
			authorization: "Bearer " + makeEmployeeToken(t, secret, 7),
			fields:        map[string]string{"position": "Dev", "role": "Backend", "years_of_experience": string(Years2To5Y)},
			files: []multipartFilePart{
				{name: "certifications_file", filename: "large.pdf", contentType: "application/pdf", content: strings.Repeat("x", maxUploadSize+1)},
			},
			expectedCode: http.StatusBadRequest,
			expectCall:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, fake := newTestHandler(t)
			fake.createEmployeeErr = tt.serviceErr

			req := newEmployeeMultipartRequest(t, tt.fields, tt.files)
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}
			rec := httptest.NewRecorder()

			var handler http.Handler = user.AuthenticatedUser(secret)(http.HandlerFunc(h.CreateEmployee))

			handler.ServeHTTP(rec, req)
			if req.MultipartForm != nil {
				t.Cleanup(func() { _ = req.MultipartForm.RemoveAll() })
			}

			if rec.Code != tt.expectedCode {
				t.Fatalf("expected status %d, got %d: %s", tt.expectedCode, rec.Code, rec.Body.String())
			}

			fake.mu.Lock()
			calls := len(fake.createEmployeeCalls)
			fake.mu.Unlock()

			if calls > 0 != tt.expectCall {
				t.Fatalf("expected service call=%v, got %v", tt.expectCall, calls > 0)
			}

			if tt.expectCall {
				fake.mu.Lock()
				call := fake.createEmployeeCalls[0]
				fake.mu.Unlock()

				if call.Principal.UserID != tt.expectedUser {
					t.Fatalf("expected userID %d, got %d", tt.expectedUser, call.Principal.UserID)
				}
				if call.Principal.Role != tt.expectedRole {
					t.Fatalf("expected role %q, got %q", tt.expectedRole, call.Principal.Role)
				}
				if call.Req.Position != tt.fields["position"] {
					t.Fatalf("expected position %q, got %q", tt.fields["position"], call.Req.Position)
				}
				if (call.File != nil) != tt.expectedFile {
					t.Fatalf("expected file present=%v, got %v", tt.expectedFile, call.File != nil)
				}
				if tt.expectedFile {
					if call.Filename != "cert.pdf" || call.ContentType != "application/pdf" || call.Size != 8 {
						t.Fatalf("unexpected file metadata: filename=%s contentType=%s size=%d", call.Filename, call.ContentType, call.Size)
					}
				}
			}
		})
	}
}

func TestUpdateEmployee(t *testing.T) {
	h, fake := newTestHandler(t)

	fields := map[string]string{"position": "Senior Dev", "role": "Backend", "years_of_experience": string(Years5To10Y)}
	files := []multipartFilePart{
		{name: "certifications_file", filename: "updated.pdf", contentType: "application/pdf", content: "%PDF-1.5"},
	}
	req := newEmployeeMultipartRequest(t, fields, files)
	req.SetPathValue("employeeID", "3")
	req.Method = "PUT"

	rec := httptest.NewRecorder()
	h.UpdateEmployee(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	fake.mu.Lock()
	calls := len(fake.updateEmployeeCalls)
	if calls != 1 {
		t.Fatalf("expected 1 update call, got %d", calls)
	}
	call := fake.updateEmployeeCalls[0]
	fake.mu.Unlock()

	if call.EmployeeID != 3 {
		t.Fatalf("expected employeeID 3, got %d", call.EmployeeID)
	}
	if call.File == nil {
		t.Fatal("expected file in update call")
	}
	if call.Filename != "updated.pdf" || call.ContentType != "application/pdf" {
		t.Fatalf("unexpected file metadata: filename=%s contentType=%s", call.Filename, call.ContentType)
	}
}

func TestGetEmployee(t *testing.T) {
	secret := "test-secret"

	tests := []struct {
		name          string
		userID        int32
		pathUserID    string
		employee      Employee
		expectedCode  int
		expectService bool
	}{
		{
			name:          "own employee email",
			userID:        5,
			pathUserID:    "5",
			employee:      Employee{ID: 10, UserID: 5, Email: "employee@example.com"},
			expectedCode:  http.StatusOK,
			expectService: true,
		},
		{
			name:          "forbidden other user",
			userID:        5,
			pathUserID:    "6",
			expectedCode:  http.StatusForbidden,
			expectService: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, fake := newTestHandler(t)
			fake.getEmployeeData = tt.employee

			req := httptest.NewRequest("GET", "/users/"+tt.pathUserID+"/employee", nil)
			req.SetPathValue("userID", tt.pathUserID)
			req.Header.Set("Authorization", "Bearer "+makeEmployeeToken(t, secret, tt.userID))
			rec := httptest.NewRecorder()

			user.AuthenticatedUser(secret)(http.HandlerFunc(h.GetEmployee)).ServeHTTP(rec, req)

			if rec.Code != tt.expectedCode {
				t.Fatalf("expected status %d, got %d: %s", tt.expectedCode, rec.Code, rec.Body.String())
			}

			fake.mu.Lock()
			called := fake.getEmployeeCalls > 0
			fake.mu.Unlock()
			if called != tt.expectService {
				t.Fatalf("expected service called=%v, got %v", tt.expectService, called)
			}

			if tt.expectService && !strings.Contains(rec.Body.String(), tt.employee.Email) {
				t.Fatalf("expected response to contain email %q, got %s", tt.employee.Email, rec.Body.String())
			}
		})
	}
}

func TestSectionHandlersCallService(t *testing.T) {
	h, fake := newTestHandler(t)

	locationBody := `{"internet_connections":[{"type":"fiber","speed":"30mb"}],"timezone":"America/Argentina/Buenos_Aires"}`
	techBody := `{"os":"Linux","paid_software":["Adobe"]}`
	availabilityBody := `{"available_hours_per_day":6,"compatible_projects":1,"incompatible_projects":0}`
	educationBody := `{"education_titles":[{"title":"Lic","status":"completed","type":"university"}]}`

	tests := []struct {
		name        string
		method      string
		path        string
		handler     func(http.ResponseWriter, *http.Request)
		body        string
		contentType string
		checkCalls  func() int
		expectedReq any
	}{
		{
			name:        "CreateLocation",
			method:      "POST",
			path:        "/employees/1/location",
			handler:     h.CreateLocation,
			body:        locationBody,
			contentType: "application/json",
			checkCalls:  func() int { fake.mu.Lock(); defer fake.mu.Unlock(); return len(fake.createLocationCalls) },
		},
		{
			name:        "UpdateLocation",
			method:      "PUT",
			path:        "/employees/2/location",
			handler:     h.UpdateLocation,
			body:        locationBody,
			contentType: "application/json",
			checkCalls:  func() int { fake.mu.Lock(); defer fake.mu.Unlock(); return len(fake.updateLocationCalls) },
		},
		{
			name:        "CreateTech",
			method:      "POST",
			path:        "/employees/3/tech",
			handler:     h.CreateTech,
			body:        techBody,
			contentType: "application/json",
			checkCalls:  func() int { fake.mu.Lock(); defer fake.mu.Unlock(); return len(fake.createTechCalls) },
		},
		{
			name:        "UpdateTech",
			method:      "PUT",
			path:        "/employees/4/tech",
			handler:     h.UpdateTech,
			body:        techBody,
			contentType: "application/json",
			checkCalls:  func() int { fake.mu.Lock(); defer fake.mu.Unlock(); return len(fake.updateTechCalls) },
		},
		{
			name:        "CreateAvailability",
			method:      "POST",
			path:        "/employees/5/availability",
			handler:     h.CreateAvailability,
			body:        availabilityBody,
			contentType: "application/json",
			checkCalls:  func() int { fake.mu.Lock(); defer fake.mu.Unlock(); return len(fake.createAvailabilityCalls) },
		},
		{
			name:        "UpdateAvailability",
			method:      "PUT",
			path:        "/employees/6/availability",
			handler:     h.UpdateAvailability,
			body:        availabilityBody,
			contentType: "application/json",
			checkCalls:  func() int { fake.mu.Lock(); defer fake.mu.Unlock(); return len(fake.updateAvailabilityCalls) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.SetPathValue("employeeID", strings.Split(tt.path, "/")[2])
			req.Header.Set("Content-Type", tt.contentType)
			rec := httptest.NewRecorder()

			tt.handler(rec, req)

			if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
				t.Fatalf("expected success status, got %d: %s", rec.Code, rec.Body.String())
			}
			if tt.checkCalls() != 1 {
				t.Fatalf("expected exactly one service call for %s", tt.name)
			}
		})
	}

	t.Run("CreateEducation", func(t *testing.T) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		if err := writer.WriteField("education", educationBody); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest("POST", "/employees/7/education", bytes.NewReader(body.Bytes()))
		req.SetPathValue("employeeID", "7")
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()

		h.CreateEducation(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
		}
		fake.mu.Lock()
		calls := len(fake.createEducationCalls)
		fake.mu.Unlock()
		if calls != 1 {
			t.Fatalf("expected 1 create education call, got %d", calls)
		}
	})

	t.Run("UpdateEducation", func(t *testing.T) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		if err := writer.WriteField("education", educationBody); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest("PUT", "/employees/8/education", bytes.NewReader(body.Bytes()))
		req.SetPathValue("employeeID", "8")
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()

		h.UpdateEducation(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
		fake.mu.Lock()
		calls := len(fake.updateEducationCalls)
		fake.mu.Unlock()
		if calls != 1 {
			t.Fatalf("expected 1 update education call, got %d", calls)
		}
	})

	t.Run("Education rejects declared document without file", func(t *testing.T) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		withMissingDocument := `{"education_titles":[{"title":"Lic","status":"completed","type":"university","document":"education_document_0"}]}`
		if err := writer.WriteField("education", withMissingDocument); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}

		fake.mu.Lock()
		callsBefore := len(fake.updateEducationCalls)
		fake.mu.Unlock()

		req := httptest.NewRequest("PUT", "/employees/8/education", bytes.NewReader(body.Bytes()))
		req.SetPathValue("employeeID", "8")
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()

		h.UpdateEducation(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
		}
		fake.mu.Lock()
		callsAfter := len(fake.updateEducationCalls)
		fake.mu.Unlock()
		if callsAfter != callsBefore {
			t.Fatal("service must not be called when a declared education document is missing")
		}
	})
}

func TestUpdateRoutesRequireAuthentication(t *testing.T) {
	h, _ := newTestHandler(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, h, NewRepository(nil), "test-secret")

	routes := []string{
		"/employees/1",
		"/employees/1/location",
		"/employees/1/tech",
		"/employees/1/availability",
		"/employees/1/education",
	}

	for _, route := range routes {
		t.Run(route, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, route, nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected route %s to require authentication, got %d", route, rec.Code)
			}
		})
	}
}

func TestEmployeeHandlersDoNotExposeInternalErrors(t *testing.T) {
	const internalDetail = "database connection password=secret"
	secret := "test-secret"

	t.Run("create", func(t *testing.T) {
		h, fake := newTestHandler(t)
		fake.createEmployeeErr = errors.New(internalDetail)
		req := newEmployeeMultipartRequest(t, map[string]string{
			"position":            "Dev",
			"role":                "Backend",
			"years_of_experience": string(Years2To5Y),
		}, nil)
		req.Header.Set("Authorization", "Bearer "+makeEmployeeToken(t, secret, 1))
		rec := httptest.NewRecorder()

		user.AuthenticatedUser(secret)(http.HandlerFunc(h.CreateEmployee)).ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), internalDetail) {
			t.Fatalf("unexpected response: status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("update", func(t *testing.T) {
		h, fake := newTestHandler(t)
		fake.updateEmployeeErr = errors.New(internalDetail)
		req := newEmployeeMultipartRequest(t, map[string]string{
			"position":            "Dev",
			"role":                "Backend",
			"years_of_experience": string(Years2To5Y),
		}, nil)
		req.Method = http.MethodPut
		req.SetPathValue("employeeID", "1")
		rec := httptest.NewRecorder()

		h.UpdateEmployee(rec, req)

		if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), internalDetail) {
			t.Fatalf("unexpected response: status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("get", func(t *testing.T) {
		h, fake := newTestHandler(t)
		fake.getEmployeeErr = errors.New(internalDetail)
		req := httptest.NewRequest(http.MethodGet, "/users/1/employee", nil)
		req.SetPathValue("userID", "1")
		req.Header.Set("Authorization", "Bearer "+makeEmployeeToken(t, secret, 1))
		rec := httptest.NewRecorder()

		user.AuthenticatedUser(secret)(http.HandlerFunc(h.GetEmployee)).ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest || strings.Contains(rec.Body.String(), internalDetail) {
			t.Fatalf("unexpected response: status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}
