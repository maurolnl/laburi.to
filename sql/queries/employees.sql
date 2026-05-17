-- name: CreateEmployee :one
INSERT INTO employees(position, role, years_of_experience, certifications, portfolio_url,created_at,updated_at )
VALUES(
  $1,
  $2,
  $3,
  $4,
  $5,
  NOW(),
  NOW()
)
RETURNING *;

-- name: GetEmployee :one
SELECT * FROM employees WHERE id = $1;
