-- name: CreateEmployer :one
INSERT INTO employers (
    user_id,
    name,
    industry,
    location,
    hiring_modalities
)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, user_id, name, industry, location, hiring_modalities, created_at, updated_at;

-- name: GetEmployerByUserID :one
SELECT id, user_id, name, industry, location, hiring_modalities, created_at, updated_at
FROM employers
WHERE user_id = $1;
