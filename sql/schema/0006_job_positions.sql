-- +goose Up
CREATE TABLE job_positions (
    id SERIAL PRIMARY KEY,
    employer_id INTEGER NOT NULL REFERENCES employers(id) ON DELETE CASCADE,
    position TEXT NOT NULL,
    role TEXT NOT NULL,
    required_experience TEXT NOT NULL,
    required_education_level TEXT NOT NULL,
    available_hours_per_day SMALLINT NOT NULL,
    timezone TEXT NOT NULL,
    technical_resources TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT job_positions_required_experience_check
        CHECK (required_experience IN ('less_1y', '1y', '2_to_5y', '5_to_10y', 'more_10y')),
    CONSTRAINT job_positions_required_education_level_check
        CHECK (required_education_level IN ('university', 'postgraduate', 'high-school-orientation', 'tertiary')),
    CONSTRAINT job_positions_available_hours_per_day_check
        CHECK (available_hours_per_day BETWEEN 1 AND 8)
);

CREATE INDEX job_positions_employer_id_idx
    ON job_positions(employer_id)
    WHERE deleted_at IS NULL;

CREATE INDEX job_positions_required_experience_idx
    ON job_positions(required_experience)
    WHERE deleted_at IS NULL;

CREATE INDEX job_positions_required_education_level_idx
    ON job_positions(required_education_level)
    WHERE deleted_at IS NULL;

CREATE INDEX job_positions_timezone_idx
    ON job_positions(timezone)
    WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS job_positions_timezone_idx;
DROP INDEX IF EXISTS job_positions_required_education_level_idx;
DROP INDEX IF EXISTS job_positions_required_experience_idx;
DROP INDEX IF EXISTS job_positions_employer_id_idx;

DROP TABLE IF EXISTS job_positions;
