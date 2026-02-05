# 数据库访问与系统升级指南

本文档介绍如何通过 `stunnel` 安全访问数据库,执行 Agent 持久化迁移,以及验证系统状态。

## 1. 数据库访问 (DB Access via stunnel)

为了安全地从本地或开发环境访问生产数据库,我们使用 `stunnel` 隧道。

### 配置说明
- **配置文件**: `deploy/stunnel-account-db-client.conf`
- **本地监听**: `127.0.0.1:15432`
- **上游连接**: `postgresql.svc.plus:443`

### 启动方式
您可以使用 Makefile 中的快捷命令:

```bash
# 启动 stunnel 隧道
make stunnel-start
```

或手动启动:
```bash
stunnel deploy/stunnel-account-db-client.conf
```

### 连接验证
启动后,可以使用 `psql` 连接:
```bash
psql "postgres://postgres:${POSTGRES_PASSWORD}@127.0.0.1:15432/account?sslmode=disable"
```

---

## 2. 升级与迁移 (Upgrade & Migration)

### Agent 持久化迁移 (2026-02-05)
本次升级新增了 `agents` 表,用于存储各节点的运行状态。

**执行迁移**:
通过隧道连接后,运行以下脚本:
```bash
psql "postgres://postgres:${POSTGRES_PASSWORD}@127.0.0.1:15432/account?sslmode=disable" -f sql/20260205_agents_table.sql
```

**验证迁移**:
确认表和索引已存在:
```bash
psql "postgres://postgres:${POSTGRES_PASSWORD}@127.0.0.1:15432/account?sslmode=disable" -c "\dt agents"
```

---

## 3. 系统验证 (Verification)

### Agent 注册验证
部署新代码后,观察日志确认 Agent 正确自报 ID 并注册:
- 查找关键词: `agent status updated`
- 检查 `agentID` 字段是否为 `hk-xhttp.svc.plus` 等具体 ID 而非 `*`。

### 数据库持久化验证
查询 `agents` 表确认数据已填充:
```sql
SELECT id, healthy, last_heartbeat, clients_count, sync_revision FROM agents;
```

### 自动清理验证 (Stale Cleanup)
系统每 5 分钟执行一次清理,自动删除 10 分钟未更新心跳的 Agent。
- 观察日志关键词: `cleaned up stale agents`

---

## 4. 常见问题调试 (Debugging)

### 401 Unauthorized (`invalid_agent_token`)
- **检查**: 确认 Agent 端的 `apiToken` 与 Cloud Run 的环境变量 `INTERNAL_SERVICE_TOKEN` 完全一致。
- **配置路径**: Agent 节点的 `/etc/agent/account-agent.yaml`。

### 500 Internal Server Error (`/api/agent/nodes`)
- **检查**: 访问 `/api/agent/nodes` 时若报错,请检查 `accounts.svc.plus` 的环境变量。
- **修复**: 确保 `INTERNAL_SERVICE_TOKEN` 已正确设置。
