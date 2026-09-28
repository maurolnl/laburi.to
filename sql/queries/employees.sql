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
  certification_name,
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
  $10,
  NOW(),
  NOW(),
  NOW()
);

-- Los certificados que una actualización conserva por document_id tienen que ser del mismo
-- empleado y estar cargados. Devuelve los que cumplen; el llamador compara contra lo pedido.
-- name: ListUploadedEmployeeFileIDs :many
SELECT id
FROM employee_files
WHERE employee_id = sqlc.arg(employee_id)
  AND id = ANY(sqlc.arg(ids)::int[])
  AND status = 'uploaded';

-- Da de baja los certificados asociados que la actualización no conserva y devuelve su
-- ubicación para borrarlos del almacenamiento después del commit. Los certificados sin
-- asociar no se tocan: solo se modifican si un ítem los referencia.
-- name: MarkUnkeptCertificationFilesDeleted :many
UPDATE employee_files
SET status = 'deleted',
    updated_at = NOW()
WHERE employee_id = sqlc.arg(employee_id)
  AND status = 'uploaded'
  AND certification_name IS NOT NULL
  AND NOT (id = ANY(sqlc.arg(kept_ids)::int[]))
RETURNING bucket, object_key;

-- Libera el nombre de los certificados conservados antes de reasignarlo, para que renombrar o
-- intercambiar certificaciones no choque de forma transitoria con el índice único.
-- name: ClearEmployeeFileCertificationNames :exec
UPDATE employee_files
SET certification_name = NULL,
    updated_at = NOW()
WHERE employee_id = sqlc.arg(employee_id)
  AND id = ANY(sqlc.arg(ids)::int[]);

-- name: SetEmployeeFileCertificationName :exec
UPDATE employee_files
SET certification_name = sqlc.arg(certification_name),
    updated_at = NOW()
WHERE employee_id = sqlc.arg(employee_id)
  AND id = sqlc.arg(id);

-- name: GetEmployee :one
SELECT
    employees.id,
    employees.position,
    employees.role,
    employees.years_of_experience,
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
    COALESCE((SELECT jsonb_agg(jsonb_build_object('name', c.name, 'document_id', f.id) ORDER BY c.ordinal) FROM unnest(employees.certifications) WITH ORDINALITY AS c(name, ordinal) LEFT JOIN employee_files f ON f.employee_id = employees.id AND f.status = 'uploaded' AND f.certification_name = c.name), '[]'::jsonb)::text AS certification_items,
    COALESCE((SELECT jsonb_agg(jsonb_build_object('id', id, 'title', original_filename) ORDER BY id) FROM employee_files WHERE employee_id = employees.id AND status = 'uploaded' AND certification_name IS NULL), '[]'::jsonb)::text AS files
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

-- name: IsEmployeeProfileComplete :one
-- Un perfil está completo cuando existen sus cinco pasos: el registro base, la locación, los
-- recursos técnicos, la disponibilidad y al menos un título de educación.
--
-- La regla es que la fila exista, no que tenga datos. El paso de recursos técnicos admite `os`
-- y `paid_software` vacíos por validación, así que exigir contenido dejaría a esos perfiles
-- fuera para siempre. Es la misma lectura que hace has_tech_profile en la resolución de
-- candidatos.
--
-- Un empleado inexistente devuelve false y no error: el llamador pregunta si corresponde
-- disparar, no si el empleado existe.
SELECT (
    EXISTS (SELECT 1 FROM employees e WHERE e.id = $1)
    AND EXISTS (SELECT 1 FROM employee_location l WHERE l.employee_id = $1)
    AND EXISTS (SELECT 1 FROM employee_profile_tech t WHERE t.employee_id = $1)
    AND EXISTS (SELECT 1 FROM employee_profile_availability a WHERE a.employee_id = $1)
    AND EXISTS (SELECT 1 FROM employee_education ed WHERE ed.employee_id = $1)
)::boolean AS complete;

-- Perfil completo direccionado por el identificador de empleado, para el borde que lo expone a
-- su dueño y a un empleador con recomendación vigente. Es una consulta aparte de GetEmployee y
-- no una variante de su WHERE porque no devuelve lo mismo: acá ningún archivo viaja con su
-- ubicación.
--
-- certification_items devuelve cada certificación en el orden declarado con el identificador
-- de su certificado activo, nulo si no tiene. files lista solo los certificados activos sin
-- certificación asociada (cargados antes de LAB-40). Ambos omiten lo que no está cargado, para
-- que coincidan exactamente con lo que las rutas de entrega aceptan servir. education reemplaza el object_key crudo por el identificador con el que se pide la
-- entrega del documento, nulo cuando el título no tiene ninguno.
-- name: GetEmployeeProfileByID :one
SELECT
    employees.id,
    employees.position,
    employees.role,
    employees.years_of_experience,
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
    COALESCE((SELECT jsonb_agg(jsonb_build_object('type', type, 'speed', speed) ORDER BY id) FROM employee_internet_connections WHERE employee_id = employees.id), '[]'::jsonb)::text AS internet_connections,
    COALESCE((SELECT jsonb_agg(jsonb_build_object('education_type', education_type, 'title', title, 'status', status, 'certification_document_id', CASE WHEN certification IS NOT NULL AND btrim(certification) <> '' THEN id END) ORDER BY id) FROM employee_education WHERE employee_id = employees.id), '[]'::jsonb)::text AS education,
    COALESCE((SELECT jsonb_agg(jsonb_build_object('name', c.name, 'document_id', f.id) ORDER BY c.ordinal) FROM unnest(employees.certifications) WITH ORDINALITY AS c(name, ordinal) LEFT JOIN employee_files f ON f.employee_id = employees.id AND f.status = 'uploaded' AND f.certification_name = c.name), '[]'::jsonb)::text AS certification_items,
    COALESCE((SELECT jsonb_agg(jsonb_build_object('id', id, 'title', original_filename) ORDER BY id) FROM employee_files WHERE employee_id = employees.id AND status = 'uploaded' AND certification_name IS NULL), '[]'::jsonb)::text AS files
FROM employees
JOIN users ON employees.user_id = users.id
LEFT JOIN employee_location ON employee_location.employee_id = employees.id
LEFT JOIN employee_profile_tech ON employee_profile_tech.employee_id = employees.id
LEFT JOIN employee_profile_availability ON employee_profile_availability.employee_id = employees.id
WHERE employees.id = $1;

-- Lectura de un certificado por el par empleado/archivo. El filtro por empleado va acá y no en
-- Go a propósito: un archivo ajeno no devuelve fila, así que es indistinguible de uno
-- inexistente desde la base y el borde no puede equivocarse al distinguirlos.
--
-- El filtro por status descarta el certificado cuyo contenido nunca terminó de subirse: no hay
-- nada que entregar, y el perfil tampoco lo lista.
-- name: GetEmployeeFileForEmployee :one
SELECT bucket, object_key, original_filename, content_type
FROM employee_files
WHERE id = $1
  AND employee_id = $2
  AND status = 'uploaded';

-- Equivalente para el documento de un título de educación, que no es una fila de
-- employee_files sino un object_key guardado en la propia fila de educación. El título sin
-- documento no devuelve fila: a efectos de entrega es lo mismo que un identificador
-- inexistente.
-- name: GetEmployeeEducationDocumentForEmployee :one
SELECT certification, title
FROM employee_education
WHERE id = $1
  AND employee_id = $2
  AND certification IS NOT NULL
  AND btrim(certification) <> '';
