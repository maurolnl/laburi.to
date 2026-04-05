package employee

import "database/sql"

type EmployeeRepository struct {
	db *sql.DB
}

func NewEmployeeRepository(db *sql.DB) *EmployeeRepository {
	return &EmployeeRepository{db: db}
}

func (r *EmployeeRepository) CreateEmployee(employee *Employee) error {
	_, err := r.db.Exec("INSERT INTO employees (name, email, password) VALUES (?, ?, ?)", employee.Name, employee.Email, employee.Password)
	return err
}

func (r *EmployeeRepository) GetEmployeeById(id int) (*Employee, error) {
	var employee Employee
	err := r.db.QueryRow("SELECT id, name, email, password FROM employees WHERE id = ?", id).Scan(&employee.ID, &employee.Name, &employee.Email, &employee.Password)
	return &employee, err
}
