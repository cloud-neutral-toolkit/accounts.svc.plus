-- 启用扩展（只需执行一次）
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
-- 或者：CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
    id SERIAL UNIQUE, -- 保留自增 id 作为内部用途
    uuid UUID PRIMARY KEY DEFAULT uuid_generate_v4(), -- 业务主键
    username TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,
    email TEXT,
    mfa_totp_secret TEXT,
    mfa_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    mfa_secret_issued_at TIMESTAMPTZ,
    mfa_confirmed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS identities (
    id SERIAL UNIQUE,
    uuid UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(uuid) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    external_id TEXT NOT NULL,
    UNIQUE(provider, external_id)
);

CREATE TABLE IF NOT EXISTS sessions (
    id SERIAL UNIQUE,
    uuid UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(uuid) ON DELETE CASCADE,
    token TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);
