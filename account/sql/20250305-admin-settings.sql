-- Migration: create admin_settings table for permission matrix

BEGIN;

CREATE TABLE IF NOT EXISTS admin_settings (
    id BIGSERIAL PRIMARY KEY,
    module_key TEXT NOT NULL,
    role TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    version BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT admin_settings_module_role_uk UNIQUE (module_key, role)
);

CREATE INDEX IF NOT EXISTS idx_admin_settings_version ON admin_settings (version);

DROP TRIGGER IF EXISTS trg_admin_settings_set_updated_at ON admin_settings;
CREATE TRIGGER trg_admin_settings_set_updated_at
BEFORE UPDATE ON admin_settings
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;
