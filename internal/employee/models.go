package employee

import "time"

type YearsOfExperience string

const (
	YearsLess1Y  YearsOfExperience = "less_1y"
	Years1Y      YearsOfExperience = "1y"
	Years2To5Y   YearsOfExperience = "2_to_5y"
	Years5To10Y  YearsOfExperience = "5_to_10y"
	YearsMore10Y YearsOfExperience = "more_10y"
)

type (
	CreateEmployeeRequest struct {
		Position          string            `json:"position" validate:"required,min=1"`
		Role              string            `json:"role" validate:"required,min=1"`
		YearsOfExperience YearsOfExperience `json:"years_of_experience" validate:"required,oneof=less_1y 1y 2_to_5y 5_to_10y more_10y"`
		Certifications    []string          `json:"certifications"`
		PortfolioURL      string            `json:"portfolio_url" validate:"omitempty,url"`
	}

	Employee struct {
		ID                 int32
		Email              string
		Position           string
		Role               string
		YearsOfExperience  string
		Certifications     []string
		CertificationsFile string
		PortfolioURL       string
		CreatedAt          time.Time
		UpdatedAt          time.Time
	}

	GetEmployeeResponse struct {
		ID                 int32
		Email              string
		Position           string
		Role               string
		YearsOfExperience  string
		Certifications     []string
		CertificationsFile string
		PortfolioURL       string
		CreatedAt          time.Time
		UpdatedAt          time.Time
	}

	EmployeeTT struct {
		Position                string   `json:"position"`
		Role                    string   `json:"role"`
		YearsOfExperience       string   `json:"yearsOfExperience"`
		Certifications          []string `json:"certifications"`
		CertificationsFile      string   `json:"certificationsFile"`
		PortfolioURL            string   `json:"portfolioUrl"`
		InternetConnectionType  []string `json:"internet_connection_type"`
		InternetConnectionSpeed []string `json:"internet_connection_speed"`
		Timezone                string   `json:"timezone"`
		Os                      string   `json:"os"`
		PaidSoftware            []string `json:"paid_software"`
		AvailableHoursPerDay    int      `json:"available_hours_perDay"`
		CompatibleProjects      []string `json:"compatible_projects"`
		IncompatibleProjects    []string `json:"incompatible_projects"`
		UniversityTitles        []struct {
			Title         string `json:"title"`
			Status        string `json:"status"`
			Certification string `json:"certification"`
			NonMultiple   string `json:"non_multiple"`
		} `json:"university_titles"`
		PostgraduateTitles []struct {
			Title         string `json:"title"`
			Status        string `json:"status"`
			Certification string `json:"certification"`
		} `json:"postgraduate_titles"`
		StudiesOrientation []struct {
			Title         string `json:"title"`
			Status        string `json:"status"`
			Certification string `json:"certification"`
		} `json:"studies_orientation"`
		TertiaryStudies []struct {
			Title         string `json:"title"`
			Status        string `json:"status"`
			Certification string `json:"certification"`
		} `json:"tertiary_studies"`
	}

	UpdateEmployeeRequest struct {
		Position                string   `json:"position"`
		Role                    string   `json:"role"`
		YearsOfExperience       string   `json:"yearsOfExperience"`
		Certifications          string   `json:"certifications"`
		CertificationsFile      string   `json:"certificationsFile"`
		PortfolioURL            string   `json:"portfolioUrl"`
		InternetConnectionType  []string `json:"internet_connection_type"`
		InternetConnectionSpeed []string `json:"internet_connection_speed"`
		Timezone                string   `json:"timezone"`
		Os                      string   `json:"os"`
		PaidSoftware            []string `json:"paid_software"`
		AvailableHoursPerDay    int      `json:"available_hours_perDay"`
		CompatibleProjects      []string `json:"compatible_projects"`
		IncompatibleProjects    []string `json:"incompatible_projects"`
		UniversityTitles        []struct {
			Title         string `json:"title"`
			Status        string `json:"status"`
			Certification string `json:"certification"`
		} `json:"university_titles"`
		PostgraduateTitles []struct {
			Title         string `json:"title"`
			Status        string `json:"status"`
			Certification string `json:"certification"`
		} `json:"postgraduate_titles"`
		StudiesOrientation []struct {
			Title         string `json:"title"`
			Status        string `json:"status"`
			Certification string `json:"certification"`
		} `json:"studies_orientation"`
		TertiaryStudies []struct {
			Title         string `json:"title"`
			Status        string `json:"status"`
			Certification string `json:"certification"`
		} `json:"tertiary_studies"`
	}
)
