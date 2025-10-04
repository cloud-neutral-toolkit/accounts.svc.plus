CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- USERS ------------------------------------------------------------------
ALTER TABLE public.users
    ADD COLUMN IF NOT EXISTS uuid uuid;

UPDATE public.users
SET uuid = gen_random_uuid()
WHERE uuid IS NULL;

ALTER TABLE public.users
    ALTER COLUMN uuid TYPE uuid USING uuid::uuid;

ALTER TABLE public.users
    ALTER COLUMN uuid SET DEFAULT gen_random_uuid();

ALTER TABLE public.users
    ALTER COLUMN uuid SET NOT NULL;

-- Ensure uuid columns are of the UUID type
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'users'
          AND column_name = 'uuid'
          AND udt_name <> 'uuid'
    ) THEN
        ALTER TABLE public.users
            ALTER COLUMN uuid TYPE uuid USING uuid::uuid;
    END IF;
END
$$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'identities'
          AND column_name = 'uuid'
          AND udt_name <> 'uuid'
    ) THEN
        ALTER TABLE public.identities
            ALTER COLUMN uuid TYPE uuid USING uuid::uuid;
    END IF;
END
$$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'identities'
          AND column_name = 'user_uuid'
          AND udt_name <> 'uuid'
    ) THEN
        ALTER TABLE public.identities
            ALTER COLUMN user_uuid TYPE uuid USING user_uuid::uuid;
    END IF;
END
$$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'sessions'
          AND column_name = 'uuid'
          AND udt_name <> 'uuid'
    ) THEN
        ALTER TABLE public.sessions
            ALTER COLUMN uuid TYPE uuid USING uuid::uuid;
    END IF;
END
$$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'sessions'
          AND column_name = 'user_uuid'
          AND udt_name <> 'uuid'
    ) THEN
        ALTER TABLE public.sessions
            ALTER COLUMN user_uuid TYPE uuid USING user_uuid::uuid;
    END IF;
END
$$;

-- Fill missing UUIDs before enforcing constraints
UPDATE public.users SET uuid = gen_random_uuid() WHERE uuid IS NULL;
UPDATE public.identities SET uuid = gen_random_uuid() WHERE uuid IS NULL;
UPDATE public.sessions SET uuid = gen_random_uuid() WHERE uuid IS NULL;

-- Ensure NOT NULL on uuid columns
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'users'
          AND column_name = 'uuid'
          AND is_nullable = 'YES'
    ) THEN
        ALTER TABLE public.users
            ALTER COLUMN uuid SET NOT NULL;
    END IF;
END
$$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'identities'
          AND column_name = 'uuid'
          AND is_nullable = 'YES'
    ) THEN
        ALTER TABLE public.identities
            ALTER COLUMN uuid SET NOT NULL;
    END IF;
END
$$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'sessions'
          AND column_name = 'uuid'
          AND is_nullable = 'YES'
    ) THEN
        ALTER TABLE public.sessions
            ALTER COLUMN uuid SET NOT NULL;
    END IF;
END
$$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'identities'
          AND column_name = 'user_uuid'
          AND is_nullable = 'YES'
    ) THEN
        ALTER TABLE public.identities
            ALTER COLUMN user_uuid SET NOT NULL;
    END IF;
END
$$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'sessions'
          AND column_name = 'user_uuid'
          AND is_nullable = 'YES'
    ) THEN
        ALTER TABLE public.sessions
            ALTER COLUMN user_uuid SET NOT NULL;
    END IF;
END
$$;

-- Ensure defaults for uuid columns
DO $$
DECLARE
    current_default text;
    target_attnum int;
BEGIN
    SELECT attnum INTO target_attnum
    FROM pg_attribute
    WHERE attrelid = 'public.users'::regclass
      AND attname = 'uuid'
      AND NOT attisdropped;

    IF target_attnum IS NOT NULL THEN
        SELECT pg_get_expr(adbin, adrelid)
        INTO current_default
        FROM pg_attrdef
        WHERE adrelid = 'public.users'::regclass
          AND adnum = target_attnum;

        IF current_default IS DISTINCT FROM 'gen_random_uuid()' THEN
            ALTER TABLE public.users
                ALTER COLUMN uuid SET DEFAULT gen_random_uuid();
        END IF;
    END IF;
