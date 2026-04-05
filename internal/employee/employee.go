package employee

type (
	EmployeeData struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Surname  string `json:"surname"`
		Password string `json:"password"`
	}
	Employee struct {
		ID       int
		Name     string
		Email    string
		Surname  string
		Password string
	}
)
