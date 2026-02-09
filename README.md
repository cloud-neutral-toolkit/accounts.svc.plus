# accounts.svc.plus

Cloud Neutral Toolkit 的账号与身份服务 (Account Service).

> A production-oriented account service for sign-in, sessions, MFA, and agent coordination.

## 部署要求 (Deployment Requirements)

| 维度 | 要求 / 规格 | 说明 |
|---|---|---|
| 网络 | 可访问的 API 域名 (可选) | 生产建议配置 `server.publicUrl` |
| 端口 | `:8080` | API 服务默认监听端口 |
| 数据库 | PostgreSQL | 存储账号/会话/状态等核心数据 |
| 缓存 (可选) | Redis | `session.cache=redis` 时需要 |
| 最低 | 1 CPU / 1GB RAM | 开发/小规模 |
| 推荐 | 2 CPU / 2GB RAM | 生产建议 |

## 快速开始 (Quickstart)

### 一键初始化 (Setup Script)

```bash
curl -fsSL "https://raw.githubusercontent.com/cloud-neutral-toolkit/accounts.svc.plus/main/scripts/setup.sh?$(date +%s)" \
  | bash -s -- accounts.svc.plus
```

### 本地运行 (Local Dev)

```bash
cp .env.example .env
make dev
```

## 核心特性 & 技术栈 (Features & Tech Stack)

核心特性：
- 账号体系：注册/登录/会话/角色与权限
- 安全能力：邮件验证、TOTP MFA（可选）
- Agent 协同：与节点/控制面协作的同步与状态上报
- 多部署形态：本地/VM、Docker、Cloud Run（含 stunnel sidecar 示例）

技术栈：
- Go + Gin
- PostgreSQL (primary store)
- Redis (optional session cache)
- stunnel (optional secure DB connectivity; Cloud Run example included)

## 说明文档 (Docs)

- 文档入口：`docs/README.md`
- 快速开始：`docs/getting-started/quickstart.md`
- 配置说明：`docs/usage/config.md`
- 部署方式：`docs/usage/deployment.md`
- API 参考：`docs/api/overview.md`
- 运维：`docs/operations/monitoring.md`, `docs/operations/troubleshooting.md`
- Runbooks：`docs/Runbook/README.md`

