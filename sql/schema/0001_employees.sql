-- +goose Up
CREATE TABLE employees (
    id SERIAL PRIMARY KEY,
    position TEXT NOT NULL,
    role TEXT NOT NULL,
    years_of_experience TEXT NOT NULL,
    certifications TEXT[],
    portfolio_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT employee_years_of_experience_check
      CHECK (years_of_experience IN ('less_1y', '1y', '2_to_5y', '5_to_10y', 'more_10y'))
);

CREATE TABLE employee_profile_tech (
  id SERIAL PRIMARY KEY,
  employee_id INTEGER NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
  os TEXT,
  paid_software TEXT[] DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

  CONSTRAINT unique_profile_tech_by_employee UNIQUE(employee_id)
);

CREATE TABLE employee_internet_connections (
  id SERIAL PRIMARY KEY,
  employee_id INTEGER NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
  type TEXT NOT NULL,
  speed TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

  CONSTRAINT employee_internet_connections_type_check
      CHECK (type IN ('fiber', 'wifi', 'coaxial', 'adsl', 'mobile')),
  CONSTRAINT employee_internet_connections_speed_check
      CHECK (speed IN ('less_10mb', '20mb', '30mb', '40mb', 'more_50mb'))
);

CREATE TABLE employee_location (
  id SERIAL PRIMARY KEY,
  employee_id INTEGER NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
  timezone TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

  CONSTRAINT unique_employee_location UNIQUE(employee_id)
);

CREATE TABLE employee_profile_availability (
    id SERIAL PRIMARY KEY,
    employee_id INTEGER NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    available_hours_per_day SMALLINT NOT NULL CHECK (available_hours_per_day between 0 and 8),
    compatible_projects SMALLINT,
    incompatible_projects SMALLINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT unique_employee_availability UNIQUE(employee_id),
    CONSTRAINT employee_profile_availability_hours_check
        CHECK (available_hours_per_day IS NULL OR available_hours_per_day BETWEEN 0 AND 24)
);

CREATE TABLE employee_education (
    id SERIAL PRIMARY KEY,
    employee_id INTEGER NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    education_type TEXT NOT NULL,
    title TEXT NOT NULL,
    status TEXT NOT NULL ,
    certification TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT employee_education_type_check
        CHECK (education_type IN ('university', 'postgraduate', 'high-school-orientation', 'tertiary')),

    CONSTRAINT employee_education_status_check
        CHECK (status IN ('in-progress', 'completed'))
);

CREATE TABLE employee_files (
    id SERIAL PRIMARY KEY,
    employee_id INTEGER NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    bucket TEXT NOT NULL,
    object_key TEXT NOT NULL,
    original_filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    checksum_sha256 TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    uploaded_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT employee_files_type_check
        CHECK (type IN ('certification')),
    CONSTRAINT employee_files_status_check
        CHECK (status IN ('pending', 'uploaded', 'failed', 'deleted')),
    CONSTRAINT employee_files_size_bytes_check
        CHECK (size_bytes >= 0),
    CONSTRAINT employee_files_bucket_object_key_key UNIQUE (bucket, object_key)
);

CREATE INDEX employee_education_employee_id_idx
    ON employee_education(employee_id);

CREATE INDEX employee_internet_connections_employee_profile_tech_id_idx
    ON employee_internet_connections(employee_id);

CREATE INDEX employee_education_type_idx
    ON employee_education(education_type);

CREATE INDEX employee_files_employee_id_idx
    ON employee_files(employee_id);

CREATE INDEX employee_files_type_idx
    ON employee_files(type);

CREATE INDEX employee_files_status_idx
    ON employee_files(status);

-- +goose Down
DROP TABLE IF EXISTS employee_files;
DROP TABLE IF EXISTS employee_education;
DROP TABLE IF EXISTS employee_profile_availability;
DROP TABLE IF EXISTS employee_internet_connections;
DROP TABLE IF EXISTS employee_profile_tech;
DROP TABLE IF EXISTS employee_location;
DROP TABLE IF EXISTS employees;
