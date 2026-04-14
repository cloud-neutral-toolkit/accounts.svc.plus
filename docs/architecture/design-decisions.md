# Design Decisions / 关键设计取舍

## 中文

| 决策 | 当前实现 | 为什么这样做 | 代价 / 约束 |
| --- | --- | --- | --- |
| 会话优先于 JWT | `api` 主路径仍围绕 session token、cookie 和 `store.GetSession` 设计；JWT 只在 `auth.enable` 打开时作为增强能力出现 | 当前控制面和 BFF 依赖 session 语义，切换成本低 | 进程内会话与 DB session 并存，认证路径不是纯 JWT-only |
| 主业务持久化走 `store.Store` | API、service、startup 统一依赖 `internal/store.Store` | 隔离 memory / postgres 实现差异，并把领域事实收口到一个接口 | 接口面较大，需要维护 memory 与 postgres 两套实现 |
| GORM 只用于管理面模型 | admin settings、homepage video、sandbox binding、tenant / XWorkmate 模型走 GORM；核心账号和大多数事实表仍走 store | 管理面模型变更频率低，使用 GORM 更便于 schema 演进和事务编排 | 持久化技术栈分裂，开发者需要同时理解 store 与 GORM |
| Agent 认证采用预共享 token | `agentserver.Registry` 用 credential token 的 SHA-256 digest 做认证 | 适合私网、受控部署和低复杂度 agent 接入 | 没有短期轮换协议，token 管理依赖外部配置治理 |
| Xray 配置采用“数据库事实 -> 模板渲染 -> 原子写文件” | `xrayconfig.Generator` / `PeriodicSyncer` 每次从源读取 clients 后整体替换 `clients[]` | 简化收敛模型，避免局部 patch 导致配置漂移 | 生成端必须拥有文件和命令执行权限 |
| XWorkmate secret 不落业务 profile 原文 | profile 中只保存 locator 元数据，真实 secret 通过 Vault 接口读写 | 避免 API 和数据库持久化 raw token | 运行时依赖 Vault 可用性；`/profile/sync` 在 bridge token 缺失时会返回冲突 |
| Root 账号强约束为 `admin@svc.plus` | `store.RootAdminEmail`、RBAC schema 和 `ensureRootUser` 共同约束单 root 身份 | 明确唯一 root 身份，防止 legacy admin 扩散 | 迁移旧数据时会触发自动降级 / 归一化逻辑 |

### 设计结论

- 当前系统不是“完全统一”的单一持久化技术栈，而是有意识地将“主业务事实层”和“管理面配置层”分开。
- 认证模型也不是“完全统一”的 JWT-only 方案，而是 session-first、JWT-optional。
- Xray 与 agent 子系统采用生成式控制模型：controller 负责事实、registry 负责 runtime read model、agent 负责落地文件与心跳。

## English

| Decision | Current implementation | Why it exists | Trade-off / constraint |
| --- | --- | --- | --- |
| Session-first over JWT-only | Core API flows still revolve around session tokens, cookies, and `store.GetSession`; JWT is optional and only active when `auth.enable` is true | The current control plane and BFF flows already depend on session semantics | Authentication is not purely JWT-based and mixes in-memory plus DB-backed session behavior |
| Primary persistence through `store.Store` | API, services, and startup code all consume `internal/store.Store` | This isolates memory vs postgres differences and centralizes domain facts | The interface surface is large and both implementations must stay aligned |
| GORM is limited to admin-side models | Admin settings, homepage video, sandbox binding, and tenant / XWorkmate models use GORM; core identity and fact tables stay in the store layer | Admin-side models change less often and are easier to evolve with GORM | Developers must understand two persistence styles |
| Agent authentication uses pre-shared tokens | `agentserver.Registry` authenticates tokens by SHA-256 digest of configured credentials | This keeps agent onboarding simple for private or controlled deployments | There is no built-in short-lived token rotation protocol |
| Xray config follows a generated convergence model | `xrayconfig.Generator` / `PeriodicSyncer` always rebuild `clients[]` from the source of truth | Full regeneration is simpler and avoids drift from partial patching | The runtime needs file write access plus validate / restart command permissions |
| XWorkmate secrets are locator-only in profiles | Profiles keep locator metadata only; raw secret values are read and written through Vault | This prevents raw tokens from being persisted in API payloads or DB rows | Runtime behavior depends on Vault availability; `/profile/sync` conflicts when bridge credentials are missing |
| Root account is strictly `admin@svc.plus` | `store.RootAdminEmail`, RBAC schema, and `ensureRootUser` enforce a single canonical root identity | This prevents root-role sprawl and keeps privilege normalization explicit | Legacy admin rows may be demoted or normalized during startup |

### Design Summary

- The service intentionally separates the primary business fact layer from the admin-side configuration layer instead of forcing one persistence style everywhere.
- Authentication is intentionally session-first with optional JWT support rather than JWT-only.
- The Xray / agent subsystem follows a generative control model: controller owns facts, registry owns runtime read state, and agents own config-file application plus heartbeats.
