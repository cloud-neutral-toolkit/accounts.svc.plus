使用新的 `migratectl` CLI 可以在不同环境下快速执行迁移、校验和重置操作：

```bash
go run ./cmd/migratectl/main.go migrate --dsn "$DB_URL"
go run ./cmd/migratectl/main.go check --cn "$CN_DSN" --global "$GLOBAL_DSN"
```

以下命令展示了如何授予 pglogical schema 访问权限：

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


执行顺序建议
步骤	节点	脚本	说明
1️⃣	Global	schema_base.sql	创建业务结构
2️⃣	CN	schema_base.sql	创建相同业务结构
3️⃣	Global	schema_pglogical_region_global.sql	定义 Global provider + 订阅 CN
4️⃣	CN	schema_pglogical_region_cn.sql	定义 CN provider + 订阅 Global
🧩 验证同步状态
SELECT * FROM pglogical.show_subscription_status();

如果输出中：

status = 'replicating'

即表示 双向复制同步正常。

🚀 双向同步特性汇总
特性	实现机制
双主写入	两端都是 Provider + Subscriber
唯一性保障	所有主键为 gen_random_uuid()，避免冲突
邮箱唯一	lower(email) 唯一索引
异步复制	WAL 级逻辑同步，自动断点续传
结构一致性	schema_base.sql 保证完全相同
幂等可重建	全部 IF NOT EXISTS，可重复执行
可扩展性	可新增字段或表，通过 replication_set_add_table() 同步
