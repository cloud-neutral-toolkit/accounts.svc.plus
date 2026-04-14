# Error Conventions / 错误约定

## 中文

### 通用原则

当前 API 没有单独的全局 error schema package，而是由几类 handler 约定共同形成：

1. 大多数业务 handler 使用 `respondError`，返回统一 envelope。
2. 一部分 internal / agent / GORM 简单 handler 直接 `c.JSON`，只返回 `error` 或 `error + message`。
3. 因此文档层面必须区分“稳定错误结构”和“简化错误结构”，不能假设所有接口完全一致。

### 标准错误 envelope

大多数认证、账户、管理员、XWorkmate、计费接口的失败响应是：

```json
{"error":"code","message":"human readable message"}
```

字段含义：

| 字段 | 含义 |
| --- | --- |
| `error` | 机器可读错误码，通常稳定，适合前端做分支。 |
| `message` | 面向人类的错误描述。 |

### 扩展错误 envelope

少数接口会在标准形状上附加字段，例如：

| 场景 | 额外字段 |
| --- | --- |
| MFA 锁定 | `retryAt`、`mfaToken` |
| Admin settings 乐观并发冲突 | `version`、`matrix` |
| Sandbox binding / GORM 错误 | 有时直接返回底层 `err.Error()` 作为 `message` |
| `/api/ping` | 不走错误 envelope；是纯成功读取接口 |

### 简化错误 envelope

以下 handler 家族经常直接返回简化结构：

```json
{"error":"some_code"}
```

或

```json
{"error":"some_code","message":"..."}
```

典型来源：

| 文件 | 典型接口 |
| --- | --- |
| `api/agent_server.go` | `/api/agent-server/v1/users`、`/status` |
| `api/user_agents.go` | `/api/agent-server/v1/nodes`、legacy `/api/agent/nodes` |
| `api/internal_sandbox_guest.go` | `/api/internal/sandbox/guest` |
| `api/admin_sandbox.go` | `/api/auth/admin/sandbox/*`、`/api/admin/sandbox/*` |
| `internal/auth/middleware.go` | JWT / internal-service middleware 直接中断请求时 |

### 错误来源分层

| 来源层 | 典型错误码 / 文本 | 说明 |
| --- | --- | --- |
| 请求绑定与参数校验 | `invalid_request`、`missing_credentials`、`invalid_email`、`password_too_short` | JSON body 缺字段、query 中带敏感参数、格式不对。 |
| Session / JWT / internal token | `session_token_required`、`invalid_session`、`missing authorization header`、`invalid or expired token`、`missing service token` | 认证前置失败。 |
| 角色 / 权限 / 账户状态 | `forbidden`、`root_only`、`account_suspended`、`read_only_account`、`root_email_enforced` | 用户存在，但当前身份不允许执行操作。 |
| 业务状态 | `email_already_exists`、`subscription_not_found`、`mfa_not_enabled`、`policy_not_found` | 领域对象状态不满足当前请求。 |
| 外部系统或后台依赖 | `verification_failed`、`xworkmate_secret_write_failed`、`stripe_cancel_failed`、`collector_status_unavailable` | SMTP、Vault、Stripe、DB、Xray render 等依赖失败。 |

### 高频错误码索引

#### 认证与会话

| 错误码 | 常见状态码 | 来源 | 含义 |
| --- | --- | --- | --- |
| `credentials_in_query` | `400` | `api/api.go` | 登录或敏感接口不允许把凭据放在 query 中。 |
| `missing_credentials` | `400` | `api/api.go` | 登录、注册缺少关键字段。 |
| `user_not_found` | `404` | `api/api.go`、`api/user_agents.go` | 通过 identifier 或 session userID 找不到用户。 |
| `invalid_credentials` | `401` | `api/api.go` | 密码校验失败。 |
| `email_not_verified` | `401` | `api/api.go` | 邮箱尚未验证，禁止登录或 OAuth 登录。 |
| `session_token_required` | `401` | `api/api.go`、`api/xworkmate.go`、`api/admin_users_metrics.go` | 需要 session token。 |
| `invalid_session` | `401` | 多个 session-based handler | session 不存在、过期或无法匹配。 |
| `session_user_lookup_failed` | `500` | 多个 handler | session 存在，但无法回查用户。 |

#### MFA

