package user

import "testing"

const testPassword = "secret123"

type testUserOption func(*testUserData)

type testUserData struct {
	id             int32
	email          string
	password       string
	hashedPassword string
	role           UserRole
}

func defaultTestUserData() testUserData {
	return testUserData{
		id:             7,
		email:          "employee@example.com",
		password:       testPassword,
		hashedPassword: "hashed-password",
		role:           UserRoleEmployee,
	}
}

func withTestUserID(id int32) testUserOption {
	return func(data *testUserData) { data.id = id }
}

func withTestUserEmail(email string) testUserOption {
	return func(data *testUserData) { data.email = email }
}

func withTestUserPassword(password string) testUserOption {
	return func(data *testUserData) { data.password = password }
}

func withTestUserHash(hash string) testUserOption {
	return func(data *testUserData) { data.hashedPassword = hash }
}

func withTestUserRole(role UserRole) testUserOption {
	return func(data *testUserData) { data.role = role }
}

func buildTestUserData(options ...testUserOption) testUserData {
	data := defaultTestUserData()
	for _, option := range options {
		option(&data)
	}
	return data
}

func newTestCreateUserRequest(options ...testUserOption) CreateUserReq {
	data := buildTestUserData(options...)
	return CreateUserReq{Email: data.email, Password: data.password, Role: data.role}
}

func newTestLoginResult(options ...testUserOption) LoginRes {
	data := buildTestUserData(options...)
	return LoginRes{ID: data.id, Email: data.email, HashedPassword: data.hashedPassword, Role: data.role}
}

func newTestLoginRequest(options ...testUserOption) LoginUserRequest {
	data := buildTestUserData(options...)
	return LoginUserRequest{Email: data.email, Password: data.password}
}

func newTestUser(options ...testUserOption) User {
	data := buildTestUserData(options...)
	return User{ID: data.id, Email: data.email, Role: data.role}
}

func TestUserFactoriesDefaultsRolesAndOverrides(t *testing.T) {
	request := newTestCreateUserRequest()
	if request.Email != "employee@example.com" || request.Password != testPassword || request.Role != UserRoleEmployee {
		t.Fatalf("unexpected request defaults: %#v", request)
	}

	employer := newTestUser(withTestUserID(9), withTestUserEmail("employer@example.com"), withTestUserRole(UserRoleEmployer))
	if employer.ID != 9 || employer.Email != "employer@example.com" || employer.Role != UserRoleEmployer {
		t.Fatalf("unexpected user overrides: %#v", employer)
	}

	login := newTestLoginResult(withTestUserHash("custom-hash"), withTestUserRole(UserRoleEmployer))
	if login.HashedPassword != "custom-hash" || login.Role != UserRoleEmployer {
		t.Fatalf("unexpected login overrides: %#v", login)
	}
	loginRequest := newTestLoginRequest(withTestUserEmail("login@example.com"))
	if loginRequest.Email != "login@example.com" || loginRequest.Password != testPassword {
		t.Fatalf("unexpected login request overrides: %#v", loginRequest)
	}

	invalid := newTestCreateUserRequest(withTestUserRole("admin"), withTestUserPassword("short"))
	if invalid.Role != "admin" || invalid.Password != "short" {
		t.Fatalf("expected controlled invalid request, got %#v", invalid)
	}
}
