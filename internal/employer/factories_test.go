package employer

import (
	"slices"
	"testing"
	"time"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

var testEmployerTime = time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)

type testEmployerOption func(*testEmployerData)

type testEmployerData struct {
	id               int32
	userID           int32
	role             user.UserRole
	name             string
	industry         string
	location         string
	hiringModalities []string
	createdAt        time.Time
	updatedAt        time.Time
}

func defaultTestEmployerData() testEmployerData {
	return testEmployerData{
		id:               7,
		userID:           42,
		role:             user.UserRoleEmployer,
		name:             "Acme",
		industry:         "Software",
		location:         "Remote",
		hiringModalities: []string{"Full time"},
		createdAt:        testEmployerTime,
		updatedAt:        testEmployerTime,
	}
}

func withTestEmployerID(id int32) testEmployerOption {
	return func(data *testEmployerData) { data.id = id }
}

func withTestEmployerUserID(userID int32) testEmployerOption {
	return func(data *testEmployerData) { data.userID = userID }
}

func withTestEmployerRole(role user.UserRole) testEmployerOption {
	return func(data *testEmployerData) { data.role = role }
}

func withTestEmployerName(name string) testEmployerOption {
	return func(data *testEmployerData) { data.name = name }
}

func withTestEmployerIndustry(industry string) testEmployerOption {
	return func(data *testEmployerData) { data.industry = industry }
}

func withTestEmployerLocation(location string) testEmployerOption {
	return func(data *testEmployerData) { data.location = location }
}

func withTestEmployerModalities(modalities []string) testEmployerOption {
	return func(data *testEmployerData) {
		data.hiringModalities = slices.Clone(modalities)
	}
}

func withTestEmployerTimes(createdAt, updatedAt time.Time) testEmployerOption {
	return func(data *testEmployerData) {
		data.createdAt = createdAt
		data.updatedAt = updatedAt
	}
}

func buildTestEmployerData(options ...testEmployerOption) testEmployerData {
	data := defaultTestEmployerData()
	for _, option := range options {
		option(&data)
	}
	data.hiringModalities = slices.Clone(data.hiringModalities)
	return data
}

func newTestCreateEmployerRequest(options ...testEmployerOption) CreateEmployerRequest {
	data := buildTestEmployerData(options...)
	return CreateEmployerRequest{
		Name:             data.name,
		Industry:         data.industry,
		Location:         data.location,
		HiringModalities: slices.Clone(data.hiringModalities),
	}
}

func newTestEmployer(options ...testEmployerOption) Employer {
	data := buildTestEmployerData(options...)
	return Employer{
		ID:               data.id,
		UserID:           data.userID,
		Name:             data.name,
		Industry:         data.industry,
		Location:         data.location,
		HiringModalities: slices.Clone(data.hiringModalities),
		CreatedAt:        data.createdAt,
		UpdatedAt:        data.updatedAt,
	}
}

func newTestDatabaseEmployer(options ...testEmployerOption) database.Employer {
	data := buildTestEmployerData(options...)
	return database.Employer{
		ID:               data.id,
		UserID:           data.userID,
		Name:             data.name,
		Industry:         data.industry,
		Location:         data.location,
		HiringModalities: slices.Clone(data.hiringModalities),
		CreatedAt:        data.createdAt,
		UpdatedAt:        data.updatedAt,
	}
}

func newTestEmployerPrincipal(options ...testEmployerOption) auth.Principal {
	data := buildTestEmployerData(options...)
	return auth.Principal{UserID: data.userID, Role: data.role}
}

func TestEmployerFactoriesDefaultsOverridesAndIndependence(t *testing.T) {
	first := newTestEmployer()
	second := newTestEmployer()
	first.HiringModalities[0] = "Part time"
	if second.HiringModalities[0] != "Full time" {
		t.Fatalf("factory instances share modalities: %#v", second.HiringModalities)
	}

	source := []string{"Seasonal"}
	request := newTestCreateEmployerRequest(
		withTestEmployerName(""),
		withTestEmployerIndustry("Construction"),
		withTestEmployerLocation("On site"),
		withTestEmployerModalities(source),
	)
	source[0] = "Changed"
	if request.Name != "" || request.Industry != "Construction" || request.Location != "On site" || request.HiringModalities[0] != "Seasonal" {
		t.Fatalf("expected controlled invalid request and cloned override, got %#v", request)
	}

	createdAt := testEmployerTime.Add(-time.Hour)
	databaseEmployer := newTestDatabaseEmployer(
		withTestEmployerID(9),
		withTestEmployerUserID(10),
		withTestEmployerModalities(nil),
		withTestEmployerTimes(createdAt, testEmployerTime),
	)
	if databaseEmployer.ID != 9 || databaseEmployer.UserID != 10 || databaseEmployer.HiringModalities != nil || databaseEmployer.CreatedAt != createdAt {
		t.Fatalf("unexpected database employer overrides: %#v", databaseEmployer)
	}

	principal := newTestEmployerPrincipal(withTestEmployerRole(user.UserRoleEmployee))
	if principal.UserID != 42 || principal.Role != user.UserRoleEmployee {
		t.Fatalf("unexpected principal: %#v", principal)
	}
}
