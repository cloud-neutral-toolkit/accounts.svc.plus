# GitHub OAuth UAT → PROD 发布 TL;DR

本文是 `accounts` 与 `portal` 的 GitHub OAuth 环境配置契约。目标是以下三个入口都能完成 GitHub 登录：

```text
UAT            https://console-cloudflare-uat.onwalk.net/login
PROD 主入口    https://console.svc.plus/login
PROD Serverless https://console-serverless-prod.svc.plus/login
```

## 结论

```text
PROD pipeline → kv/data/prod/accounts/* → PROD Accounts service
UAT pipeline  → kv/data/uat/accounts/*  → UAT Accounts service
```

- GitOps 保存非敏感配置：`enabled`、`client_id`、回调 URL、前端 URL，以及对应 Vault 路径。
- Vault 只保存敏感 OAuth 值：`client_secret`。
- UAT 与 PROD 使用不同 GitHub OAuth App、不同 Client ID、不同 Client Secret。
- `console-serverless-prod.svc.plus` 属于 PROD，复用 PROD OAuth App；它的 `frontend_url` 必须加入 PROD Accounts 的允许来源，但 GitHub callback 仍使用规范的 `accounts.svc.plus` 地址。
- 不在源码、GitOps、GitHub Variables 或镜像中保存 Client Secret。

## 1. 环境、入口与 OAuth App

| 环境 | GitHub OAuth App name | Console 入口 | Accounts 入口 | GitHub callback |
|---|---|---|---|---|
| UAT | `onwalk.net Console (UAT)` | `https://console-cloudflare-uat.onwalk.net` | `https://accounts-cloudflare-uat.onwalk.net` | `https://accounts-cloudflare-uat.onwalk.net/api/auth/oauth/callback/github` |
| PROD | `svc.plus Console (PROD)` | `https://console.svc.plus` | `https://accounts.svc.plus` | `https://accounts.svc.plus/api/auth/oauth/callback/github` |
| PROD Serverless | 复用 `svc.plus Console (PROD)` | `https://console-serverless-prod.svc.plus` | `https://accounts.svc.plus` | `https://accounts.svc.plus/api/auth/oauth/callback/github` |

GitHub Developer Settings 中两套 App 均按以下选项创建：

- **Allow wildcard matching**：关闭
- **Enable Device Flow**：关闭
- **Expire user access tokens**：开启
- callback URL 使用精确的环境地址，不使用通配符

GitHub Client ID 是公开标识，可写入 GitOps；Client Secret 只能写入对应环境的 Vault。

## 2. GitOps 路径

仓库：`/Users/shenlan/workspaces/ai-workspace-infra/gitops`

当前已落地的 GitHub 配置文件：

```text
services/accounts/uat/oauth/github.json
services/accounts/prod/oauth/github.json
```

对应 Vault 路径由文件中的 `vault_secret_path` 声明：

```text
kv/data/uat/accounts/oauth/github
kv/data/prod/accounts/oauth/github
```

UAT GitOps JSON：

```json
{
  "enabled": true,
  "client_id": "Ov23lig2e4djTzbo3vo1",
  "redirect_url": "https://accounts-cloudflare-uat.onwalk.net/api/auth/oauth/callback/github",
  "frontend_url": "https://console-cloudflare-uat.onwalk.net",
  "vault_secret_path": "kv/data/uat/accounts/oauth/github",
  "vault_secret_key": "client_secret"
}
```

PROD GitOps JSON：

```json
{
  "enabled": true,
  "client_id": "Ov23lijWHcN9Xa9HANCd",
  "redirect_url": "https://accounts.svc.plus/api/auth/oauth/callback/github",
  "frontend_url": "https://console.svc.plus",
  "vault_secret_path": "kv/data/prod/accounts/oauth/github",
  "vault_secret_key": "client_secret"
}
```

`console-serverless-prod.svc.plus` 是同一 PROD OAuth App 的额外前端入口，不新增 GitOps Secret，也不新增 GitHub callback；部署 PROD Accounts 时将该域名加入 `ALLOWED_ORIGINS`。

## 3. Vault KV 结构

Vault KV v2 的完整数据只包含敏感字段：

```json
{
  "client_secret": "<environment-github-client-secret>"
}
```

路径与用途：

| 环境 | Vault KV path | 保存字段 | 消费服务 |
|---|---|---|---|
| UAT | `kv/data/uat/accounts/oauth/github` | `client_secret` | UAT Accounts |
| PROD | `kv/data/prod/accounts/oauth/github` | `client_secret` | PROD Accounts |

写入或读取时使用 Vault CLI 的 KV v2 语义，例如：

```bash
vault kv put kv/uat/accounts/oauth/github client_secret='<UAT_SECRET>'
vault kv put kv/prod/accounts/oauth/github client_secret='<PROD_SECRET>'
vault kv get kv/uat/accounts/oauth/github
vault kv get kv/prod/accounts/oauth/github
```

