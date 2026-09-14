-- +goose Up
ALTER TABLE users
ADD COLUMN role TEXT;

UPDATE users
SET role = 'employer';

ALTER TABLE users
ADD CONSTRAINT users_role_check
CHECK (role IN ('employee', 'employer'));

ALTER TABLE users
ALTER COLUMN role SET NOT NULL;

-- +goose StatementBegin
CREATE FUNCTION prevent_user_role_change()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.role IS DISTINCT FROM OLD.role THEN
        RAISE EXCEPTION 'user role cannot be changed'
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER prevent_user_role_change
BEFORE UPDATE OF role ON users
FOR EACH ROW
EXECUTE FUNCTION prevent_user_role_change();

CREATE TABLE employers (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    industry TEXT NOT NULL,
    location TEXT NOT NULL,
    hiring_modalities TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT unique_employer_by_user UNIQUE (user_id)
);

-- +goose StatementBegin
CREATE FUNCTION enforce_exclusive_user_profile()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(1818, NEW.user_id);

    IF TG_TABLE_NAME = 'employees' THEN
        IF EXISTS (
            SELECT 1
            FROM employers
            WHERE user_id = NEW.user_id
        ) THEN
            RAISE EXCEPTION 'user already has an employer profile'
                USING ERRCODE = '23514';
        END IF;
    ELSIF TG_TABLE_NAME = 'employers' THEN
        IF EXISTS (
            SELECT 1
            FROM employees
            WHERE user_id = NEW.user_id
        ) THEN
            RAISE EXCEPTION 'user already has an employee profile'
                USING ERRCODE = '23514';
        END IF;
    END IF;

    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER enforce_exclusive_employee_profile
BEFORE INSERT OR UPDATE OF user_id ON employees
FOR EACH ROW
EXECUTE FUNCTION enforce_exclusive_user_profile();

CREATE TRIGGER enforce_exclusive_employer_profile
BEFORE INSERT OR UPDATE OF user_id ON employers
FOR EACH ROW
EXECUTE FUNCTION enforce_exclusive_user_profile();

-- +goose Down
DROP TRIGGER IF EXISTS enforce_exclusive_employee_profile ON employees;
DROP TRIGGER IF EXISTS enforce_exclusive_employer_profile ON employers;
DROP FUNCTION IF EXISTS enforce_exclusive_user_profile();

DROP TABLE IF EXISTS employers;

DROP TRIGGER IF EXISTS prevent_user_role_change ON users;
DROP FUNCTION IF EXISTS prevent_user_role_change();

ALTER TABLE users
DROP CONSTRAINT IF EXISTS users_role_check;

ALTER TABLE users
DROP COLUMN IF EXISTS role;
