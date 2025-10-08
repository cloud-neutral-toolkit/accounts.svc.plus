BEGIN;
ALTER TABLE public.users
  ADD COLUMN IF NOT EXISTS level INTEGER DEFAULT 20 NOT NULL,
  ADD COLUMN IF NOT EXISTS role TEXT DEFAULT 'user' NOT NULL,
  ADD COLUMN IF NOT EXISTS groups JSONB DEFAULT '[]'::jsonb NOT NULL,
  ADD COLUMN IF NOT EXISTS permissions JSONB DEFAULT '[]'::jsonb NOT NULL,
  ADD COLUMN IF NOT EXISTS mfa_totp_secret TEXT,
  ADD COLUMN IF NOT EXISTS mfa_enabled BOOLEAN DEFAULT false NOT NULL,
  ADD COLUMN IF NOT EXISTS mfa_secret_issued_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS mfa_confirmed_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS email_verified BOOLEAN GENERATED ALWAYS AS ((email_verified_at IS NOT NULL)) STORED;

-- 重新创建最新的用户索引
CREATE UNIQUE INDEX IF NOT EXISTS users_username_lower_uk ON public.users (lower(username));
CREATE UNIQUE INDEX IF NOT EXISTS users_email_lower_uk ON public.users (lower(email)) WHERE email IS NOT NULL;

UPDATE public.users
SET role = CASE level
    WHEN 0 THEN 'admin'
    WHEN 10 THEN 'operator'
    ELSE 'user'
  END,
  groups = CASE level
    WHEN 0 THEN '["Admin"]'::jsonb
    WHEN 10 THEN '["Operator"]'::jsonb
    ELSE '["User"]'::jsonb
  END,
  permissions = CASE level
    WHEN 0 THEN '["session:read","session:write","user:manage"]'::jsonb
    WHEN 10 THEN '["session:read","session:write"]'::jsonb
    ELSE '["session:read"]'::jsonb
  END
WHERE role IS NULL OR role = '' OR groups = '[]'::jsonb;

COMMIT;
