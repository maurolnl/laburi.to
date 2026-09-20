-- +goose Up
CREATE TABLE recommendation_batches (
    id SERIAL PRIMARY KEY,
    subject_type TEXT NOT NULL,
    employee_id INTEGER REFERENCES employees(id) ON DELETE CASCADE,
    job_position_id INTEGER REFERENCES job_positions(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT recommendation_batches_subject_type_check
        CHECK (subject_type IN ('employee', 'job_position')),
    CONSTRAINT recommendation_batches_status_check
        CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    -- El sujeto es excluyente: exactamente una de las dos claves foráneas está
    -- presente y debe coincidir con subject_type. La redundancia es deliberada:
    -- subject_type hace legibles las queries y el check impide que las tres columnas
    -- se desincronicen.
    CONSTRAINT recommendation_batches_subject_check
        CHECK (
            (subject_type = 'employee'
                AND employee_id IS NOT NULL
                AND job_position_id IS NULL)
            OR
            (subject_type = 'job_position'
                AND job_position_id IS NOT NULL
                AND employee_id IS NULL)
        )
);

-- Un único batch en curso por sujeto: la base rechaza el trabajo paralelo, de modo que
-- un redelivery o un mensaje duplicado no crean dos ejecuciones para el mismo sujeto.
CREATE UNIQUE INDEX recommendation_batches_employee_in_flight_key
    ON recommendation_batches(employee_id)
    WHERE employee_id IS NOT NULL AND status IN ('pending', 'processing');

CREATE UNIQUE INDEX recommendation_batches_job_position_in_flight_key
    ON recommendation_batches(job_position_id)
    WHERE job_position_id IS NOT NULL AND status IN ('pending', 'processing');

-- Resuelven tanto el batch más reciente del sujeto como el último completado.
CREATE INDEX recommendation_batches_employee_recent_idx
    ON recommendation_batches(employee_id, created_at DESC)
    WHERE employee_id IS NOT NULL;

CREATE INDEX recommendation_batches_job_position_recent_idx
    ON recommendation_batches(job_position_id, created_at DESC)
    WHERE job_position_id IS NOT NULL;

CREATE TABLE recommendations (
    id SERIAL PRIMARY KEY,
    batch_id INTEGER NOT NULL REFERENCES recommendation_batches(id) ON DELETE CASCADE,
    employee_id INTEGER NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    job_position_id INTEGER NOT NULL REFERENCES job_positions(id) ON DELETE CASCADE,
    -- Nullable a propósito: LAB-30 todavía no define los indicadores y la épica prohíbe
    -- inventar un algoritmo temporal. Ausencia de puntaje no es puntaje cero.
    -- (7,4) y no (6,4): LAB-30 todavía no fija la escala del puntaje, y el techo de
    -- 99.9999 dejaría afuera el caso más probable, un score porcentual de 100.
    score NUMERIC(7,4),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT recommendations_unique_pair_per_batch
        UNIQUE (batch_id, employee_id, job_position_id)
);

-- NULLS LAST replica el orden de lectura: las recomendaciones sin puntaje van al final.
CREATE INDEX recommendations_batch_score_idx
    ON recommendations(batch_id, score DESC NULLS LAST);

CREATE INDEX recommendations_employee_id_idx
    ON recommendations(employee_id);

CREATE INDEX recommendations_job_position_id_idx
    ON recommendations(job_position_id);

-- +goose Down
DROP INDEX IF EXISTS recommendations_job_position_id_idx;
DROP INDEX IF EXISTS recommendations_employee_id_idx;
DROP INDEX IF EXISTS recommendations_batch_score_idx;

DROP TABLE IF EXISTS recommendations;

DROP INDEX IF EXISTS recommendation_batches_job_position_recent_idx;
DROP INDEX IF EXISTS recommendation_batches_employee_recent_idx;
DROP INDEX IF EXISTS recommendation_batches_job_position_in_flight_key;
DROP INDEX IF EXISTS recommendation_batches_employee_in_flight_key;

DROP TABLE IF EXISTS recommendation_batches;
