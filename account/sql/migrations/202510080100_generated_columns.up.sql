-- =========================================
-- 20251008-fix-generated-columns.sql
-- Migration: Remove generated columns incompatible with pglogical
-- Safe to re-run (幂等)
-- =========================================

BEGIN;

-- 尝试确保当前用户对 pglogical schema 有访问权限
DO $grant$
BEGIN
    BEGIN
        EXECUTE 'GRANT USAGE ON SCHEMA pglogical TO ' || current_user;
        EXECUTE 'GRANT ALL ON ALL TABLES IN SCHEMA pglogical TO ' || current_user;
        EXECUTE 'GRANT ALL ON ALL SEQUENCES IN SCHEMA pglogical TO ' || current_user;
        EXECUTE 'GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA pglogical TO ' || current_user;
        RAISE NOTICE 'Granted pglogical schema permissions to current_user: %', current_user;
    EXCEPTION WHEN others THEN
        RAISE NOTICE 'Skipping GRANT for schema pglogical (possibly already granted or insufficient privilege)';
    END;
END;
$grant$;

-- =========================================
-- Step 1. 检查并删除旧的 generated column
-- =========================================
DO $drop$
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
        EXECUTE 'ALTER TABLE public.users DROP COLUMN email_verified';
    ELSE
        RAISE NOTICE 'No generated column detected: users.email_verified is safe';
    END IF;
END;
$drop$;

-- =========================================
-- Step 2. 添加普通布尔列 (safe re-run)
-- =========================================
DO $add$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'users'
          AND column_name = 'email_verified'
    ) THEN
        RAISE NOTICE 'Adding normal column: users.email_verified (BOOLEAN DEFAULT false)';
        EXECUTE 'ALTER TABLE public.users ADD COLUMN email_verified BOOLEAN DEFAULT false NOT NULL';
    ELSE
        RAISE NOTICE 'Column users.email_verified already exists, skipping ADD COLUMN';
    END IF;
END;
$add$;

-- =========================================
-- Step 3. 创建维护触发器
-- =========================================
DROP FUNCTION IF EXISTS maintain_email_verified() CASCADE;

CREATE FUNCTION maintain_email_verified() RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.email_verified := (NEW.email_verified_at IS NOT NULL);
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_users_maintain_email_verified ON public.users;

CREATE TRIGGER trg_users_maintain_email_verified
BEFORE INSERT OR UPDATE ON public.users
FOR EACH ROW EXECUTE FUNCTION maintain_email_verified();

-- =========================================
-- Step 4. 修正现有数据
-- =========================================
UPDATE public.users
SET email_verified = (email_verified_at IS NOT NULL)
WHERE email_verified IS DISTINCT FROM (email_verified_at IS NOT NULL);

COMMIT;

-- =========================================
-- End of migration
-- =========================================

