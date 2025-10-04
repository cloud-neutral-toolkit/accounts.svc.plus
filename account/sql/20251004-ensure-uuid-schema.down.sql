-- Drop trigger before removing supporting function
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_trigger
        WHERE tgname = 'trg_users_set_updated_at'
          AND tgrelid = 'public.users'::regclass
    ) THEN
        DROP TRIGGER trg_users_set_updated_at ON public.users;
    END IF;
END
$$;

DROP FUNCTION IF EXISTS public.set_updated_at();

-- Drop indexes introduced by the migration
DROP INDEX IF EXISTS public.idx_sessions_user_uuid;
DROP INDEX IF EXISTS public.idx_identities_provider;
DROP INDEX IF EXISTS public.idx_identities_user_uuid;

-- Drop foreign keys
ALTER TABLE public.sessions
    DROP CONSTRAINT IF EXISTS sessions_user_fk;

ALTER TABLE public.identities
    DROP CONSTRAINT IF EXISTS identities_user_fk;

-- Drop unique constraints
ALTER TABLE public.identities
    DROP CONSTRAINT IF EXISTS identities_provider_external_id_uk;

ALTER TABLE public.users
    DROP CONSTRAINT IF EXISTS users_username_uk;

ALTER TABLE public.users
    DROP CONSTRAINT IF EXISTS users_email_uk;

-- Remove generated column but retain supporting timestamps
ALTER TABLE public.users
    DROP COLUMN IF EXISTS email_verified;


ALTER TABLE public.users
    ALTER COLUMN updated_at DROP DEFAULT;


-- Restore uuid columns to neutral defaults
ALTER TABLE public.sessions
    ALTER COLUMN uuid DROP DEFAULT,
    ALTER COLUMN uuid DROP NOT NULL;

ALTER TABLE public.identities
    ALTER COLUMN uuid DROP DEFAULT,
    ALTER COLUMN uuid DROP NOT NULL,
    ALTER COLUMN user_uuid DROP NOT NULL;

ALTER TABLE public.users
    ALTER COLUMN uuid DROP DEFAULT,
    ALTER COLUMN uuid DROP NOT NULL;

ALTER TABLE public.sessions
    ALTER COLUMN user_uuid DROP NOT NULL;

-- Drop primary keys added by the migration
ALTER TABLE public.sessions
    DROP CONSTRAINT IF EXISTS sessions_pkey;

ALTER TABLE public.identities
    DROP CONSTRAINT IF EXISTS identities_pkey;

ALTER TABLE public.users
    DROP CONSTRAINT IF EXISTS users_pkey;