| 错误码 | 常见状态码 | 来源 | 含义 |
| --- | --- | --- | --- |
| `mfa_ticket_required` | `400` | `verifyMFALogin` | 登录挑战完成接口缺少 ticket。 |
| `mfa_code_required` | `400` | `verifyMFALogin`、`verifyTOTP` | 缺少 TOTP 验证码。 |
| `invalid_mfa_ticket` | `401` | `verifyMFALogin` | 登录用 MFA challenge 已失效。 |
| `invalid_mfa_token` | `401` | `provisionTOTP`、`verifyTOTP` | TOTP provisioning / verify 的 token 不存在或过期。 |
| `invalid_mfa_code` | `401` / `500` | MFA handlers | 验证码错误，或库校验过程失败。 |
| `mfa_challenge_locked` | `429` | `verifyTOTP` | 连续错误次数过多，被暂时锁定。 |
| `mfa_not_enabled` | `400` | `verifyMFALogin`、`disableMFA` | 用户当前没有启用 MFA。 |

#### 注册、邮箱验证、密码重置

| 错误码 | 常见状态码 | 来源 | 含义 |
| --- | --- | --- | --- |
| `verification_required` | `400` | `register` | 已启用邮箱验证，但未提供验证码。 |
| `invalid_code` | `400` | `register`、`verifyEmail` | 验证码错误或过期。 |
| `verification_failed` | `500` | 邮件验证流程 | SMTP 发送或用户更新失败。 |
| `email_already_exists` | `409` | `register`、`sendEmailVerification` | 邮箱已存在或已验证。 |
| `name_already_exists` | `409` | `register` | 用户名冲突。 |
| `password_reset_failed` | `500` | password reset flows | 密码重置邮件发送、用户更新或其他内部步骤失败。 |
| `invalid_token` | `400` | `confirmPasswordReset` | reset token 无效或过期。 |

#### OAuth / token exchange / refresh

| 错误码 | 常见状态码 | 来源 | 含义 |
| --- | --- | --- | --- |
| `provider_not_found` | `404` | OAuth routes | provider 未注册。 |
| `code_missing` | `400` | `oauthCallback` | callback query 中缺少 `code`。 |
| `oauth_exchange_failed` | `500` | `oauthCallback` | 与 provider 交换 token 失败。 |
| `fetch_profile_failed` | `500` | `oauthCallback` | 拉取 provider profile 失败。 |
| `invalid_exchange_code` | `401` | `exchangeToken` | 一次性 exchange code 无效或已过期。 |
| `token_service_unavailable` | `503` | `refreshToken` | 当前未启用 token service。 |
| `invalid_refresh_token` | `401` | `refreshToken` | refresh token 无效或已过期。 |

#### 管理员与权限

| 错误码 | 常见状态码 | 来源 | 含义 |
| --- | --- | --- | --- |
| `forbidden` | `403` | `requireAdminPermission`、`RequireRole` | 用户权限不足。 |
| `root_only` | `403` | sandbox bind / assume / tenant bootstrap | 只有 root 可执行。 |
| `root_email_enforced` | `403` | `requireAdminPermission` | root role 被限制给 `admin@svc.plus`。 |
| `metrics_unavailable` | `503` / 其他 | `adminUsersMetrics` | 指标 provider 未配置或执行失败。 |
| `read_only_account` | `403` | 多个写接口 | demo/read-only 账号禁止写操作。 |
| `account_suspended` | `403` | session user checks | 账号被暂停。 |

#### XWorkmate / Vault

| 错误码 | 常见状态码 | 来源 | 含义 |
| --- | --- | --- | --- |
| `xworkmate_vault_unavailable` | `503` | `ensureXWorkmateVaultService` | 当前未注入 Vault backend。 |
| `tenant_membership_required` | `403` | `xworkmate.go` | 当前用户没有 tenant membership。 |
| `tenant_not_found` | `404` | `xworkmate.go` | host 解析出的 tenant 不存在。 |
| `xworkmate_profile_forbidden` | `403` | `updateXWorkmateProfile` | 无权修改 tenant integration profile。 |
| `token_persistence_forbidden` | `400` | `updateXWorkmateProfile` | 禁止把 raw token/password/api key 直接落库。 |
| `xworkmate_secret_unknown_target` | `400` | secret PUT/DELETE | `:target` 不在允许列表内。 |
| `xworkmate_secret_write_failed` | `500` | secret PUT | 写入 Vault/backend 失败。 |
| `xworkmate_secret_delete_failed` | `500` | secret DELETE | 删除 Vault/backend secret 失败。 |