END
$$;

DO $$
DECLARE
    current_default text;
    target_attnum int;
BEGIN
    SELECT attnum INTO target_attnum
    FROM pg_attribute
    WHERE attrelid = 'public.identities'::regclass
      AND attname = 'uuid'
      AND NOT attisdropped;

    IF target_attnum IS NOT NULL THEN
        SELECT pg_get_expr(adbin, adrelid)
        INTO current_default
        FROM pg_attrdef
        WHERE adrelid = 'public.identities'::regclass
          AND adnum = target_attnum;

        IF current_default IS DISTINCT FROM 'gen_random_uuid()' THEN
            ALTER TABLE public.identities
                ALTER COLUMN uuid SET DEFAULT gen_random_uuid();
        END IF;
    END IF;
END
$$;

DO $$
DECLARE
    current_default text;
    target_attnum int;
BEGIN
    SELECT attnum INTO target_attnum
    FROM pg_attribute
    WHERE attrelid = 'public.sessions'::regclass
      AND attname = 'uuid'
      AND NOT attisdropped;

    IF target_attnum IS NOT NULL THEN
        SELECT pg_get_expr(adbin, adrelid)
        INTO current_default
        FROM pg_attrdef
        WHERE adrelid = 'public.sessions'::regclass
          AND adnum = target_attnum;

        IF current_default IS DISTINCT FROM 'gen_random_uuid()' THEN
            ALTER TABLE public.sessions
                ALTER COLUMN uuid SET DEFAULT gen_random_uuid();
        END IF;
    END IF;
END
$$;


ALTER TABLE public.users
    ADD COLUMN IF NOT EXISTS email_verified_at timestamptz;

ALTER TABLE public.users
    ADD COLUMN IF NOT EXISTS updated_at timestamptz;

UPDATE public.users

SET updated_at = COALESCE(updated_at, now());

DO $$
DECLARE
    target_attnum int;
    current_default text;
BEGIN
    SELECT attnum INTO target_attnum
    FROM pg_attribute
    WHERE attrelid = 'public.users'::regclass
      AND attname = 'updated_at'
      AND NOT attisdropped;

    IF target_attnum IS NOT NULL THEN
        SELECT pg_get_expr(adbin, adrelid)
        INTO current_default
        FROM pg_attrdef
        WHERE adrelid = 'public.users'::regclass
          AND adnum = target_attnum;

        IF current_default IS DISTINCT FROM 'now()' THEN
            ALTER TABLE public.users
                ALTER COLUMN updated_at SET DEFAULT now();
        END IF;
    END IF;
END
$$;

DO $$
DECLARE
    att_generated char(1);
BEGIN
    SELECT a.attgenerated
    INTO att_generated
    FROM pg_attribute a
    WHERE a.attrelid = 'public.users'::regclass
      AND a.attname = 'email_verified'
      AND NOT a.attisdropped;

    IF att_generated IS NULL THEN
        EXECUTE 'ALTER TABLE public.users ADD COLUMN email_verified boolean GENERATED ALWAYS AS (email_verified_at IS NOT NULL) STORED';
    ELSIF att_generated <> 's' THEN
        EXECUTE 'ALTER TABLE public.users DROP COLUMN email_verified';
        EXECUTE 'ALTER TABLE public.users ADD COLUMN email_verified boolean GENERATED ALWAYS AS (email_verified_at IS NOT NULL) STORED';
    END IF;
END
$$;

CREATE OR REPLACE FUNCTION public.set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;


DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_trigger
        WHERE tgname = 'trg_users_set_updated_at'
          AND tgrelid = 'public.users'::regclass
          AND NOT tgisinternal
    ) THEN
        CREATE TRIGGER trg_users_set_updated_at
        BEFORE UPDATE ON public.users
        FOR EACH ROW
        EXECUTE FUNCTION public.set_updated_at();
    END IF;
