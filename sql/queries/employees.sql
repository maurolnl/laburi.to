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
  new_employee RETURNING employee_id;

-- name: GetEmployee :one
SELECT employees.*, users.email FROM employees JOIN users ON employees.user_id = users.id WHERE employees.id = $1;

-- name: CreateEmployeeConnection :one 
INSERT INTO employee_internet_connections(employee_id, type, speed, created_at, updated_at)
VALUES($1, $2, $3, NOW(), NOW())
RETURNING *;

-- name: GetEmployeeConnection :many
SELECT * FROM employee_internet_connections WHERE employee_id = $1;

-- name: CreateEmployeeLocation :one
INSERT INTO employee_location(employee_id, timezone, created_at, updated_at)
VALUES($1, $2, NOW(), NOW()) RETURNING *;

-- name: GetEmployeeLocation :many
SELECT * FROM employee_location WHERE employee_id = $1;

-- name: CreateEmployeeProfileTech :one
INSERT INTO employee_profile_tech(
    employee_id,
    os,
    paid_software,
    created_at,
    updated_at
) VALUES (
  $1,
  $2,
  $3,
  NOW(),
  NOW()
)  RETURNING *;

-- name: GetEmployeeProfileTech :one
SELECT * FROM employee_profile_tech WHERE employee_id = $1 LIMIT 1;

-- name: CreateEmployeeProfileAvailability :one
INSERT INTO employee_profile_availability (
  employee_id,
  available_hours_per_day,
  compatible_projects,
  incompatible_projects,
  created_at,
  updated_at
) VALUES (
  $1,
  $2,
  $3,
  $4,
  NOW(),
  NOW()
) RETURNING *;

-- name: GetEmployeeProfileAvailability :one
SELECT * FROM employee_profile_availability WHERE employee_id = $1;

-- name: CreateEmployeeEducation :one
INSERT INTO employee_education (
  employee_id,
  education_type,
  title,
  status,
  certification,
  created_at,
  updated_at
) VALUES (
  $1,
  $2,
  $3,
  $4,
  $5,
  NOW(),
  NOW()
) RETURNING *;

-- name: GetEmployeeEducation :one
SELECT * FROM employee_education WHERE employee_id = $1;