#### 计费、订阅、流量与调度

| 错误码 | 常见状态码 | 来源 | 含义 |
| --- | --- | --- | --- |
| `subscription_not_found` | `404` | `cancelSubscription` | 订阅不存在。 |
| `subscription_upsert_failed` | `500` | `upsertSubscription` | 订阅落库失败。 |
| `stripe_cancel_failed` | `502` | `cancelSubscription` | 调用 Stripe 取消失败。 |
| `usage_summary_unavailable` | `500` | `accountUsageSummary` | 读取使用量汇总失败。 |
| `invalid_start` / `invalid_end` | `400` | `accountUsageBuckets` | 时间范围参数不是 RFC3339。 |
| `policy_not_found` | `404` | `accountPolicy`、`internalAccountPolicy` | 账户策略快照不存在。 |
| `collector_status_unavailable` | `500` | `adminCollectorStatus` | collector 读面不可用。 |
| `scheduler_status_unavailable` | `500` | `adminSchedulerStatus` | 调度决策读面不可用。 |

#### Agent 与内部服务

| 错误码 / 文本 | 常见状态码 | 来源 | 含义 |
| --- | --- | --- | --- |
| `missing_token` | `401` | `api/agent_server.go` | agent token 缺失。 |
| `invalid_token` | `401` | `api/agent_server.go` | agent token 无效。 |
| `agent_registry_unavailable` | `503` | `api/agent_server.go` | 未注入 `agentserver.Registry`。 |
| `list_users_failed` | `500` | `internalPublicOverview`、`listAgentUsers`、`internalNetworkIdentities` | 枚举用户失败。 |
| `store_not_configured` / `store_unavailable` | `503` | internal handlers | store 未配置。 |
| `sandbox_missing` | `404` | sandbox guest handlers | sandbox 用户不存在。 |

### 调用方建议

1. 对 `respondError` 路由，优先按 `error` 做程序分支，`message` 仅用于展示。
2. 对 agent/internal 路由，不要假设一定有 `message`。
3. 对 `429 mfa_challenge_locked`，调用方应读取 `retryAt` 决定 UI 倒计时。
4. 对 `POST /api/auth/admin/settings` 的 `409`，应读取返回的 `version` 与 `matrix` 重新加载服务端最新值。

## English

### Core Rule

The current API does not have a single shared error-schema package. Instead, error behavior is shaped by a few implementation conventions:

1. Most business handlers use `respondError`, which produces the standard envelope.
2. Some internal / agent / simple GORM-backed handlers call `c.JSON` directly and return only `error` or `error + message`.
3. Documentation therefore has to distinguish between the stable error envelope and simplified handler-local errors.

### Standard Error Envelope

Most authentication, account, admin, XWorkmate, and accounting handlers return:

```json
{"error":"code","message":"human readable message"}
```

Field meaning:

| Field | Meaning |
| --- | --- |
| `error` | Machine-readable error code that is usually stable enough for frontend branching. |
| `message` | Human-readable explanation. |

### Extended Error Envelope

Some handlers add extra fields on top of the standard shape:

| Scenario | Extra fields |
| --- | --- |
| MFA lockout | `retryAt`, `mfaToken` |
| Admin settings optimistic conflict | `version`, `matrix` |
| Sandbox binding / GORM errors | Sometimes the raw `err.Error()` is surfaced in `message` |
| `/api/ping` | Not an error-envelope route; it is a pure success read |

### Simplified Error Envelope

The following handler families commonly return:

```json
{"error":"some_code"}
```

or:

```json
{"error":"some_code","message":"..."}
```

Typical sources:

| File | Example APIs |
| --- | --- |
| `api/agent_server.go` | `/api/agent-server/v1/users`, `/status` |
| `api/user_agents.go` | `/api/agent-server/v1/nodes`, legacy `/api/agent/nodes` |
| `api/internal_sandbox_guest.go` | `/api/internal/sandbox/guest` |
| `api/admin_sandbox.go` | `/api/auth/admin/sandbox/*`, `/api/admin/sandbox/*` |
| `internal/auth/middleware.go` | Direct JWT / internal-service middleware rejection responses |