END
$$;

DO $$
DECLARE
    pk_name text;
BEGIN
    SELECT conname INTO pk_name
    FROM pg_constraint
    WHERE conrelid = 'public.users'::regclass
      AND contype = 'p'
    LIMIT 1;

    IF pk_name IS NULL THEN
        ALTER TABLE public.users
            ADD CONSTRAINT users_pkey PRIMARY KEY (uuid);
    ELSIF pk_name <> 'users_pkey' THEN
        EXECUTE format('ALTER TABLE public.users RENAME CONSTRAINT %I TO users_pkey', pk_name);
    END IF;
END
$$;


DO $$
DECLARE
    constraint_name text;
    email_att smallint;
BEGIN
    SELECT attnum INTO email_att
    FROM pg_attribute
    WHERE attrelid = 'public.users'::regclass
      AND attname = 'email'
      AND NOT attisdropped;

    IF email_att IS NOT NULL THEN
        SELECT conname INTO constraint_name
        FROM pg_constraint
        WHERE conrelid = 'public.users'::regclass
          AND contype = 'u'
          AND conkey = ARRAY[email_att]
        LIMIT 1;

        IF constraint_name IS NULL THEN
            ALTER TABLE public.users
                ADD CONSTRAINT users_email_uk UNIQUE (email);
        ELSIF constraint_name <> 'users_email_uk' THEN
            EXECUTE format('ALTER TABLE public.users RENAME CONSTRAINT %I TO users_email_uk', constraint_name);
        END IF;
    END IF;
END
$$;

DO $$
DECLARE
    constraint_name text;
    username_att smallint;
BEGIN
    SELECT attnum INTO username_att
    FROM pg_attribute
    WHERE attrelid = 'public.users'::regclass
      AND attname = 'username'
      AND NOT attisdropped;

    IF username_att IS NOT NULL THEN
        SELECT conname INTO constraint_name
        FROM pg_constraint
        WHERE conrelid = 'public.users'::regclass
          AND contype = 'u'
          AND conkey = ARRAY[username_att]
        LIMIT 1;

        IF constraint_name IS NULL THEN
            ALTER TABLE public.users
                ADD CONSTRAINT users_username_uk UNIQUE (username);
        ELSIF constraint_name <> 'users_username_uk' THEN
            EXECUTE format('ALTER TABLE public.users RENAME CONSTRAINT %I TO users_username_uk', constraint_name);
        END IF;
    END IF;
END
$$;


-- IDENTITIES --------------------------------------------------------------
ALTER TABLE public.identities
    ADD COLUMN IF NOT EXISTS uuid uuid;

UPDATE public.identities
SET uuid = gen_random_uuid()
WHERE uuid IS NULL;

ALTER TABLE public.identities
    ALTER COLUMN uuid TYPE uuid USING uuid::uuid;

ALTER TABLE public.identities
    ALTER COLUMN uuid SET DEFAULT gen_random_uuid();

ALTER TABLE public.identities
    ALTER COLUMN uuid SET NOT NULL;

ALTER TABLE public.identities
    ADD COLUMN IF NOT EXISTS user_uuid uuid;

ALTER TABLE public.identities
    ALTER COLUMN user_uuid TYPE uuid USING user_uuid::uuid;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'identities'
          AND column_name = 'user_id'
    ) THEN
        UPDATE public.identities i
        SET user_uuid = u.uuid
        FROM public.users u
        WHERE i.user_uuid IS NULL
          AND u.id = i.user_id;
    END IF;
END
$$;

ALTER TABLE public.identities
    ALTER COLUMN user_uuid SET NOT NULL;

DO $$
DECLARE
    pk_name text;
