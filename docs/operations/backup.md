# 备份与恢复

## 逻辑备份（YAML）

```bash
migratectl export --dsn "$DB_URL" --output account-export.yaml
migratectl import --dsn "$DB_URL" --file account-export.yaml
```

## 数据库层备份

建议使用 PostgreSQL 原生命令：

```bash
pg_dump "$DB_URL" > account.dump
pg_restore -d "$DB_URL" account.dump
```
