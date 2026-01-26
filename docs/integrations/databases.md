# 数据库对接

## PostgreSQL

- 主业务存储使用 PostgreSQL（见 `internal/store/postgres.go`）
- Schema 位于 `sql/schema.sql`
- 迁移工具：`migratectl`

## 同步策略

仓库提供两类同步方式（见 `sql/readme.md`）：

1) pgsync（单向异步）
- 适合单主写入 + 异步同步
- 不需要超级用户权限

2) pglogical（双主最终一致）
- 适合多区域双写
- 需要安装 pglogical 扩展

## 常用命令

```bash
# 初始化 schema
go run ./cmd/migratectl/main.go migrate --dsn "$DB_URL"

# 校验 schema
go run ./cmd/migratectl/main.go verify --dsn "$DB_URL" --schema sql/schema.sql
```
