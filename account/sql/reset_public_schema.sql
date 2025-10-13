-- Ensure the public schema exists without requiring superuser privileges.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_namespace WHERE nspname = 'public'
    ) THEN
        EXECUTE format('CREATE SCHEMA public AUTHORIZATION %I;', :'db_user');
    END IF;
END
$$;

-- Grant privileges when we own the schema, otherwise skip with a notice.
DO $$
DECLARE
    owner name;
BEGIN
    SELECT pg_catalog.pg_get_userbyid(nspowner)
      INTO owner
      FROM pg_namespace
     WHERE nspname = 'public';

    IF owner = current_user THEN
        EXECUTE format('GRANT ALL ON SCHEMA public TO %I;', :'db_user');
        EXECUTE 'GRANT ALL ON SCHEMA public TO public;';
    ELSE
        RAISE NOTICE 'Skipping GRANT on schema public because current user % is not owner (%).',
            current_user,
            owner;
    END IF;
END
$$;
