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

-- El worker necesita distinguir un batch inexistente de uno ya terminal: el primero
-- significa que el sujeto o el batch desaparecieron, y el segundo que otro procesamiento
-- ya lo cerró. Ambos se reconocen sin trabajo, pero el diagnóstico es distinto.
-- name: GetRecommendationBatch :one
SELECT id, subject_type, employee_id, job_position_id, status, created_at, updated_at
FROM recommendation_batches
WHERE id = $1;

-- Reclamo condicional del trabajo. A diferencia de TransitionRecommendationBatch, que es
-- incondicional y le alcanza al productor porque solo cierra un batch que acaba de abrir,
-- este no toca un batch terminal: un redelivery de un mensaje cuyo batch ya está
-- 'completed' lo devolvería a 'processing' y el reemplazo siguiente destruiría el conjunto
-- vigente que ese batch dejó.
--
-- El predicado incluye 'processing' y no solo 'pending' a propósito. Excluirlo haría el
-- reclamo estrictamente exclusivo, pero convertiría cualquier caída del worker a mitad de
-- un batch en un sujeto bloqueado para siempre por el índice único parcial: el redelivery
-- no podría reclamarlo. Que dos entregas se solapen es inocuo, porque completar un batch es
-- un reemplazo atómico completo y el último en commitear gana.
-- name: ClaimRecommendationBatch :one
UPDATE recommendation_batches
SET status = 'processing',
    updated_at = now()
WHERE id = $1
  AND status IN ('pending', 'processing')
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

-- Universo de candidatos. Todas estas consultas alimentan al worker: traducen las filas a
-- la entrada normalizada del contrato de scoring. Los puestos eliminados lógicamente quedan
-- fuera de todas, porque un puesto eliminado nunca se recomienda ni recibe candidatos.

-- Los LEFT JOIN son deliberados: el perfil del empleado se completa en cinco pasos, y un
-- paso sin completar debe producir NULL y no la desaparición de la fila. La ausencia de un
-- dato y su valor cero son estados distintos que el contrato de scoring distingue.
--
-- highest_education se resuelve en Go y no acá: el orden de los niveles ya es parte del
-- contrato de scoring, y una segunda copia en un CASE se desincronizaría.
-- name: GetEmployeeScoringProfile :one
SELECT e.id AS employee_id,
       e.years_of_experience,
       a.available_hours_per_day,
       l.timezone,
       t.paid_software,
       (t.employee_id IS NOT NULL)::boolean AS has_tech_profile,
       ARRAY(
           SELECT ed.education_type
           FROM employee_education ed
           WHERE ed.employee_id = e.id
       )::text[] AS education_types
FROM employees e
LEFT JOIN employee_profile_availability a ON a.employee_id = e.id
LEFT JOIN employee_location l ON l.employee_id = e.id
LEFT JOIN employee_profile_tech t ON t.employee_id = e.id
WHERE e.id = $1;

-- name: ListEmployeeScoringProfiles :many
SELECT e.id AS employee_id,
       e.years_of_experience,
       a.available_hours_per_day,
       l.timezone,
       t.paid_software,
       (t.employee_id IS NOT NULL)::boolean AS has_tech_profile,
       ARRAY(
           SELECT ed.education_type
           FROM employee_education ed
           WHERE ed.employee_id = e.id
       )::text[] AS education_types
FROM employees e
LEFT JOIN employee_profile_availability a ON a.employee_id = e.id
LEFT JOIN employee_location l ON l.employee_id = e.id
LEFT JOIN employee_profile_tech t ON t.employee_id = e.id
ORDER BY e.id;

-- name: GetActiveJobPositionRequirements :one
SELECT id AS job_position_id,
       required_experience,
       required_education_level,
       available_hours_per_day,
       timezone,
       technical_resources
FROM job_positions
WHERE id = $1
  AND deleted_at IS NULL;

-- name: ListActiveJobPositionRequirements :many
SELECT id AS job_position_id,
       required_experience,
       required_education_level,
       available_hours_per_day,
       timezone,
       technical_resources
FROM job_positions
WHERE deleted_at IS NULL
ORDER BY id;

-- El worker distingue un empleado inexistente de uno sin datos de perfil: el primero cierra
-- el batch sin recomendaciones, el segundo es un candidato válido.
-- name: EmployeeExists :one
SELECT EXISTS (SELECT 1 FROM employees WHERE id = $1);
