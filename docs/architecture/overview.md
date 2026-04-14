# Architecture Overview / 架构总览

## 中文

### 系统边界

`accounts.svc.plus` 是一个以 Gin 为 HTTP 入口的 Go 单体服务。它同时承担四类职责：

1. 账号与认证：注册、登录、会话、MFA、OAuth、密码重置。
2. 账号控制面：管理员权限矩阵、用户管理、Sandbox 假扮、黑名单。
3. Agent / Xray 控制：向 agent 提供客户端列表、接收 agent 心跳、按数据库状态生成 Xray 配置。
4. 使用量与计费读面：账户流量桶、账本、配额、策略、节点健康和调度决策读取。

### 主启动链路

`cmd/accountsvc/main.go` 的 server 路径可以概括为：

1. 读取配置并选择运行模式：`server`、`agent`、`server-agent`。
2. 初始化主 store 与管理面 GORM DB，后者负责 `admin_settings`、homepage video、sandbox binding、tenant / XWorkmate 相关模型。
3. 应用 RBAC schema，并确保 root / review / sandbox 用户以及相关体验性账户状态满足当前契约。
4. 根据 SMTP 配置决定是否启用真实邮件发送；未配置或是示例域名时自动退回“禁用邮件验证”。
5. 在 `auth.enable` 为真时构造 `auth.TokenService`，否则系统仍以 session token 主路径工作。
6. 构造 `agentserver.Registry`，将静态 credential、持久化 agent 状态、sandbox binding 预加载到内存读面。
7. 在 `xray.sync.enabled` 为真时启动 `xrayconfig.PeriodicSyncer`，以数据库为源周期性重建 Xray 配置并执行 validate / restart 命令。
8. 构造 `api.Option` 列表，把 store、mailer、token service、Stripe、Vault、OAuth provider、metrics provider、agent registry、GORM DB 注入 `api.RegisterRoutes`。
9. 启动 Gin HTTP 服务，对外提供 `/healthz`、`/api/auth/*`、`/api/internal/*`、`/api/admin/*`、`/api/agent-server/v1/*` 与 `/api/account/*`。

### 运行时主数据流

#### 1. 账号登录与会话

- `POST /api/auth/login` 读取 `store.Store` 中的用户和密码哈希。
- 成功后由 `handler.createSession` 在 session store 中落 token，再由 `sanitizeUser` 组装用户返回体。
- 若用户启用 MFA，则先返回 challenge，再由 `POST /api/auth/mfa/verify` 完成会话签发。

#### 2. 配置化管理面

- `internal/service/admin_settings.go` 使用 GORM 读写 `model.AdminSetting`。
- API 层通过 `requireAdminPermission` + `service.GetAdminSettings` / `SaveAdminSettings` 组合出“会话鉴权 + 权限矩阵”的控制面能力。
- `internal/service/homepage_video_settings.go` 复用同一套 GORM DB，负责首页视频默认项和域名覆盖项。

#### 3. Xray 配置生成

- Controller 模式下，`internal/xrayconfig.GormClientSource` 从数据库读取用户并转换为 `xrayconfig.Client`。
- `xrayconfig.Generator` 把 `Definition` 模板渲染成 JSON，再替换 `inbounds[0].settings.clients`。
- `xrayconfig.PeriodicSyncer` 周期性执行 `Generate`，可选执行 validate / restart 命令。
- Agent 模式下，`internal/agentmode.Client` 通过 `/api/agent-server/v1/users` 获取客户端列表，复用同一套 `PeriodicSyncer` 本地生成配置并上报 `/status`。

#### 4. 使用量与计费读取

- `internal/store.Store` 暴露分钟桶、账本、配额、账单 profile、策略快照、节点健康、调度决策等读取方法。
- `api/accounting.go` 将这些事实表组合为 `/api/account/*` 与 `/api/admin/traffic/*` 读接口。
- 这些接口的 source of truth 是 PostgreSQL 事实表，不依赖 Prometheus 聚合。

### 后台循环与状态持有者

- `agentserver.Registry` 在内存中持有 credential digest、agent 身份、最新状态快照和 sandbox agent 集。
- `handler` 在 API 层持有 session、MFA challenge、邮箱验证码、密码重置 token、OAuth exchange code 等进程内状态。
- `PeriodicSyncer` 和 `agentmode.runStatusReporter` 是主要后台循环；前者负责配置文件收敛，后者负责 agent 健康上报。

### 当前明确边界

