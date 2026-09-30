-- +migrate Down
ALTER TABLE listings
    DROP COLUMN IF EXISTS contract_document_token,
    DROP COLUMN IF EXISTS landlord_phone,
    DROP COLUMN IF EXISTS landlord_full_name,
    DROP COLUMN IF EXISTS tenant_phone,
    DROP COLUMN IF EXISTS tenant_full_name;