BEGIN
    SELECT conname INTO pk_name
    FROM pg_constraint
    WHERE conrelid = 'public.identities'::regclass
      AND contype = 'p'
    LIMIT 1;

    IF pk_name IS NULL THEN
        ALTER TABLE public.identities
            ADD CONSTRAINT identities_pkey PRIMARY KEY (uuid);
    ELSIF pk_name <> 'identities_pkey' THEN
        EXECUTE format('ALTER TABLE public.identities RENAME CONSTRAINT %I TO identities_pkey', pk_name);
    END IF;
END
$$;


DO $$
DECLARE
    constraint_name text;
    provider_att smallint;
    external_att smallint;
BEGIN
    SELECT attnum INTO provider_att
    FROM pg_attribute
    WHERE attrelid = 'public.identities'::regclass
      AND attname = 'provider'
      AND NOT attisdropped;

    SELECT attnum INTO external_att
    FROM pg_attribute
    WHERE attrelid = 'public.identities'::regclass
      AND attname = 'external_id'
      AND NOT attisdropped;

    IF provider_att IS NOT NULL AND external_att IS NOT NULL THEN
        SELECT conname INTO constraint_name
        FROM pg_constraint
        WHERE conrelid = 'public.identities'::regclass
          AND contype = 'u'
          AND conkey = ARRAY[provider_att, external_att]
        LIMIT 1;

        IF constraint_name IS NULL THEN
            ALTER TABLE public.identities
                ADD CONSTRAINT identities_provider_external_id_uk UNIQUE (provider, external_id);
        ELSIF constraint_name <> 'identities_provider_external_id_uk' THEN
            EXECUTE format('ALTER TABLE public.identities RENAME CONSTRAINT %I TO identities_provider_external_id_uk', constraint_name);
        END IF;
    END IF;
END
$$;


DO $$
DECLARE
    fk_name text;
BEGIN
    SELECT conname INTO fk_name
    FROM pg_constraint
    WHERE conrelid = 'public.identities'::regclass
      AND contype = 'f'
      AND confrelid = 'public.users'::regclass
    LIMIT 1;

    IF fk_name IS NULL THEN
        ALTER TABLE public.identities
            ADD CONSTRAINT identities_user_fk FOREIGN KEY (user_uuid)
                REFERENCES public.users(uuid) ON DELETE CASCADE;
    ELSIF fk_name <> 'identities_user_fk' THEN
        EXECUTE format('ALTER TABLE public.identities RENAME CONSTRAINT %I TO identities_user_fk', fk_name);
    END IF;
END
$$;

DO $$
DECLARE
    idx_name text;
    att smallint;
BEGIN
    SELECT attnum INTO att

    FROM pg_attribute
    WHERE attrelid = 'public.identities'::regclass
      AND attname = 'user_uuid'
      AND NOT attisdropped;


    IF att IS NOT NULL THEN

        SELECT cls.relname INTO idx_name
        FROM pg_index idx
        JOIN pg_class cls ON cls.oid = idx.indexrelid
        WHERE idx.indrelid = 'public.identities'::regclass
          AND idx.indisunique = FALSE
          AND idx.indkey = ARRAY[att]::int2vector
        LIMIT 1;

        IF idx_name IS NULL THEN
            CREATE INDEX IF NOT EXISTS idx_identities_user_uuid ON public.identities (user_uuid);
        ELSIF idx_name <> 'idx_identities_user_uuid' THEN
            EXECUTE format('ALTER INDEX %I RENAME TO idx_identities_user_uuid', idx_name);
        END IF;
    END IF;
END
$$;

DO $$
DECLARE
    idx_name text;
    att smallint;
BEGIN
    SELECT attnum INTO att

    FROM pg_attribute
    WHERE attrelid = 'public.identities'::regclass
      AND attname = 'provider'
      AND NOT attisdropped;


    IF att IS NOT NULL THEN
        SELECT cls.relname INTO idx_name
        FROM pg_index idx
        JOIN pg_class cls ON cls.oid = idx.indexrelid
        WHERE idx.indrelid = 'public.identities'::regclass
          AND idx.indisunique = FALSE
          AND idx.indkey = ARRAY[att]::int2vector

        LIMIT 1;

        IF idx_name IS NULL THEN
            CREATE INDEX IF NOT EXISTS idx_identities_provider ON public.identities (provider);
        ELSIF idx_name <> 'idx_identities_provider' THEN
            EXECUTE format('ALTER INDEX %I RENAME TO idx_identities_provider', idx_name);
        END IF;
    END IF;
