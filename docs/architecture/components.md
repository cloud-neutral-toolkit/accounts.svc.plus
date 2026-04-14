# Component Responsibilities / 组件职责边界

## 中文

### 包级职责矩阵

| 包 / 目录 | owning responsibility | 主要输入 | 主要输出 | 直接协作对象 |
| --- | --- | --- | --- | --- |
| `cmd/accountsvc` | 进程装配、模式切换、依赖注入、后台循环启动 | `config.Config`、环境变量、数据库连接 | Gin 服务、agent loop、xray sync loop | `api`、`store`、`auth`、`service`、`agentserver`、`agentmode`、`xrayconfig` |
| `api` | HTTP 路由注册、请求校验、鉴权拼装、响应 shape | Gin request、session token、service/store 依赖 | JSON 响应、cookie、错误结构 | `store`、`auth`、`service`、`agentserver`、`xrayconfig` |
| `internal/store` | 主业务持久化抽象与实现 | 领域模型、SQL / memory state | `Store` 接口、domain types、读写行为 | `api`、`service`、`cmd/accountsvc` |
| `internal/auth` | token service、OAuth provider、中间件上下文写入 | JWT secret、OAuth config、session token | `TokenService`、`OAuthProvider`、Gin middleware | `api`、`cmd/accountsvc` |
| `internal/service` | 管理面业务逻辑与聚合读模型 | GORM DB、`Store` 适配器 | admin settings、homepage video、user metrics | `api`、`cmd/accountsvc` |
| `internal/xrayconfig` | Xray 定义模板、客户端列表渲染、周期同步 | `ClientSource`、模板、命令配置 | config JSON、sync result | `cmd/accountsvc`、`agentmode`、`agentproto` |
| `internal/agentmode` | agent 运行时：拉取用户、生成配置、上报状态 | controller URL、agent token、xray sync config | HTTP client、status reporter、local sync loop | `agentproto`、`xrayconfig` |
| `internal/agentserver` | controller 侧 agent credential 与状态注册表 | credential config、status report、store | authenticated agent identity、status snapshots | `api`、`cmd/accountsvc` |
| `internal/agentproto` | controller 与 agent 之间的稳定 DTO | client list、status report field contract | JSON payload struct | `api`、`agentmode`、`agentserver` |

### 依赖方向

```text
cmd/accountsvc
  -> api
  -> internal/store
  -> internal/auth
  -> internal/service
  -> internal/agentserver
  -> internal/xrayconfig
  -> internal/agentmode

api
  -> internal/store
  -> internal/auth
  -> internal/service
  -> internal/agentserver
  -> internal/agentproto
  -> internal/xrayconfig

internal/agentmode
  -> internal/agentproto
  -> internal/xrayconfig
```

### 关键所有权说明

- `api.handler` 是 HTTP 层的状态拥有者，负责进程内 session / MFA / verification / reset / OAuth exchange 状态。
- `store.Store` 是业务事实层的唯一抽象，API、service、startup 逻辑都经由它读取用户、订阅、agent、计费、tenant 和 XWorkmate profile。
- `service` 不持有 HTTP 语义，只提供“管理面配置”和“聚合指标”。
- `agentserver.Registry` 是 controller 面的 runtime read model，不是持久化 source of truth；它会从 `store.Store` 回填，但仍以 store 为最终落点。
- `xrayconfig.PeriodicSyncer` 只做“配置收敛”，并不知道 HTTP、账户权限或 Stripe。

## English

### Package Responsibility Matrix

| Package / directory | Owning responsibility | Main inputs | Main outputs | Direct collaborators |
| --- | --- | --- | --- | --- |
| `cmd/accountsvc` | Process composition, mode switching, dependency injection, background loop startup | `config.Config`, environment, database handles | Gin server, agent loop, xray sync loop | `api`, `store`, `auth`, `service`, `agentserver`, `agentmode`, `xrayconfig` |
| `api` | Route registration, request validation, auth composition, response shaping | Gin requests, session tokens, injected services / stores | JSON responses, cookies, error payloads | `store`, `auth`, `service`, `agentserver`, `xrayconfig` |
| `internal/store` | Primary persistence abstraction and implementations | Domain models, SQL / in-memory state | `Store` interface, domain types, persistence behavior | `api`, `service`, `cmd/accountsvc` |
| `internal/auth` | Token service, OAuth providers, middleware context population | JWT secrets, OAuth config, session tokens | `TokenService`, `OAuthProvider`, Gin middleware | `api`, `cmd/accountsvc` |
| `internal/service` | Admin-side business logic and aggregated read models | GORM DB, `Store` adapters | admin settings, homepage video, user metrics | `api`, `cmd/accountsvc` |
| `internal/xrayconfig` | Xray definition templates, client rendering, periodic sync | `ClientSource`, templates, command config | config JSON, sync results | `cmd/accountsvc`, `agentmode`, `agentproto` |
| `internal/agentmode` | Agent runtime: fetch clients, generate configs, report status | controller URL, agent token, xray sync config | HTTP client, status reporter, local sync loop | `agentproto`, `xrayconfig` |
| `internal/agentserver` | Controller-side agent credential and status registry | credential config, status reports, store | authenticated agent identities, status snapshots | `api`, `cmd/accountsvc` |
| `internal/agentproto` | Stable DTO contract between controller and agents | client list fields, status report fields | JSON payload structs | `api`, `agentmode`, `agentserver` |

### Dependency Direction

```text
cmd/accountsvc
  -> api
  -> internal/store
  -> internal/auth
  -> internal/service
  -> internal/agentserver
  -> internal/xrayconfig
  -> internal/agentmode

api
  -> internal/store
  -> internal/auth
  -> internal/service
  -> internal/agentserver
  -> internal/agentproto
  -> internal/xrayconfig

internal/agentmode
  -> internal/agentproto
  -> internal/xrayconfig
```

### Key Ownership Notes

- `api.handler` is the HTTP-layer state owner for in-process session, MFA, verification, reset, and OAuth exchange state.
- `store.Store` is the single abstraction for business facts consumed by API, services, and startup code.
- `service` is HTTP-agnostic and focuses on admin configuration plus aggregated metrics.
- `agentserver.Registry` is a controller-side runtime read model, not the durable source of truth; it hydrates from and persists back to `store.Store`.
- `xrayconfig.PeriodicSyncer` owns configuration convergence only; it does not know about HTTP, account permissions, or Stripe.
