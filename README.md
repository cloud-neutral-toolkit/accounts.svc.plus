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
  | bash -s -- accounts.svc.plus --mode process --deploy
```

Docker 部署模式：

```bash
curl -fsSL "https://raw.githubusercontent.com/cloud-neutral-toolkit/accounts.svc.plus/main/scripts/setup.sh?$(date +%s)" \
  | bash -s -- accounts.svc.plus --mode docker --deploy
```

Cloud Run 部署模式：

```bash
curl -fsSL "https://raw.githubusercontent.com/cloud-neutral-toolkit/accounts.svc.plus/main/scripts/setup.sh?$(date +%s)" \
  | bash -s -- accounts.svc.plus --mode cloudrun
```

单机 `process` / `docker` 模式默认会写入 Caddy 站点配置到 `/etc/caddy/conf.d/accounts.svc.plus.conf`，并反向代理到本机 `127.0.0.1:8080`。

### 本地运行 (Local Dev)

```bash
cp .env.example .env
make dev
```

## 提交前同步要求 (Pre-Commit Sync Requirement)

控制仓库中的 `subrepos/accounts.svc.plus` 在每次提交前，必须先同步当前线上运行实例。

线上实例命名规则：

- `<server-name>-<hostname-or-env>-<git-commit-short-id>.<domain>`

例如：

- `accounts-us-xhttp-2886a64.svc.plus`

执行要求：

```bash
cd /Users/shenlan/workspaces/cloud-neutral-toolkit/github-org-cloud-neutral-toolkit/subrepos/accounts.svc.plus

# 1. 确认线上当前运行 revision / image
ssh root@us-xhttp.svc.plus 'docker ps --format "table {{.Names}}\t{{.Image}}\t{{.RunningFor}}" | grep accounts'

# 2. 动态定位当前 active accounts 实例并核对 compose 目录与镜像 tag
ssh root@us-xhttp.svc.plus '
name=$(docker ps --format "{{.Names}}" | grep "^accounts-" | head -n 1) &&
echo "$name" &&
sed -n "1,80p" "/opt/cloud-neutral/accounts/${name}/docker-compose.yml"
'

# 3. 再开始本地提交
git status
```

如果线上 revision 已变化，应先以新的 `<server-name>-<hostname-or-env>-<git-commit-short-id>.<domain>` 实例为准完成同步，再提交本地改动。

## Stripe 配置 (Stripe Billing Setup)

Stripe 相关服务端能力现在由 `accounts.svc.plus` 承担，包括：

- Checkout Session 创建
- Customer Portal 跳转
- Webhook 验签与订阅状态回写

需要的环境变量：

| 变量 | 用途 |
| --- | --- |
| `STRIPE_SECRET_KEY` | Stripe API secret key |
| `STRIPE_WEBHOOK_SECRET` | Stripe webhook endpoint secret |
| `STRIPE_ALLOWED_PRICE_IDS` | 允许下单的 `price_...` 白名单，逗号分隔 |

联调说明见 `docs/usage/stripe-billing.md`。

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
- Stripe 联调：`docs/usage/stripe-billing.md`
- 部署方式：`docs/usage/deployment.md`
- API 参考：`docs/api/overview.md`
- 运维：`docs/operations/monitoring.md`, `docs/operations/troubleshooting.md`
- Runbooks：`docs/Runbook/README.md`
