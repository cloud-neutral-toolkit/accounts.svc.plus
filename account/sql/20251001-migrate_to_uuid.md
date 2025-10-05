# 历史 UUID 迁移指引（已归档）

> **说明**：项目已经统一改为直接使用 `schema.sql` 初始化数据库，不再提供单独的 UUID 迁移脚本。本指南仅保留作为旧环境排查的参考，若你是全新部署，可直接执行 `schema.sql` 并忽略本文件。

1. **确认扩展**  
   登录 PostgreSQL，执行 `\dx` 检查是否已经启用了 `uuid-ossp` 或 `pgcrypto`。若没有，可执行：

   ```sql
   CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
   CREATE EXTENSION IF NOT EXISTS "pgcrypto";
   ```

   推荐使用 `pgcrypto` 的 `gen_random_uuid()`；若历史库已启用 `uuid-ossp`，脚本中的 `uuid_generate_v4()` 亦可直接使用。

2. **备份数据库**  
   任何迁移前都应留存快照：

   ```bash
   pg_dump -U <user> -d <dbname> > backup_before_uuid.sql
   ```

3. **执行迁移脚本（仅旧项目）**  
   旧版项目曾通过 `migrate_to_uuid.sql` 完成以下步骤：
   - 新增 `uuid` 主键列并填充现有数据；
   - 添加 `user_uuid` 外键字段，与旧的自增 `id` 并行；
   - 删除旧的 `id`、`user_id` 列并将 `uuid` 设为主键。

   如仍需在遗留环境执行，可根据上述逻辑自行编写脚本，或从历史提交中找回旧版 SQL。

4. **验证结果**  
   迁移完成后进入数据库（`psql -U <user> -d <dbname>`）并检查：

   ```sql
   \d users
   \d identities
   \d sessions
   ```

   预期结构示例：

   - `users(uuid PRIMARY KEY, username, password, email, created_at …)`
   - `identities(uuid PRIMARY KEY, user_uuid REFERENCES users(uuid), provider, external_id …)`
   - `sessions(uuid PRIMARY KEY, user_uuid REFERENCES users(uuid), token, expires_at …)`

   亦可通过信息架构核对外键：

   ```sql
   SELECT constraint_name, table_name, column_name, foreign_table_name
   FROM information_schema.key_column_usage
   WHERE table_schema = 'public';
   ```

5. **回滚策略**  
   若迁移出现问题，可使用备份恢复：

   ```bash
   psql -U <user> -d <dbname> < backup_before_uuid.sql
   ```

> **小结**：新部署只需运行 `schema.sql`。旧环境若要沿用 UUID 主键改造，可参考上述思路自行调整脚本，并务必在操作前备份。
