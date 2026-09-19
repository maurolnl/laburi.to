-- name: CreateJobPosition :one
INSERT INTO job_positions (
    employer_id,
    position,
    role,
    required_experience,
    required_education_level,
    available_hours_per_day,
    timezone,
    technical_resources
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, employer_id, position, role, required_experience, required_education_level,
          available_hours_per_day, timezone, technical_resources, created_at, updated_at, deleted_at;

-- name: GetActiveJobPositionByID :one
SELECT id, employer_id, position, role, required_experience, required_education_level,
       available_hours_per_day, timezone, technical_resources, created_at, updated_at, deleted_at
FROM job_positions
WHERE id = $1
  AND deleted_at IS NULL;

-- name: ListActiveJobPositionsByEmployer :many
SELECT id, employer_id, position, role, required_experience, required_education_level,
       available_hours_per_day, timezone, technical_resources, created_at, updated_at, deleted_at
FROM job_positions
WHERE employer_id = $1
  AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: UpdateActiveJobPosition :one
UPDATE job_positions
SET position = $2,
    role = $3,
    required_experience = $4,
    required_education_level = $5,
    available_hours_per_day = $6,
    timezone = $7,
    technical_resources = $8,
    updated_at = now()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id, employer_id, position, role, required_experience, required_education_level,
          available_hours_per_day, timezone, technical_resources, created_at, updated_at, deleted_at;

-- name: SoftDeleteJobPosition :one
UPDATE job_positions
SET deleted_at = now(),
    updated_at = now()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id, employer_id, deleted_at;
