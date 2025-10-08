sudo -u postgres psql -d account -c "GRANT USAGE ON SCHEMA pglogical TO PUBLIC;"

-- 登录 postgres
sudo -u postgres psql -d account

-- 授权 shenlan 对 public schema 全权限
ALTER SCHEMA public OWNER TO shenlan;
GRANT ALL ON SCHEMA public TO shenlan;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO shenlan;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO shenlan;
GRANT ALL PRIVILEGES ON ALL FUNCTIONS IN SCHEMA public TO shenlan;

-- 授权 pglogical schema 使用权限（防止混用）
GRANT USAGE ON SCHEMA pglogical TO shenlan;



\q

