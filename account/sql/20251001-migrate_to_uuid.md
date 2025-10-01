# 前置准备

1. 确认扩展
登录 PostgreSQL，执行： \dx 看看是否已经有 uuid-ossp 或 pgcrypto。
如果没有，就执行： 

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
或者：
CREATE EXTENSION IF NOT EXISTS "pgcrypto";


⚠️ 推荐用 pgcrypto（函数是 gen_random_uuid()），更现代；
如果你已经用了 uuid-ossp，脚本里的 uuid_generate_v4() 就能直接用。

2. 备份数据库

这是最关键的保险：

pg_dump -U <user> -d <dbname> > backup_before_uuid.sql

3. 执行迁移

假设你的 schema 在 schema.sql 文件里，迁移脚本在 migrate_to_uuid.sql 文件里。

执行： psql -U <user> -d <dbname> -f migrate_to_uuid.sql


这个脚本会做三步：

- 新增 uuid 列，填充数据。
- 新建 user_uuid 外键字段，保持和旧 id 外键并存。
- 删除旧的 id、user_id，把 uuid 设为主键。

4. 验证结果

迁移后，进入数据库： psql -U <user> -d <dbname>

查看表结构：

\d users
\d identities
\d sessions


应该看到：

users(uuid PRIMARY KEY, username, password, email, created_at …)
identities(uuid PRIMARY KEY, user_uuid REFERENCES users(uuid), provider, external_id …)
sessions(uuid PRIMARY KEY, user_uuid REFERENCES users(uuid), token, expires_at …)

再验证外键是否正确：

SELECT constraint_name, table_name, column_name, foreign_table_name
FROM information_schema.key_column_usage
WHERE table_schema = 'public';

回滚策略

如果迁移出错，可以用之前的备份恢复：

psql -U <user> -d <dbname> < backup_before_uuid.sql

总结：

先启用扩展
再运行迁移脚本
验证新表结构和外键
保留备份，随时可回滚
