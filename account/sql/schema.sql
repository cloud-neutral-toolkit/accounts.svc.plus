-- =========================================
-- PostgreSQL schema initialization script
-- Safe to re-run (幂等)
-- =========================================

SET statement_timeout = 0;
SET lock_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SET row_security = off;
SET search_path = public;

-- 必要扩展
CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;

-- =========================================
-- Function: set_updated_at()
-- =========================================
DROP FUNCTION IF EXISTS set_updated_at() CASCADE;

CREATE FUNCTION set_updated_at() RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

-- =========================================
-- Tables
-- =========================================

CREATE TABLE IF NOT EXISTS users (
    username TEXT NOT NULL,
    password TEXT NOT NULL,
    email TEXT,
    level INTEGER DEFAULT 20 NOT NULL,
    role TEXT DEFAULT 'user' NOT NULL,
    groups JSONB DEFAULT '[]'::jsonb NOT NULL,
    permissions JSONB DEFAULT '[]'::jsonb NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    uuid UUID DEFAULT gen_random_uuid() NOT NULL,
    mfa_totp_secret TEXT,
    mfa_enabled BOOLEAN DEFAULT false NOT NULL,
    mfa_secret_issued_at TIMESTAMPTZ,
    mfa_confirmed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ DEFAULT now(),
    email_verified_at TIMESTAMPTZ,
    email_verified BOOLEAN GENERATED ALWAYS AS ((email_verified_at IS NOT NULL)) STORED,
    CONSTRAINT users_pkey PRIMARY KEY (uuid),
    CONSTRAINT users_username_uk UNIQUE (username),
    CONSTRAINT users_email_uk UNIQUE (email)
);

CREATE UNIQUE INDEX IF NOT EXISTS users_username_lower_uk ON users (lower(username));
CREATE UNIQUE INDEX IF NOT EXISTS users_email_lower_uk ON users (lower(email)) WHERE email IS NOT NULL;

CREATE TABLE IF NOT EXISTS identities (
    provider TEXT NOT NULL,
    external_id TEXT NOT NULL,
    uuid UUID DEFAULT gen_random_uuid() NOT NULL,
    user_uuid UUID NOT NULL,
    CONSTRAINT identities_pkey PRIMARY KEY (uuid),
    CONSTRAINT identities_provider_external_id_uk UNIQUE (provider, external_id),
    CONSTRAINT identities_user_fk FOREIGN KEY (user_uuid)
        REFERENCES users(uuid) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS sessions (
    token TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    uuid UUID DEFAULT gen_random_uuid() NOT NULL,
    user_uuid UUID NOT NULL,
    CONSTRAINT sessions_pkey PRIMARY KEY (uuid),
    CONSTRAINT sessions_user_fk FOREIGN KEY (user_uuid)
        REFERENCES users(uuid) ON DELETE CASCADE
);

-- =========================================
-- Indexes
-- =========================================
CREATE INDEX IF NOT EXISTS idx_identities_provider ON identities (provider);
CREATE INDEX IF NOT EXISTS idx_identities_user_uuid ON identities (user_uuid);
CREATE INDEX IF NOT EXISTS idx_sessions_user_uuid ON sessions (user_uuid);

-- =========================================
-- Trigger
-- =========================================
DROP TRIGGER IF EXISTS trg_users_set_updated_at ON users;
CREATE TRIGGER trg_users_set_updated_at
BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- =========================================
-- End of schema.sql
-- =========================================
