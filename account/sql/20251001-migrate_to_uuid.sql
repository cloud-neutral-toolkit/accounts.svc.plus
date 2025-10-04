------------------------------------------------
-- STEP 0: 启用 UUID 扩展
------------------------------------------------
-- 两种方式任选其一，推荐 pgcrypto（更现代）
-- 如果数据库还没装扩展，可以先运行 CREATE EXTENSION

-- 方式一：使用 uuid-ossp
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 方式二：使用 pgcrypto
-- CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- 注意：
-- uuid-ossp 用 uuid_generate_v4()
-- pgcrypto 用 gen_random_uuid()

------------------------------------------------
-- STEP 1: 平滑迁移（添加 UUID 字段并建立外键）
------------------------------------------------

-- ========== Users ==========
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS uuid UUID DEFAULT uuid_generate_v4();

UPDATE users SET uuid = uuid_generate_v4() WHERE uuid IS NULL;

ALTER TABLE users
    ALTER COLUMN uuid SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'users'::regclass
          AND conname = 'users_uuid_unique'
    ) THEN
        EXECUTE 'ALTER TABLE users ADD CONSTRAINT users_uuid_unique UNIQUE (uuid)';
    END IF;
END $$;

-- ========== Identities ==========
ALTER TABLE identities
    ADD COLUMN IF NOT EXISTS uuid UUID DEFAULT uuid_generate_v4();

UPDATE identities SET uuid = uuid_generate_v4() WHERE uuid IS NULL;

ALTER TABLE identities
    ALTER COLUMN uuid SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'identities'::regclass
          AND conname = 'identities_uuid_unique'
    ) THEN
        EXECUTE 'ALTER TABLE identities ADD CONSTRAINT identities_uuid_unique UNIQUE (uuid)';
    END IF;
END $$;

-- 新增 user_uuid 外键字段
ALTER TABLE identities
    ADD COLUMN IF NOT EXISTS user_uuid UUID;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'identities'
          AND column_name = 'user_id'
          AND table_schema = 'public'
    ) THEN
        EXECUTE 'UPDATE identities i SET user_uuid = u.uuid FROM users u WHERE i.user_id = u.id;';
    END IF;
END $$;

ALTER TABLE identities
    ALTER COLUMN user_uuid SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'identities'::regclass
          AND conname = 'identities_user_uuid_fk'
    ) THEN
        EXECUTE 'ALTER TABLE identities ADD CONSTRAINT identities_user_uuid_fk FOREIGN KEY (user_uuid) REFERENCES users(uuid) ON DELETE CASCADE';
    END IF;
END $$;

-- ========== Sessions ==========
ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS uuid UUID DEFAULT uuid_generate_v4();

UPDATE sessions SET uuid = uuid_generate_v4() WHERE uuid IS NULL;

ALTER TABLE sessions
    ALTER COLUMN uuid SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'sessions'::regclass
          AND conname = 'sessions_uuid_unique'
    ) THEN
        EXECUTE 'ALTER TABLE sessions ADD CONSTRAINT sessions_uuid_unique UNIQUE (uuid)';
    END IF;
END $$;

-- 新增 user_uuid 外键字段
ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS user_uuid UUID;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'sessions'
          AND column_name = 'user_id'
          AND table_schema = 'public'
    ) THEN
        EXECUTE 'UPDATE sessions s SET user_uuid = u.uuid FROM users u WHERE s.user_id = u.id;';
    END IF;
END $$;

ALTER TABLE sessions
    ALTER COLUMN user_uuid SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'sessions'::regclass
          AND conname = 'sessions_user_uuid_fk'
    ) THEN
        EXECUTE 'ALTER TABLE sessions ADD CONSTRAINT sessions_user_uuid_fk FOREIGN KEY (user_uuid) REFERENCES users(uuid) ON DELETE CASCADE';
    END IF;
END $$;


------------------------------------------------
-- STEP 2: 清理（彻底切换到 UUID 主键）
------------------------------------------------

-- 删除原有的主键约束
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_pkey;
ALTER TABLE identities DROP CONSTRAINT IF EXISTS identities_pkey;
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_pkey;

-- 删除旧的 id 外键
ALTER TABLE identities DROP CONSTRAINT IF EXISTS identities_user_id_fkey;
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_user_id_fkey;

-- 设置 uuid 为新的主键
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'users'::regclass
          AND conname = 'users_pkey'
    ) THEN
        EXECUTE 'ALTER TABLE users ADD CONSTRAINT users_pkey PRIMARY KEY (uuid)';
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'identities'::regclass
          AND conname = 'identities_pkey'
    ) THEN
        EXECUTE 'ALTER TABLE identities ADD CONSTRAINT identities_pkey PRIMARY KEY (uuid)';
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'sessions'::regclass
          AND conname = 'sessions_pkey'
    ) THEN
        EXECUTE 'ALTER TABLE sessions ADD CONSTRAINT sessions_pkey PRIMARY KEY (uuid)';
    END IF;
END $$;

-- 删除旧的 id 字段
ALTER TABLE users DROP COLUMN IF EXISTS id;
ALTER TABLE identities DROP COLUMN IF EXISTS id;
ALTER TABLE sessions DROP COLUMN IF EXISTS id;

-- 删除旧的 user_id 外键字段
ALTER TABLE identities DROP COLUMN IF EXISTS user_id;
ALTER TABLE sessions DROP COLUMN IF EXISTS user_id;

------------------------------------------------
-- Done: 所有表都只用 UUID 作为主键/外键
------------------------------------------------
