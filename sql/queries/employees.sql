-- name: CreateEmployee :one
WITH new_employee AS (
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
  RETURNING *
) INSERT INTO employee_files(
    employee_id,
    type,
    bucket,
    object_key,
    original_filename,
    content_type,
    size_bytes,
    checksum_sha256,
    status,
    created_at,
    uploaded_at,
    updated_at
) SELECT id, $7, $8, $9, $10, $11, $12, $13, $14, NOW(), NOW(), NOW() FROM 
  new_employee returning employee_id;

-- name: GetEmployee :one
SELECT employees.*, users.email FROM employees JOIN users ON employees.user_id = users.id WHERE employees.id = $1;