END
$$;

-- SESSIONS ----------------------------------------------------------------
ALTER TABLE public.sessions
    ADD COLUMN IF NOT EXISTS uuid uuid;

UPDATE public.sessions
SET uuid = gen_random_uuid()
WHERE uuid IS NULL;

ALTER TABLE public.sessions
    ALTER COLUMN uuid TYPE uuid USING uuid::uuid;

ALTER TABLE public.sessions
    ALTER COLUMN uuid SET DEFAULT gen_random_uuid();

ALTER TABLE public.sessions
    ALTER COLUMN uuid SET NOT NULL;

ALTER TABLE public.sessions
    ADD COLUMN IF NOT EXISTS user_uuid uuid;

ALTER TABLE public.sessions
    ALTER COLUMN user_uuid TYPE uuid USING user_uuid::uuid;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'sessions'
          AND column_name = 'user_id'
    ) THEN
        UPDATE public.sessions s
        SET user_uuid = u.uuid
        FROM public.users u
        WHERE s.user_uuid IS NULL
          AND u.id = s.user_id;
    END IF;
END
$$;

ALTER TABLE public.sessions
    ALTER COLUMN user_uuid SET NOT NULL;

DO $$
DECLARE
    pk_name text;
BEGIN
    SELECT conname INTO pk_name
    FROM pg_constraint
    WHERE conrelid = 'public.sessions'::regclass
      AND contype = 'p'
    LIMIT 1;

    IF pk_name IS NULL THEN
        ALTER TABLE public.sessions
            ADD CONSTRAINT sessions_pkey PRIMARY KEY (uuid);
    ELSIF pk_name <> 'sessions_pkey' THEN
        EXECUTE format('ALTER TABLE public.sessions RENAME CONSTRAINT %I TO sessions_pkey', pk_name);
    END IF;
END
$$;

DO $$
DECLARE
    fk_name text;
BEGIN
    SELECT conname INTO fk_name
    FROM pg_constraint
    WHERE conrelid = 'public.sessions'::regclass
      AND contype = 'f'
      AND confrelid = 'public.users'::regclass
    LIMIT 1;

    IF fk_name IS NULL THEN
        ALTER TABLE public.sessions
            ADD CONSTRAINT sessions_user_fk FOREIGN KEY (user_uuid)
                REFERENCES public.users(uuid) ON DELETE CASCADE;
    ELSIF fk_name <> 'sessions_user_fk' THEN
        EXECUTE format('ALTER TABLE public.sessions RENAME CONSTRAINT %I TO sessions_user_fk', fk_name);
    END IF;
END
$$;

DO $$
DECLARE
    idx_name text;
    att smallint;
BEGIN
    SELECT attnum INTO att

    FROM pg_attribute
    WHERE attrelid = 'public.sessions'::regclass
      AND attname = 'user_uuid'
      AND NOT attisdropped;


    IF att IS NOT NULL THEN

        SELECT cls.relname INTO idx_name
        FROM pg_index idx
        JOIN pg_class cls ON cls.oid = idx.indexrelid
        WHERE idx.indrelid = 'public.sessions'::regclass
          AND idx.indisunique = FALSE
          AND idx.indkey = ARRAY[att]::int2vector

        LIMIT 1;

        IF idx_name IS NULL THEN
            CREATE INDEX IF NOT EXISTS idx_sessions_user_uuid ON public.sessions (user_uuid);
        ELSIF idx_name <> 'idx_sessions_user_uuid' THEN
            EXECUTE format('ALTER INDEX %I RENAME TO idx_sessions_user_uuid', idx_name);
        END IF;
    END IF;
END
$$;
