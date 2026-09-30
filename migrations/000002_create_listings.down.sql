-- +migrate Down
DROP INDEX IF EXISTS idx_listings_status;
DROP INDEX IF EXISTS idx_listings_tenant_id;
DROP INDEX IF EXISTS idx_listings_one_active_per_landlord;
DROP TABLE IF EXISTS listings;
