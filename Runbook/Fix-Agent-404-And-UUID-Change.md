# 修复 Agent 404 错误和用户 UUID 变更

**日期**: 2026-02-05  
**负责人**: SRE Team  
**审核人**: DevOps Lead  
**最后更新**: 2026-02-05T15:15:00+08:00

## 问题描述

### 1. Agent 通信 404 错误
- **现象**: Agent 服务在向 `accounts-svc-plus` 报告状态时收到 404 错误
- **影响范围**: 所有 agent 节点无法正常上报心跳和配置同步
- **错误日志**:
  ```
  POST 404 https://accounts-svc-plus-266500572462.asia-northeast1.run.app/api/agent-server/v1/status
  GET 404 https://accounts-svc-plus-266500572462.asia-northeast1.run.app/api/agent-server/v1/users
  ```

### 2. 用户 UUID 变更需求
- **用户**: tester123@example.com
- **原 UUID**: `4b66928e-a81e-4981-bae0-289ddb92439c`
- **新 UUID**: `18d270a9-533d-4b13-b3f1-e7f55540a9b2`
- **原因**: 业务需求，需要将用户 ID 更改为指定值

### 3. Agent 节点数据显示问题
- **现象**: `/panel/agent` 页面显示 "Loading control center..."
- **影响**: 用户无法查看运行节点状态

## 诊断步骤

### 1. 检查 Agent 日志
```bash
# 查看 accounts-svc-plus 日志
gcloud run services logs read accounts-svc-plus \
  --project=xzerolab-480008 \
  --region=asia-northeast1 \
  --limit=20

# 发现 404 错误
# POST 404 /api/agent-server/v1/status
# GET 404 /api/agent-server/v1/users
```

### 2. 检查路由配置
```bash
# 查看 accounts.svc.plus/cmd/accountsvc/main.go
# 确认后端已注册 /api/agent-server/v1 路由

# 查看 console.svc.plus/src/app/api 目录
# 发现缺少 agent-server 代理路由
```

### 3. 检查数据库约束
```bash
# 连接到 PostgreSQL
ssh -i ~/.ssh/id_rsa root@postgresql.svc.plus

# 查看外键约束
docker exec postgresql-svc-plus psql -U postgres -d account -c "
  SELECT conname, conrelid::regclass 
  FROM pg_constraint 
  WHERE confrelid = 'public.users'::regclass;
"

# 结果显示:
# - identities_user_uuid_fkey
# - sessions_user_uuid_fkey
# - subscriptions_user_uuid_fkey
```

### 4. 检查用户表结构
```bash
docker exec postgresql-svc-plus psql -U postgres -d account -c "\d users"

# 确认表中没有 active 字段（已在代码中处理但未在生产数据库中）
```

## 修复方案

### 修复 1: 添加 Agent Server 代理路由

**文件**: `console.svc.plus/src/app/api/agent-server/[...segments]/route.ts`

```typescript
export const dynamic = 'force-dynamic'

import type { NextRequest } from 'next/server'

import { createUpstreamProxyHandler } from '@lib/apiProxy'
import { getAccountServiceBaseUrl } from '@server/serviceConfig'

const AGENT_SERVER_PREFIX = '/api/agent-server'

function createHandler() {
  const upstreamBaseUrl = getAccountServiceBaseUrl()
  return createUpstreamProxyHandler({
    upstreamBaseUrl,
    upstreamPathPrefix: AGENT_SERVER_PREFIX,
  })
}

const handler = createHandler()

export function GET(request: NextRequest) {
  return handler(request)
}

export function POST(request: NextRequest) {
  return handler(request)
}

export function PUT(request: NextRequest) {
  return handler(request)
}

export function PATCH(request: NextRequest) {
  return handler(request)
}

export function DELETE(request: NextRequest) {
  return handler(request)
}

export function HEAD(request: NextRequest) {
  return handler(request)
}

export function OPTIONS(request: NextRequest) {
  return handler(request)
}
```

**说明**: 
- 创建代理路由将前端的 `/api/agent-server/*` 请求转发到 `accounts-svc-plus`
- 解决 404 错误问题

### 修复 2: 增强 Registry 持久化和日志

**文件**: `accounts.svc.plus/internal/agentserver/registry.go`

**变更**:
1. 添加 `logger *slog.Logger` 字段到 `Registry` 结构体
2. 添加 `SetLogger()` 方法
3. 在 `RegisterAgent()` 和 `ReportStatus()` 中添加错误日志

**关键代码**:
```go
// 在 ReportStatus 中添加日志
if err := r.store.UpsertAgent(ctx, dbAgent); err != nil {
    r.logger.Error("failed to persist agent status heartbeat", "agent", a.ID, "err", err)
}

// 在 RegisterAgent 中添加日志
if err := r.store.UpsertAgent(ctx, dbAgent); err != nil {
    r.logger.Error("failed to persist dynamically registered agent", "agent", id, "err", err)
}
```

