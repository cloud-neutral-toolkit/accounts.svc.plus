
------------------------------------------------
-- STEP 2: 切换到 UUID-only
------------------------------------------------

-- 删除旧外键（如果还在的话）
ALTER TABLE identities DROP CONSTRAINT IF EXISTS identities_user_id_fkey;
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_user_id_fkey;

-- 删除旧主键
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_pkey;
ALTER TABLE identities DROP CONSTRAINT IF EXISTS identities_pkey;
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_pkey;

-- 设置 uuid 为主键
ALTER TABLE users ADD CONSTRAINT users_pkey PRIMARY KEY (uuid);
ALTER TABLE identities ADD CONSTRAINT identities_pkey PRIMARY KEY (uuid);
ALTER TABLE sessions ADD CONSTRAINT sessions_pkey PRIMARY KEY (uuid);

-- 删除旧 id 字段（如果还存在）
ALTER TABLE users DROP COLUMN IF EXISTS id;
ALTER TABLE identities DROP COLUMN IF EXISTS id;
ALTER TABLE sessions DROP COLUMN IF EXISTS id;

-- 删除旧的 user_id 外键字段（如果还存在）
ALTER TABLE identities DROP COLUMN IF EXISTS user_id;
ALTER TABLE sessions DROP COLUMN IF EXISTS user_id;

