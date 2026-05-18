-- name: CreateEmployee :one
INSERT INTO employees(position, role, years_of_experience, certifications, portfolio_url, user_id, created_at, updated_at)
VALUES(
  $1,
  $2,
  $3,
  $4,
  $5,
  $6,
  NOW(),
  NOW()
)
RETURNING *;

-- name: GetEmployee :one
SELECT employees.*, users.email FROM employees JOIN users ON employees.user_id = users.id WHERE employees.id = $1;
