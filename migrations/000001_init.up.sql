-- +migrate Up
CREATE TABLE IF NOT EXISTS users (
    id         UUID PRIMARY KEY,
    username   VARCHAR(32) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users (username);
