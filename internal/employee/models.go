package employee

import (
	"time"
)

type YearsOfExperience string

const (
	YearsLess1Y  YearsOfExperience = "less_1y"
	Years1Y      YearsOfExperience = "1y"
	Years2To5Y   YearsOfExperience = "2_to_5y"
	Years5To10Y  YearsOfExperience = "5_to_10y"
	YearsMore10Y YearsOfExperience = "more_10y"
)

type (
	BaseEmployeeRequest struct {
		Position          string            `json:"position" validate:"required,min=1"`
		Role              string            `json:"role" validate:"required,min=1"`
		YearsOfExperience YearsOfExperience `json:"years_of_experience" validate:"required,oneof=less_1y 1y 2_to_5y 5_to_10y more_10y"`
		Certifications    []string          `json:"certifications"`
		PortfolioURL      string            `json:"portfolio_url" validate:"omitempty,url"`
	}
	CreateEmployeeRequest struct {
		BaseEmployeeRequest
	}

	InternetConnection struct {
		Type  string `json:"type"`
		Speed string `json:"speed"`
	}

	EducationItem struct {
		EducationType string  `json:"education_type"`
		Title         string  `json:"title"`
		Status        string  `json:"status"`
		Certification *string `json:"certification,omitempty"`
	}

	FileItem struct {
		Title string `json:"title"`
	}

	Employee struct {
		ID                   int32
		UserID               int32
		Email                string
		Position             string
		Role                 string
		YearsOfExperience    string
		Certifications       []string
		PortfolioURL         string
		Timezone             string
		Os                   string
		PaidSoftware         []string
		AvailableHoursPerDay int16
		CompatibleProjects   *int16
		IncompatibleProjects *int16
		InternetConnections  []InternetConnection
		Education            []EducationItem
		Files                []FileItem
		CreatedAt            time.Time
		UpdatedAt            time.Time
	}

	GetEmployeeResponse struct {
		ID                   int32                `json:"id"`
		UserID               int32                `json:"user_id"`
		Email                string               `json:"email"`
		Position             string               `json:"position"`
		Role                 string               `json:"role"`
		YearsOfExperience    string               `json:"years_of_experience"`
		Certifications       []string             `json:"certifications"`
		PortfolioURL         string               `json:"portfolio_url,omitempty"`
		Timezone             string               `json:"timezone"`
		Os                   string               `json:"os"`
		PaidSoftware         []string             `json:"paid_software"`
		AvailableHoursPerDay int16                `json:"available_hours_per_day"`
		CompatibleProjects   *int16               `json:"compatible_projects"`
		IncompatibleProjects *int16               `json:"incompatible_projects"`
		InternetConnections  []InternetConnection `json:"internet_connections"`
		Education            []EducationItem      `json:"education"`
		Files                []FileItem           `json:"files"`
		CreatedAt            time.Time            `json:"created_at"`
		UpdatedAt            time.Time            `json:"updated_at"`
	}

	Timezone struct {
		Name      string `json:"name"`
		Abbrev    string `json:"abbrev"`
		UTCOffset string `json:"utc_offset"`
		IsDST     bool   `json:"is_dst"`
	}

	// ============================================================== Steps 1-5 ==============================================================

	// Step 1
	UpdateEmployeeRequest struct {
		BaseEmployeeRequest
		EmployeeID int32 `json:"employee_id" validate:"required"`
	}

	// Step 2
	EmployeeInternetConnection struct {
		Type  string `json:"type" validate:"required,oneof=fiber wifi coaxial adsl mobile"`
		Speed string `json:"speed" validate:"required,oneof=less_10mb 20mb 30mb 40mb more_50mb"`
	}

	BaseEmployeeLocation struct {
		InternetConnections []EmployeeInternetConnection `json:"internet_connections" validate:"required,min=1"`
		Timezone            string                       `json:"timezone" validate:"required,min=2,max=100"`
	}

	CreateEmployeeLocationRequest struct {
		BaseEmployeeLocation
	}

	// Step 3
	BaseEmployeeTech struct {
		Os           string   `json:"os" validate:"omitempty,min=2,max=50"`
		PaidSoftware []string `json:"paid_software" validate:"omitempty,dive,max=50"`
	}

	CreateEmployeeTechRequest struct {
		BaseEmployeeTech
	}

	// Step 4
	BaseEmployeeProfileAvailability struct {
		AvailableHoursPerDay int `json:"available_hours_per_day" validate:"required,min=1,max=8"`
		CompatibleProjects   int `json:"compatible_projects" validate:"omitempty,min=0"`
		IncompatibleProjects int `json:"incompatible_projects" validate:"omitempty,min=0"`
	}

	CreateEmployeeProfileAvailabilityRequest struct {
		BaseEmployeeProfileAvailability
	}

	// Step 5
	EmployeeEducationTitles struct {
		Title         string  `json:"title" validate:"required,min=2,max=100"`
		Status        string  `json:"status" validate:"required,oneof=completed in-progress"`
		EducationType string  `json:"type" validate:"required,oneof=university postgraduate high-school-orientation tertiary"`
		Document      *string `json:"document" validate:"omitempty"`
	}

	BaseEmployeeEducation struct {
		EducationTitles []EmployeeEducationTitles `json:"education_titles" validate:"required,min=1,dive"`
	}

	CreateEmployeeEducationRequest struct {
		BaseEmployeeEducation
	}
)
