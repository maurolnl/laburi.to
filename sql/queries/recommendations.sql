-- name: CreateRecommendationBatch :one
INSERT INTO recommendation_batches (
    subject_type,
    employee_id,
    job_position_id
)
VALUES ($1, $2, $3)
RETURNING id, subject_type, employee_id, job_position_id, status, created_at, updated_at;

-- name: TransitionRecommendationBatch :one
UPDATE recommendation_batches
SET status = $2,
    updated_at = now()
WHERE id = $1
RETURNING id, subject_type, employee_id, job_position_id, status, created_at, updated_at;

-- El batch vigente es el más reciente cualquiera sea su estado: es el que determina si
-- el sujeto se muestra procesando, completado, vacío o con error.
-- name: GetCurrentBatchByEmployee :one
SELECT id, subject_type, employee_id, job_position_id, status, created_at, updated_at
FROM recommendation_batches
WHERE employee_id = $1
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: GetCurrentBatchByJobPosition :one
SELECT id, subject_type, employee_id, job_position_id, status, created_at, updated_at
FROM recommendation_batches
WHERE job_position_id = $1
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- El conjunto vigente sale del último batch completado, que puede no ser el más
-- reciente: un batch fallido posterior no destruye las recomendaciones anteriores.
-- name: GetLastCompletedBatchByEmployee :one
SELECT id, subject_type, employee_id, job_position_id, status, created_at, updated_at
FROM recommendation_batches
WHERE employee_id = $1
  AND status = 'completed'
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: GetLastCompletedBatchByJobPosition :one
SELECT id, subject_type, employee_id, job_position_id, status, created_at, updated_at
FROM recommendation_batches
WHERE job_position_id = $1
  AND status = 'completed'
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: InsertRecommendation :one
INSERT INTO recommendations (
    batch_id,
    employee_id,
    job_position_id,
    score
)
VALUES ($1, $2, $3, $4)
RETURNING id, batch_id, employee_id, job_position_id, score, created_at;

-- Poda de retención: al completar un batch se descartan los demás batches del mismo
-- sujeto. El ON DELETE CASCADE de recommendations arrastra sus filas.
-- name: DeleteOtherBatchesForEmployee :exec
DELETE FROM recommendation_batches
WHERE employee_id = $1
  AND id <> $2;

-- name: DeleteOtherBatchesForJobPosition :exec
DELETE FROM recommendation_batches
WHERE job_position_id = $1
  AND id <> $2;

-- Los puestos eliminados lógicamente quedan fuera con independencia de que la
-- recomendación se haya generado antes de esa eliminación.
-- name: ListJobRecommendationsForEmployee :many
SELECT r.id, r.batch_id, r.score,
       j.id AS job_position_id,
       j.employer_id,
       j.position,
       j.role,
       j.required_experience,
       j.required_education_level,
       j.available_hours_per_day,
       j.timezone,
       j.technical_resources,
       j.created_at,
       j.updated_at
FROM recommendations r
JOIN job_positions j ON j.id = r.job_position_id
WHERE r.batch_id = $1
  AND j.deleted_at IS NULL
ORDER BY r.score DESC NULLS LAST, j.created_at DESC, r.id ASC
LIMIT $2 OFFSET $3;

-- name: ListEmployeeRecommendationsForJobPosition :many
SELECT r.id, r.batch_id, r.score,
       e.id AS employee_id,
       e.user_id,
       e.position,
       e.role,
       e.years_of_experience,
       e.certifications,
       e.portfolio_url,
       e.created_at,
       e.updated_at
FROM recommendations r
JOIN employees e ON e.id = r.employee_id
JOIN job_positions j ON j.id = r.job_position_id
WHERE r.batch_id = $1
  AND j.deleted_at IS NULL
ORDER BY r.score DESC NULLS LAST, e.updated_at DESC, r.id ASC
LIMIT $2 OFFSET $3;
