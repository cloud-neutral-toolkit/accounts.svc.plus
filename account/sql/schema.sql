-- =========================================
-- schema_base.sql
-- Shared schema for pglogical bidirectional sync
-- PostgreSQL 16 + gen_random_uuid()
-- =========================================

DROP SCHEMA IF EXISTS public CASCADE;
CREATE SCHEMA public AUTHORIZATION CURRENT_USER;

-- =========================================
-- Extensions
-- =========================================
CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;
CREATE EXTENSION IF NOT EXISTS pglogical WITH SCHEMA pglogical;

-- =========================================
-- Functions
-- =========================================
CREATE OR REPLACE FUNCTION public.set_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION public.maintain_email_verified() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  NEW.email_verified := (NEW.email_verified_at IS NOT NULL);
  RETURN NEW;
END;
$$;

-- =========================================
-- Tables
-- =========================================

CREATE TABLE public.users (
  uuid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  username TEXT NOT NULL,
  password TEXT NOT NULL,
  email TEXT,
  role TEXT NOT NULL DEFAULT 'user',
  level INTEGER NOT NULL DEFAULT 20,
  groups JSONB NOT NULL DEFAULT '[]'::jsonb,
  permissions JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  mfa_totp_secret TEXT,
  mfa_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  mfa_secret_issued_at TIMESTAMPTZ,
  mfa_confirmed_at TIMESTAMPTZ,
  email_verified_at TIMESTAMPTZ,
  email_verified BOOLEAN GENERATED ALWAYS AS ((email_verified_at IS NOT NULL)) STORED
);

CREATE TABLE public.identities (
  uuid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  provider TEXT NOT NULL,
  external_id TEXT NOT NULL,
  user_uuid UUID NOT NULL REFERENCES public.users(uuid) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT identities_provider_external_id_uk UNIQUE (provider, external_id)
);

CREATE TABLE public.sessions (
  uuid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  token TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  user_uuid UUID NOT NULL REFERENCES public.users(uuid) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE public.admin_settings (
  uuid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  module_key TEXT NOT NULL,
  role TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT FALSE,
  version BIGINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT admin_settings_module_role_uk UNIQUE (module_key, role)
);

-- =========================================
-- Indexes
-- =========================================
CREATE UNIQUE INDEX users_username_lower_uk ON public.users (lower(username));
CREATE UNIQUE INDEX users_email_lower_uk ON public.users (lower(email)) WHERE email IS NOT NULL;
CREATE INDEX idx_identities_user_uuid ON public.identities (user_uuid);
CREATE INDEX idx_sessions_user_uuid ON public.sessions (user_uuid);
CREATE INDEX idx_admin_settings_version ON public.admin_settings (version);

-- =========================================
-- Triggers
-- =========================================
CREATE TRIGGER trg_users_set_updated_at
  BEFORE UPDATE ON public.users
  FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

CREATE TRIGGER trg_users_maintain_email_verified
  BEFORE INSERT OR UPDATE ON public.users
  FOR EACH ROW EXECUTE FUNCTION public.maintain_email_verified();

CREATE TRIGGER trg_identities_set_updated_at
  BEFORE UPDATE ON public.identities
  FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

CREATE TRIGGER trg_sessions_set_updated_at
  BEFORE UPDATE ON public.sessions
  FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

CREATE TRIGGER trg_admin_settings_set_updated_at
  BEFORE UPDATE ON public.admin_settings
  FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