**文件**: `accounts.svc.plus/cmd/accountsvc/main.go`

```go
if agentRegistry != nil {
    agentRegistry.SetStore(st)
    agentRegistry.SetLogger(logger.With("component", "agent-registry"))
    // ... 其余代码
}
```

### 修复 3: 用户 UUID 变更

**连接数据库**:
```bash
ssh -i ~/.ssh/id_rsa root@postgresql.svc.plus
```

**执行 SQL 事务**:
```sql
BEGIN;

-- 1. 重命名旧用户（避免唯一约束冲突）
UPDATE users 
SET username = username || '_old', 
    email = email || '_old' 
WHERE uuid = '4b66928e-a81e-4981-bae0-289ddb92439c';

-- 2. 创建新用户记录（使用新 UUID）
INSERT INTO users (
    uuid, username, password, email, role, level, groups, permissions, 
    created_at, updated_at, version, origin_node, mfa_totp_secret, 
    mfa_enabled, mfa_secret_issued_at, mfa_confirmed_at, email_verified_at
)
SELECT 
    '18d270a9-533d-4b13-b3f1-e7f55540a9b2', 
    REPLACE(username, '_old', ''), 
    password, 
    REPLACE(email, '_old', ''), 
    role, level, groups, permissions, 
    created_at, updated_at, version, origin_node, mfa_totp_secret, 
    mfa_enabled, mfa_secret_issued_at, mfa_confirmed_at, email_verified_at
FROM users 
WHERE uuid = '4b66928e-a81e-4981-bae0-289ddb92439c';

-- 3. 更新所有外键引用
UPDATE identities 
SET user_uuid = '18d270a9-533d-4b13-b3f1-e7f55540a9b2' 
WHERE user_uuid = '4b66928e-a81e-4981-bae0-289ddb92439c';

UPDATE sessions 
SET user_uuid = '18d270a9-533d-4b13-b3f1-e7f55540a9b2' 
WHERE user_uuid = '4b66928e-a81e-4981-bae0-289ddb92439c';

UPDATE subscriptions 
SET user_uuid = '18d270a9-533d-4b13-b3f1-e7f55540a9b2' 
WHERE user_uuid = '4b66928e-a81e-4981-bae0-289ddb92439c';

-- 4. 删除旧用户记录
DELETE FROM users 
WHERE uuid = '4b66928e-a81e-4981-bae0-289ddb92439c';

COMMIT;
```

**执行命令**:
```bash
docker exec postgresql-svc-plus psql -U postgres -d account -c "
BEGIN;
UPDATE users SET username = username || '_old', email = email || '_old' WHERE uuid = '4b66928e-a81e-4981-bae0-289ddb92439c';
INSERT INTO users (uuid, username, password, email, role, level, groups, permissions, created_at, updated_at, version, origin_node, mfa_totp_secret, mfa_enabled, mfa_secret_issued_at, mfa_confirmed_at, email_verified_at)
SELECT '18d270a9-533d-4b13-b3f1-e7f55540a9b2', REPLACE(username, '_old', ''), password, REPLACE(email, '_old', ''), role, level, groups, permissions, created_at, updated_at, version, origin_node, mfa_totp_secret, mfa_enabled, mfa_secret_issued_at, mfa_confirmed_at, email_verified_at
FROM users WHERE uuid = '4b66928e-a81e-4981-bae0-289ddb92439c';
UPDATE identities SET user_uuid = '18d270a9-533d-4b13-b3f1-e7f55540a9b2' WHERE user_uuid = '4b66928e-a81e-4981-bae0-289ddb92439c';
UPDATE sessions SET user_uuid = '18d270a9-533d-4b13-b3f1-e7f55540a9b2' WHERE user_uuid = '4b66928e-a81e-4981-bae0-289ddb92439c';
UPDATE subscriptions SET user_uuid = '18d270a9-533d-4b13-b3f1-e7f55540a9b2' WHERE user_uuid = '4b66928e-a81e-4981-bae0-289ddb92439c';
DELETE FROM users WHERE uuid = '4b66928e-a81e-4981-bae0-289ddb92439c';
COMMIT;
"
```

## 验证方法

### 1. 验证 Agent 通信
```bash
# 查看 accounts-svc-plus 日志，确认没有 404 错误
gcloud run services logs read accounts-svc-plus \
  --project=xzerolab-480008 \
  --region=asia-northeast1 \
  --limit=50 | grep "agent-server"

# 应该看到 200 状态码
```

