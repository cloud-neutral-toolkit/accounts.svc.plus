-- =========================================
-- 20251008.migrate.generated-columns.sql
-- Migration: Convert generated columns to normal + trigger
-- Safe to re-run (幂等)
-- =========================================

BEGIN;

-- 1️⃣ 兼容 pglogical：确保当前用户可访问 schema
DO $$
BEGIN
    BEGIN
        GRANT USAGE ON SCHEMA pglogical TO CURRENT_USER;
        RAISE NOTICE 'Granted USAGE on schema pglogical to current user';
    EXCEPTION WHEN OTHERS THEN
        RAISE NOTICE 'Skipping GRANT for schema pglogical (possibly already granted or insufficient privilege)';
    END;
END;
$$;

-- 2️⃣ 删除不兼容的 generated column
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'users'
          AND column_name = 'email_verified'
          AND is_generated = 'ALWAYS'
    ) THEN
        RAISE NOTICE 'Detected generated column: users.email_verified — dropping it for pglogical compatibility';
        ALTER TABLE public.users DROP COLUMN email_verified;
    ELSE
        RAISE NOTICE 'No generated column detected: users.email_verified is safe';
    END IF;
END;
$$;

-- 3️⃣ 确保存在普通列 email_verified
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'users'
          AND column_name = 'email_verified'
    ) THEN
        RAISE NOTICE 'Adding normal column: users.email_verified (BOOLEAN DEFAULT false)';
        ALTER TABLE public.users ADD COLUMN email_verified BOOLEAN DEFAULT false NOT NULL;
    ELSE
        RAISE NOTICE 'Column users.email_verified already exists, skipping ADD COLUMN';
    END IF;
END;
$$;

-- 4️⃣ 定义幂等触发器函数
DROP FUNCTION IF EXISTS public.maintain_email_verified() CASCADE;

CREATE FUNCTION public.maintain_email_verified() RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.email_verified := (NEW.email_verified_at IS NOT NULL);
    RETURN NEW;
END;
$$;

-- 5️⃣ 触发器同步 email_verified 与 email_verified_at
DROP TRIGGER IF EXISTS trg_users_maintain_email_verified ON public.users;

CREATE TRIGGER trg_users_maintain_email_verified
BEFORE INSERT OR UPDATE ON public.users
FOR EACH ROW EXECUTE FUNCTION public.maintain_email_verified();

-- 6️⃣ 修正历史数据
UPDATE public.users
SET email_verified = (email_verified_at IS NOT NULL)
WHERE email_verified IS DISTINCT FROM (email_verified_at IS NOT NULL);

COMMIT;

-- =========================================
-- End of migration
-- =========================================

