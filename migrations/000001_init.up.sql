-- +migrate Up
CREATE TABLE IF NOT EXISTS users (
    id         BIGINT PRIMARY KEY,
    role       VARCHAR(16),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_users_role CHECK (role IS NULL OR role IN ('tenant', 'landlord'))
);

COMMENT ON TABLE users IS 'Пользователи бота MAXRent';
COMMENT ON COLUMN users.id IS 'Идентификатор пользователя в MAX (user_id)';
COMMENT ON COLUMN users.role IS 'Роль в сделке: tenant — арендатель, landlord — арендодатель, NULL — не выбрана';