### Error Sources By Layer

| Source layer | Typical codes / texts | Meaning |
| --- | --- | --- |
| Request binding and validation | `invalid_request`, `missing_credentials`, `invalid_email`, `password_too_short` | Bad JSON bodies, forbidden query-string credentials, invalid formats. |
| Session / JWT / internal token | `session_token_required`, `invalid_session`, `missing authorization header`, `invalid or expired token`, `missing service token` | Authentication precondition failed. |
| Role / permission / account-state checks | `forbidden`, `root_only`, `account_suspended`, `read_only_account`, `root_email_enforced` | The user exists but is not allowed to perform the action. |
| Business-state failures | `email_already_exists`, `subscription_not_found`, `mfa_not_enabled`, `policy_not_found` | Domain state does not satisfy the requested transition. |
| External or backend dependency failures | `verification_failed`, `xworkmate_secret_write_failed`, `stripe_cancel_failed`, `collector_status_unavailable` | SMTP, Vault, Stripe, DB, Xray rendering, and similar dependency failures. |

### High-Frequency Error Index

#### Authentication And Sessions

| Code | Typical status | Source | Meaning |
| --- | --- | --- | --- |
| `credentials_in_query` | `400` | `api/api.go` | Sensitive credentials were sent through the query string. |
| `missing_credentials` | `400` | `api/api.go` | Required login or registration fields are missing. |
| `user_not_found` | `404` | `api/api.go`, `api/user_agents.go` | No user matches the identifier or session-derived user ID. |
| `invalid_credentials` | `401` | `api/api.go` | Password verification failed. |
| `email_not_verified` | `401` | `api/api.go` | The email is not verified, so login is blocked. |
| `session_token_required` | `401` | `api/api.go`, `api/xworkmate.go`, `api/admin_users_metrics.go` | A session token is required. |
| `invalid_session` | `401` | Multiple session-based handlers | The session does not exist, is expired, or cannot be matched. |
| `session_user_lookup_failed` | `500` | Multiple handlers | The session exists but the backing user cannot be loaded. |

#### MFA

| Code | Typical status | Source | Meaning |
| --- | --- | --- | --- |
| `mfa_ticket_required` | `400` | `verifyMFALogin` | The MFA completion endpoint is missing its ticket. |
| `mfa_code_required` | `400` | `verifyMFALogin`, `verifyTOTP` | No TOTP code was supplied. |
| `invalid_mfa_ticket` | `401` | `verifyMFALogin` | The login MFA challenge has expired or is invalid. |
| `invalid_mfa_token` | `401` | `provisionTOTP`, `verifyTOTP` | The provisioning / verification token does not exist or has expired. |
| `invalid_mfa_code` | `401` / `500` | MFA handlers | The code is wrong, or the verification library errored. |
| `mfa_challenge_locked` | `429` | `verifyTOTP` | Too many invalid attempts caused a temporary lockout. |
| `mfa_not_enabled` | `400` | `verifyMFALogin`, `disableMFA` | The user does not currently have MFA enabled. |

#### Registration, Email Verification, And Password Reset

| Code | Typical status | Source | Meaning |
| --- | --- | --- | --- |
| `verification_required` | `400` | `register` | Email verification is enabled but the code was not provided. |
| `invalid_code` | `400` | `register`, `verifyEmail` | The verification code is wrong or expired. |
| `verification_failed` | `500` | Email-verification flows | SMTP delivery or user-update work failed. |
| `email_already_exists` | `409` | `register`, `sendEmailVerification` | The email already exists or is already verified. |
| `name_already_exists` | `409` | `register` | Username conflict. |
| `password_reset_failed` | `500` | Password-reset flows | Email delivery, user update, or other internal reset steps failed. |
| `invalid_token` | `400` | `confirmPasswordReset` | The reset token is invalid or expired. |

#### OAuth, Token Exchange, And Refresh

| Code | Typical status | Source | Meaning |
| --- | --- | --- | --- |
| `provider_not_found` | `404` | OAuth routes | The provider is not registered. |
| `code_missing` | `400` | `oauthCallback` | The callback query does not contain `code`. |
| `oauth_exchange_failed` | `500` | `oauthCallback` | Exchanging the provider code failed. |
| `fetch_profile_failed` | `500` | `oauthCallback` | Fetching the provider profile failed. |
| `invalid_exchange_code` | `401` | `exchangeToken` | The one-time exchange code is invalid or expired. |
| `token_service_unavailable` | `503` | `refreshToken` | The token service is not enabled. |
| `invalid_refresh_token` | `401` | `refreshToken` | The refresh token is invalid or expired. |

