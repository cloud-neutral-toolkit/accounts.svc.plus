-- =========================================
-- schema_pglogical_init.sql
-- Dedicated pglogical schema initialization separated from business schema.
-- Execute schema_pglogical_patch.sql afterwards to align origin defaults.

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_namespace WHERE nspname = 'pglogical'
  ) THEN
    EXECUTE format('CREATE SCHEMA pglogical AUTHORIZATION %I', current_user);
  END IF;
END;
$$;

-- Install pglogical extension into the dedicated schema.
CREATE EXTENSION IF NOT EXISTS pglogical WITH SCHEMA pglogical;
