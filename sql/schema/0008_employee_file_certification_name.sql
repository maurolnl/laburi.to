-- +goose Up
-- Cada certificado queda asociado al nombre de su certificación en employees.certifications.
-- Los certificados cargados antes de este cambio conservan NULL: son certificados sin asociar,
-- se siguen listando y entregando, y nadie infiere a qué certificación pertenecen.
--
-- Todas las sentencias son idempotentes: la migración puede reaplicarse sin error ni cambios.
ALTER TABLE employee_files
    ADD COLUMN IF NOT EXISTS certification_name TEXT;

-- A lo sumo un certificado activo por certificación. Los dados de baja quedan fuera del índice,
-- así que el historial de reemplazos no choca con el certificado vigente.
CREATE UNIQUE INDEX IF NOT EXISTS employee_files_active_certification_key
    ON employee_files(employee_id, certification_name)
    WHERE status = 'uploaded' AND certification_name IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS employee_files_active_certification_key;

ALTER TABLE employee_files
    DROP COLUMN IF EXISTS certification_name;