#### Admin And Permission Errors

| Code | Typical status | Source | Meaning |
| --- | --- | --- | --- |
| `forbidden` | `403` | `requireAdminPermission`, `RequireRole` | The caller lacks the required permission. |
| `root_only` | `403` | Sandbox bind / assume / tenant bootstrap flows | Only the root user may perform the action. |
| `root_email_enforced` | `403` | `requireAdminPermission` | The root role is restricted to `admin@svc.plus`. |
| `metrics_unavailable` | `503` and others | `adminUsersMetrics` | The metrics provider is missing or failed. |
| `read_only_account` | `403` | Multiple write handlers | Demo/read-only accounts are blocked from writes. |
| `account_suspended` | `403` | Session user checks | The account has been suspended. |

#### XWorkmate And Vault

| Code | Typical status | Source | Meaning |
| --- | --- | --- | --- |
| `xworkmate_vault_unavailable` | `503` | `ensureXWorkmateVaultService` | No Vault backend has been injected. |
| `tenant_membership_required` | `403` | `xworkmate.go` | The caller has no tenant membership. |
| `tenant_not_found` | `404` | `xworkmate.go` | The host-resolved tenant does not exist. |
| `xworkmate_profile_forbidden` | `403` | `updateXWorkmateProfile` | The caller may not update the tenant integration profile. |
| `token_persistence_forbidden` | `400` | `updateXWorkmateProfile` | Raw token/password/api-key values may not be persisted directly. |
| `xworkmate_secret_unknown_target` | `400` | Secret PUT/DELETE handlers | The `:target` path value is outside the allowed set. |
| `xworkmate_secret_write_failed` | `500` | Secret PUT | Persisting the Vault/backend secret failed. |
| `xworkmate_secret_delete_failed` | `500` | Secret DELETE | Deleting the Vault/backend secret failed. |

#### Billing, Subscription, Traffic, And Scheduler Reads

| Code | Typical status | Source | Meaning |
| --- | --- | --- | --- |
| `subscription_not_found` | `404` | `cancelSubscription` | The subscription does not exist. |
| `subscription_upsert_failed` | `500` | `upsertSubscription` | Persisting the subscription failed. |
| `stripe_cancel_failed` | `502` | `cancelSubscription` | Stripe cancellation failed. |
| `usage_summary_unavailable` | `500` | `accountUsageSummary` | Usage summary reads failed. |
| `invalid_start` / `invalid_end` | `400` | `accountUsageBuckets` | The time-range query parameters are not valid RFC3339 timestamps. |
| `policy_not_found` | `404` | `accountPolicy`, `internalAccountPolicy` | No account policy snapshot is available. |
| `collector_status_unavailable` | `500` | `adminCollectorStatus` | Collector read models are unavailable. |
| `scheduler_status_unavailable` | `500` | `adminSchedulerStatus` | Scheduler decision reads are unavailable. |

#### Agent And Internal-Service Errors

| Code / text | Typical status | Source | Meaning |
| --- | --- | --- | --- |
| `missing_token` | `401` | `api/agent_server.go` | The agent token is missing. |
| `invalid_token` | `401` | `api/agent_server.go` | The agent token is invalid. |
| `agent_registry_unavailable` | `503` | `api/agent_server.go` | No `agentserver.Registry` has been injected. |
| `list_users_failed` | `500` | `internalPublicOverview`, `listAgentUsers`, `internalNetworkIdentities` | Enumerating users failed. |
| `store_not_configured` / `store_unavailable` | `503` | Internal handlers | The store has not been configured. |
| `sandbox_missing` | `404` | Sandbox guest handlers | The sandbox user does not exist. |

### Client Guidance

1. For `respondError`-based routes, branch on `error` first and treat `message` as display text.
2. For agent/internal routes, do not assume `message` is always present.
3. For `429 mfa_challenge_locked`, read `retryAt` and drive the UI countdown from it.
4. For `409` from `POST /api/auth/admin/settings`, reload the server-returned `version` and `matrix` before retrying.