- Session、MFA challenge、邮箱验证码、OAuth exchange code 都是进程内状态，不是横向可共享存储。
- JWT token service 是可选增强，不是当前主控制面的唯一鉴权来源；大部分业务仍围绕 session token 设计。
- GORM 只承载管理面模型，不替代主业务 store。
- `/api/agent/nodes` 是 legacy alias，规范路径是 `/api/agent-server/v1/nodes`。

## English

### System Scope

`accounts.svc.plus` is a Gin-based Go monolith. It owns four main responsibility areas:

1. Identity and authentication: registration, login, sessions, MFA, OAuth, password reset.
2. Account control plane: admin permission matrix, user operations, sandbox assume flows, blacklist management.
3. Agent / Xray control: serving client lists to agents, receiving agent heartbeats, generating Xray configs from database state.
4. Usage and billing read models: traffic buckets, ledger entries, quota state, policy snapshots, node health, and scheduler decisions.

### Main Startup Chain

The server path in `cmd/accountsvc/main.go` is:

1. Load config and choose runtime mode: `server`, `agent`, or `server-agent`.
2. Initialize the primary store plus a GORM-backed admin DB used for admin settings, homepage video, sandbox binding, and tenant / XWorkmate models.
3. Apply RBAC schema and normalize root / review / sandbox accounts so runtime invariants are satisfied.
4. Build the mailer if SMTP is configured; otherwise email verification is disabled automatically.
5. Build `auth.TokenService` only when `auth.enable` is true; the service still keeps session-token flows as the primary path.
6. Build `agentserver.Registry`, hydrate it from configured credentials and persisted agent rows, and preload sandbox bindings.
7. Start `xrayconfig.PeriodicSyncer` when `xray.sync.enabled` is true so Xray configs are regenerated from database state and optionally validated / restarted.
8. Build the `api.Option` list and inject store, mailer, token service, Stripe, Vault, OAuth providers, metrics provider, agent registry, and GORM DB into `api.RegisterRoutes`.
9. Start Gin and expose `/healthz`, `/api/auth/*`, `/api/internal/*`, `/api/admin/*`, `/api/agent-server/v1/*`, and `/api/account/*`.

### Primary Runtime Flows

#### 1. Identity and Session Flow

- `POST /api/auth/login` reads users and password hashes from `store.Store`.
- On success, `handler.createSession` writes a session token and `sanitizeUser` shapes the user payload.
- If MFA is enabled, login first returns a challenge and `POST /api/auth/mfa/verify` completes session issuance.

#### 2. Configurable Admin Surface

- `internal/service/admin_settings.go` reads and writes `model.AdminSetting` through GORM.
- The API layer combines `requireAdminPermission` with `service.GetAdminSettings` / `SaveAdminSettings` to enforce session-based access plus permission-matrix rules.
- `internal/service/homepage_video_settings.go` uses the same GORM DB for default and per-domain homepage video entries.

#### 3. Xray Config Generation

- In controller mode, `internal/xrayconfig.GormClientSource` loads users from the database and converts them into `xrayconfig.Client` records.
- `xrayconfig.Generator` renders a `Definition` template into JSON and replaces `inbounds[0].settings.clients`.
- `xrayconfig.PeriodicSyncer` repeatedly calls `Generate` and may run validate / restart commands.
- In agent mode, `internal/agentmode.Client` fetches `/api/agent-server/v1/users`, feeds the same `PeriodicSyncer`, and reports `/status`.

#### 4. Usage and Billing Reads

- `internal/store.Store` exposes traffic buckets, ledger, quota, billing profile, policy snapshot, node health, and scheduler decision reads.
- `api/accounting.go` composes those facts into `/api/account/*` and `/api/admin/traffic/*` responses.
- The source of truth for those reads is PostgreSQL-backed fact tables, not Prometheus.

### Background Loops and State Owners

- `agentserver.Registry` owns credential digests, agent identities, latest status snapshots, and sandbox-agent flags in memory.
- API `handler` owns process-local session, MFA challenge, email verification, password reset, and OAuth exchange state.
- `PeriodicSyncer` and `agentmode.runStatusReporter` are the main background loops for config convergence and agent health reporting.

### Current Hard Boundaries

- Sessions, MFA challenges, email verification state, and OAuth exchange codes are process-local, not horizontally shared.
- JWT is optional and not the sole authentication source for the current control plane; most business flows are still session-centric.
- GORM is used for admin-side models only and does not replace the primary business store abstraction.
- `/api/agent/nodes` is a legacy alias; `/api/agent-server/v1/nodes` is the canonical route family.