### 2. 验证 UUID 变更
```bash
# 查询新 UUID
docker exec postgresql-svc-plus psql -U postgres -d account -c "
  SELECT uuid, username, email 
  FROM users 
  WHERE email = 'tester123@example.com';
"

# 预期结果:
#                  uuid                 | username  |         email         
# --------------------------------------+-----------+-----------------------
#  18d270a9-533d-4b13-b3f1-e7f55540a9b2 | tester123 | tester123@example.com
```

### 3. 验证关联数据
```bash
# 检查订阅是否正确关联
docker exec postgresql-svc-plus psql -U postgres -d account -c "
  SELECT user_uuid, external_id, status 
  FROM subscriptions 
  WHERE user_uuid = '18d270a9-533d-4b13-b3f1-e7f55540a9b2';
"

# 检查会话是否正确关联
docker exec postgresql-svc-plus psql -U postgres -d account -c "
  SELECT user_uuid, expires_at 
  FROM sessions 
  WHERE user_uuid = '18d270a9-533d-4b13-b3f1-e7f55540a9b2';
"
```

### 4. 验证前端显示
```bash
# 访问 https://www.svc.plus/panel/agent
# 确认页面能够正常加载（虽然可能因为 401 错误暂时无数据）
```

## 回滚计划

### 如果 Agent 代理路由导致问题
```bash
# 删除代理路由文件
rm /Users/shenlan/workspaces/cloud-neutral-toolkit/console.svc.plus/src/app/api/agent-server/[...segments]/route.ts

# 重新构建和部署
cd /Users/shenlan/workspaces/cloud-neutral-toolkit/console.svc.plus
npm run build
# 部署到 Cloud Run
```

### 如果 UUID 变更导致问题
```sql
-- 反向操作（需要提前备份数据）
BEGIN;

-- 重命名当前用户
UPDATE users 
SET username = username || '_new', 
    email = email || '_new' 
WHERE uuid = '18d270a9-533d-4b13-b3f1-e7f55540a9b2';

-- 恢复旧 UUID
INSERT INTO users (uuid, username, password, email, ...)
SELECT '4b66928e-a81e-4981-bae0-289ddb92439c', 
       REPLACE(username, '_new', ''), 
       ...
FROM users 
WHERE uuid = '18d270a9-533d-4b13-b3f1-e7f55540a9b2';

-- 更新外键
UPDATE identities SET user_uuid = '4b66928e-a81e-4981-bae0-289ddb92439c' 
WHERE user_uuid = '18d270a9-533d-4b13-b3f1-e7f55540a9b2';

UPDATE sessions SET user_uuid = '4b66928e-a81e-4981-bae0-289ddb92439c' 
WHERE user_uuid = '18d270a9-533d-4b13-b3f1-e7f55540a9b2';

UPDATE subscriptions SET user_uuid = '4b66928e-a81e-4981-bae0-289ddb92439c' 
WHERE user_uuid = '18d270a9-533d-4b13-b3f1-e7f55540a9b2';

-- 删除新记录
DELETE FROM users WHERE uuid = '18d270a9-533d-4b13-b3f1-e7f55540a9b2';

COMMIT;
```

## 已知问题

### 1. `/api/agent/nodes` 返回 401 错误
- **现象**: 前端访问 `/api/agent/nodes` 时收到 401 Unauthorized
- **原因**: 认证 token 未正确传递到该端点
- **影响**: 用户无法查看节点列表
- **状态**: 待修复
- **临时方案**: 直接访问后端 API 或使用 admin 账户

### 2. 数据库 schema 不一致
- **现象**: 代码中使用 `active` 字段，但生产数据库表中不存在
- **影响**: 可能导致某些功能异常
- **状态**: 需要数据库迁移
- **建议**: 执行 schema 更新脚本

## 相关文档

- [Agent 架构文档](../docs/agent-architecture.md)
- [数据库 Schema](../sql/schema.sql)
- [API 路由配置](../api/api.go)

## 附录

### 数据库连接信息
```bash
# SSH 连接
ssh -i ~/.ssh/id_rsa root@postgresql.svc.plus

# Docker 容器名称
postgresql-svc-plus

# 数据库名称
account

# 用户名
postgres

# 密码
见 .env 文件
```

### 相关服务
- **accounts-svc-plus**: Cloud Run 服务，处理认证和用户管理
- **console.svc.plus**: 前端控制台
- **agent.svc.plus**: Agent 服务节点

### 监控和日志
```bash
# 查看 Cloud Run 日志
gcloud run services logs read accounts-svc-plus \
  --project=xzerolab-480008 \
  --region=asia-northeast1

# 查看数据库日志
ssh -i ~/.ssh/id_rsa root@postgresql.svc.plus \
  "docker logs postgresql-svc-plus --tail=100"
```
