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

-- name: CreateEmployeeWithoutFile :one
INSERT INTO employees(position, role, years_of_experience, certifications, portfolio_url, user_id, created_at, updated_at)
VALUES($1, $2, $3, $4, $5, $6, NOW(), NOW())
RETURNING id;

-- name: UpdateEmployee :exec
UPDATE employees
SET
  position = $2,
  role = $3,
  years_of_experience = $4,
  certifications = $5,
  portfolio_url = $6,
  updated_at = NOW()
WHERE id = $1;

-- name: CreateEmployeeFile :exec
INSERT INTO employee_files(
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
) VALUES (
  $1,
  $2,
  $3,
  $4,
  $5,
  $6,
  $7,
  $8,
  $9,
  NOW(),
  NOW(),
  NOW()
);

-- name: GetEmployee :one
SELECT
    employees.id,
    employees.position,
    employees.role,
    employees.years_of_experience,
    employees.certifications,
    employees.portfolio_url,
    employees.created_at,
    employees.updated_at,
    employees.user_id,
    users.email,
    employee_location.timezone,
    employee_profile_tech.os,
    employee_profile_tech.paid_software,
    employee_profile_availability.available_hours_per_day,
    employee_profile_availability.compatible_projects,
    employee_profile_availability.incompatible_projects,
    COALESCE((SELECT jsonb_agg(jsonb_build_object('type', type, 'speed', speed)) FROM employee_internet_connections WHERE employee_id = employees.id), '[]'::jsonb)::text AS internet_connections,
    COALESCE((SELECT jsonb_agg(jsonb_build_object('education_type', education_type, 'title', title, 'status', status, 'certification', certification)) FROM employee_education WHERE employee_id = employees.id), '[]'::jsonb)::text AS education,
    COALESCE((SELECT jsonb_agg(jsonb_build_object('title', original_filename)) FROM employee_files WHERE employee_id = employees.id), '[]'::jsonb)::text AS files
FROM employees
JOIN users ON employees.user_id = users.id
LEFT JOIN employee_location ON employee_location.employee_id = employees.id
LEFT JOIN employee_profile_tech ON employee_profile_tech.employee_id = employees.id
LEFT JOIN employee_profile_availability ON employee_profile_availability.employee_id = employees.id
WHERE users.id = $1;

-- name: GetEmployeeByID :one
SELECT id, user_id FROM employees WHERE id = $1;

-- name: CreateEmployeeConnection :one 
INSERT INTO employee_internet_connections(employee_id, type, speed, created_at, updated_at)
VALUES($1, $2, $3, NOW(), NOW())
RETURNING *;

-- name: GetEmployeeConnection :many
SELECT * FROM employee_internet_connections WHERE employee_id = $1;

-- name: CreateEmployeeLocation :one
INSERT INTO employee_location(employee_id, timezone, created_at, updated_at)
VALUES($1, $2, NOW(), NOW()) RETURNING *;

-- name: UpsertEmployeeLocation :one
INSERT INTO employee_location(employee_id, timezone, created_at, updated_at)
VALUES($1, $2, NOW(), NOW())
ON CONFLICT (employee_id) DO UPDATE
SET
  timezone = EXCLUDED.timezone,
  updated_at = NOW()
RETURNING *;

-- name: DeleteEmployeeConnections :exec
DELETE FROM employee_internet_connections WHERE employee_id = $1;

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

-- name: UpsertEmployeeProfileTech :one
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
)
ON CONFLICT (employee_id) DO UPDATE
SET
  os = EXCLUDED.os,
  paid_software = EXCLUDED.paid_software,
  updated_at = NOW()
RETURNING *;

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

-- name: UpsertEmployeeProfileAvailability :one
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
)
ON CONFLICT (employee_id) DO UPDATE
SET
  available_hours_per_day = EXCLUDED.available_hours_per_day,
  compatible_projects = EXCLUDED.compatible_projects,
  incompatible_projects = EXCLUDED.incompatible_projects,
  updated_at = NOW()
RETURNING *;

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

-- name: DeleteEmployeeEducation :exec
DELETE FROM employee_education WHERE employee_id = $1;

-- name: GetEmployeeEducation :one
SELECT * FROM employee_education WHERE employee_id = $1;
