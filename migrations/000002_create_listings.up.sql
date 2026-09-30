-- +migrate Up
-- Заявка арендодателя: объект и условия аренды до подключения арендатора.
CREATE TABLE IF NOT EXISTS listings (
    id          BIGSERIAL PRIMARY KEY,
    -- Код выдаётся при публикации, поэтому у черновика он пуст.
    -- Несколько NULL не нарушают UNIQUE.
    code        VARCHAR(6) UNIQUE,
    landlord_id BIGINT      NOT NULL REFERENCES users (id),
    tenant_id   BIGINT REFERENCES users (id),
    status      VARCHAR(16) NOT NULL DEFAULT 'draft',
    step        VARCHAR(32) NOT NULL DEFAULT 'address',

    address     TEXT,
    price       NUMERIC(12, 2),
    deposit     NUMERIC(12, 2),
    term        VARCHAR(16),
    utilities   VARCHAR(16),
    description TEXT,

    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_listings_status CHECK (status IN ('draft', 'published', 'paired', 'cancelled')),
    CONSTRAINT chk_listings_step CHECK (step IN ('address', 'price', 'deposit', 'term', 'utilities', 'description', 'done')),
    CONSTRAINT chk_listings_term CHECK (term IS NULL OR term IN ('short', 'long')),
    CONSTRAINT chk_listings_utilities CHECK (utilities IS NULL OR utilities IN ('tenant', 'landlord', 'shared')),
    -- Код обязателен, когда заявку уже можно найти по нему.
    -- У черновика и у отменённого черновика кода нет — иначе /cancel
    -- на незаполненной анкете падал бы с нарушением ограничения.
    CONSTRAINT chk_listings_published_has_code
        CHECK (status NOT IN ('published', 'paired') OR code IS NOT NULL)
);

COMMENT ON TABLE listings IS 'Заявки арендодателя на сдачу жилья';
COMMENT ON COLUMN listings.code IS 'Шестизначный код для поиска заявки арендатором, NULL у черновика';
COMMENT ON COLUMN listings.landlord_id IS 'Арендодатель, создавший заявку';
COMMENT ON COLUMN listings.tenant_id IS 'Арендатор, подключившийся к заявке';
COMMENT ON COLUMN listings.step IS 'Текущий вопрос анкеты';

-- Один активный черновик или опубликованная заявка на арендодателя.
CREATE UNIQUE INDEX IF NOT EXISTS idx_listings_one_active_per_landlord
    ON listings (landlord_id)
    WHERE status IN ('draft', 'published');

CREATE INDEX IF NOT EXISTS idx_listings_tenant_id ON listings (tenant_id);
CREATE INDEX IF NOT EXISTS idx_listings_status ON listings (status);
