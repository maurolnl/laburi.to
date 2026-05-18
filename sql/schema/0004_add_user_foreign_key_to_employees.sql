-- +goose Up
ALTER TABLE employees
ADD COLUMN user_id INTEGER NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE employees
DROP COLUMN user_id;
