-- +migrate Up
-- Сведения сторон для договора и отметка о том, что документ сформирован.
-- Объект аренды и условия уже хранятся в listings, здесь только данные людей.
ALTER TABLE listings
    ADD COLUMN IF NOT EXISTS tenant_full_name       VARCHAR(128),
    ADD COLUMN IF NOT EXISTS tenant_phone           VARCHAR(32),
    ADD COLUMN IF NOT EXISTS landlord_full_name     VARCHAR(128),
    ADD COLUMN IF NOT EXISTS landlord_phone         VARCHAR(32),
    ADD COLUMN IF NOT EXISTS contract_document_token VARCHAR(255);

COMMENT ON COLUMN listings.tenant_full_name IS 'ФИО нанимателя для договора';
COMMENT ON COLUMN listings.tenant_phone IS 'Телефон нанимателя для договора';
COMMENT ON COLUMN listings.landlord_full_name IS 'ФИО наймодателя для договора';
COMMENT ON COLUMN listings.landlord_phone IS 'Телефон наймодателя для договора';
COMMENT ON COLUMN listings.contract_document_token IS 'Токен загруженного файла договора в MAX';