命令中的 secret 只作为占位符示例，禁止把真实值写入 shell history、日志或文档。

## 4. Backend 配置契约

`config/account-uat.yaml` 与 `config/account-prod.yaml` 通过部署环境注入以下变量。配置模板经过 `envsubst` 渲染，使用纯 `${VAR}`，不使用 `${VAR:-default}`：

```yaml
auth:
  enable: true
  oauth:
    frontendUrl: "${OAUTH_FRONTEND_URL}"
    github:
      clientId: "${GITHUB_CLIENT_ID}"
      clientSecret: "${GITHUB_CLIENT_SECRET}"
      redirectUrl: "${OAUTH_GITHUB_REDIRECT_URL}"
```

环境映射：

| 变量 | UAT | PROD |
|---|---|---|
| `GITHUB_CLIENT_ID` | GitOps UAT `client_id` | GitOps PROD `client_id` |
| `GITHUB_CLIENT_SECRET` | Vault UAT `client_secret` | Vault PROD `client_secret` |
| `OAUTH_FRONTEND_URL` | `https://console-cloudflare-uat.onwalk.net` | `https://console.svc.plus` |
| `OAUTH_GITHUB_REDIRECT_URL` | UAT GitHub callback | PROD GitHub callback |
| `ALLOWED_ORIGINS` | UAT Console + Accounts origins | `console.svc.plus` + `console-serverless-prod.svc.plus` + Accounts origin |

部署适配器负责把 GitOps 的非敏感字段和 Vault 的 `client_secret` 合并后注入容器；Accounts 代码不直接访问 Vault，也不把 secret 写回配置仓库。

## 5. Frontend 配置与路由

`portal/src/app/api/auth/oauth/login/[provider]/route.ts` 在服务端根据当前请求 host 解析 Accounts API：

- `console-cloudflare-uat.onwalk.net` → UAT runtime config → `accounts-cloudflare-uat.onwalk.net`
- `console.svc.plus` → PROD runtime config → `accounts.svc.plus`
- `console-serverless-prod.svc.plus` → PROD runtime config → `accounts.svc.plus`

`frontend_url` 使用浏览器当前 origin 传给 Accounts。Accounts 只接受配置的环境来源，因此 Serverless PROD 入口必须在 PROD `ALLOWED_ORIGINS` 中声明。

前端不读取、不构造、不转发 Client Secret。OAuth 登录按钮只触发：

```text
{accounts_origin}/api/auth/oauth/login/github
```

## 6. UAT → PROD 发布顺序

1. 在 GitHub Developer Settings 创建或确认 UAT App 与 PROD App，callback 精确匹配本表。
2. 将 UAT/PROD Client Secret 分别写入对应 Vault KV path；不得交叉复用。
3. 确认 GitOps 文件中的 `enabled`、Client ID、URL 与 Vault path 一致。
4. 发布 UAT pipeline，注入 UAT GitOps + UAT Vault，验证 UAT 登录。
5. 仅在 UAT 验证通过后，把同一变更提升到 PROD pipeline，注入 PROD GitOps + PROD Vault。
6. PROD 发布前确认 `console-serverless-prod.svc.plus` 已加入 PROD `ALLOWED_ORIGINS`。
7. 发布后分别验证三个入口的 GitHub 登录闭环。

## 7. 验证清单

先检查 Accounts login endpoint 是否返回 GitHub 授权跳转：

```bash
curl -fsSIL "https://accounts-cloudflare-uat.onwalk.net/api/auth/oauth/login/github"
curl -fsSIL "https://accounts.svc.plus/api/auth/oauth/login/github"
```

浏览器验证：

```text
https://console-cloudflare-uat.onwalk.net/login
https://console.svc.plus/login
https://console-serverless-prod.svc.plus/login
```

成功标准：

- 登录按钮跳转到 GitHub 授权页，且 `client_id` 属于对应环境 App。
- GitHub callback 无 `redirect_uri_mismatch`。
- callback 完成后回到发起登录的 Console origin，而不是固定回 PROD 主入口。
- UAT 不出现 `accounts.svc.plus`；Serverless PROD 不出现 UAT Accounts 地址。
- `/api/ping` 的发布镜像、commit 与 pipeline 发布版本一致。

常见故障：

| 现象 | 优先检查 |
|---|---|
| `provider_not_found` | `enabled`、Client ID/Secret 注入、`auth.enable` |
| `redirect_uri_mismatch` | GitHub App callback 与 `OAUTH_GITHUB_REDIRECT_URL` 是否逐字符一致 |
| UAT 跳到 PROD | portal UAT runtime config、请求 host 路由、`ACCOUNT_SERVICE_URL` 是否覆盖了 UAT |
| Serverless PROD 回调被拒绝 | PROD `ALLOWED_ORIGINS` 是否包含 `https://console-serverless-prod.svc.plus` |
| GitOps 泄露 secret | 检查提交内容；GitOps 只允许 `vault_secret_path`/`vault_secret_key`，禁止 `client_secret` |
