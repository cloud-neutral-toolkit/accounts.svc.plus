# Code Structure / 代码结构

## 中文

### 阅读顺序

建议按下面顺序阅读当前仓库主路径：

1. `cmd/accountsvc/main.go`
2. `api/api.go`
3. `internal/store/store.go`
4. `internal/auth/*`
5. `internal/service/*`
6. `internal/xrayconfig/*`
7. `internal/agentserver/registry.go`
8. `internal/agentmode/*`
9. `internal/agentproto/types.go`

### `cmd/accountsvc`（运行时装配）

**核心职责**

- 读取配置并决定运行模式：`server`、`agent`、`server-agent`。
- 装配主 store、GORM admin DB、mailer、token service、OAuth provider、agent registry、xray syncer。
- 确保 root / review / sandbox 账户与 RBAC schema 满足当前契约。

**关键非导出 owner**

| 符号 | 说明 | 主要输入 | 主要输出 |
| --- | --- | --- | --- |
| `rootCmd.RunE` | CLI 主入口，负责 mode dispatch | `configPath`、`logLevel` | 调用 `runServer` / `runAgent` / `runServerAndAgent` |
| `runServer(ctx, cfg, logger)` | server 模式总装配函数 | `context.Context`、`config.Config`、`*slog.Logger` | Gin server、后台循环、依赖注入 |
| `runAgent(ctx, cfg, logger)` | agent 模式装配函数 | 同上 | `agentmode.Run(...)` |
| `openAdminSettingsDB(cfg.Store)` | 打开并迁移 GORM DB | `config.Store` | `*gorm.DB`、cleanup 闭包 |
| `applyRBACSchema(ctx, db, driver)` | 写入 RBAC / users / agents / sessions 等 schema 与 seed | `context.Context`、`*gorm.DB`、driver 名 | error |

### `api`（HTTP 控制层）

**文件**

- `api/api.go`：主路由注册、auth/session/MFA/subscription/OAuth。
- `api/admin_*.go`：admin metrics、agent status、sandbox assume/bind、用户管理。
- `api/accounting.go`：account / traffic / policy / heartbeat 读写面。
- `api/xworkmate*.go`：tenant/profile/secret/Vault 集成。
- `api/stripe.go`：checkout / portal / webhook。
- `api/homepage_video.go`、`api/config_sync.go`、`api/internal_*.go`：辅助控制面接口。

**核心导出符号**

| 符号 | kind | 签名 / 字段 | 参数 | 返回 | 说明 |
| --- | --- | --- | --- | --- | --- |
| `Option` | func type | `type Option func(*handler)` | `*handler` | 无 | API 层依赖注入闭包。 |
| `WithStore` | func | `WithStore(st store.Store) Option` | `st`: 主业务 store | `Option` | 覆盖默认内存 store。 |
| `WithSessionTTL` | func | `WithSessionTTL(ttl time.Duration) Option` | `ttl`: session 生命周期 | `Option` | 控制会话签发 TTL。 |
| `WithEmailSender` | func | `WithEmailSender(sender EmailSender) Option` | `sender`: 邮件发送器 | `Option` | 注入真实或 mock mailer。 |
| `WithEmailVerification` | func | `WithEmailVerification(enabled bool) Option` | `enabled`: 是否要求邮件验证 | `Option` | 控制注册前邮箱验证逻辑。 |
| `WithEmailVerificationTTL` | func | `WithEmailVerificationTTL(ttl time.Duration) Option` | `ttl`: 验证码 TTL | `Option` | 覆盖默认邮箱验证码 TTL。 |
| `WithUserMetricsProvider` | func | `WithUserMetricsProvider(provider service.UserMetricsProvider) Option` | metrics provider | `Option` | 注入用户指标聚合器。 |
| `WithAgentStatusReader` | func | `WithAgentStatusReader(reader agentStatusReader) Option` | registry / status reader | `Option` | 注入 admin agent status 读面。 |
| `WithPasswordResetTTL` | func | `WithPasswordResetTTL(ttl time.Duration) Option` | reset TTL | `Option` | 控制密码重置 token 生命周期。 |
| `WithTokenService` | func | `WithTokenService(tokenService *auth.TokenService) Option` | token service | `Option` | 启用可选 JWT + session middleware。 |
| `WithOAuthProviders` | func | `WithOAuthProviders(providers map[string]auth.OAuthProvider) Option` | provider map | `Option` | 注册 GitHub / Google OAuth provider。 |
| `WithServerPublicURL` | func | `WithServerPublicURL(url string) Option` | public URL | `Option` | 供 sync config / VLESS URL 生成使用。 |
| `WithXrayConfigRenderer` | func | `WithXrayConfigRenderer(renderer func(*store.User) (string, string, []string, error)) Option` | 自定义 renderer | `Option` | 主要用于测试或替换 sync render。 |
| `WithOAuthFrontendURL` | func | `WithOAuthFrontendURL(url string) Option` | frontend URL | `Option` | 决定 OAuth callback 的前端跳转目标。 |
| `WithXWorkmateVaultService` | func | `WithXWorkmateVaultService(vaultService xworkmateVaultService) Option` | Vault backend | `Option` | 启用 XWorkmate secret 读写。 |
| `WithAgentRegistry` | func | `WithAgentRegistry(registry agentRegistry) Option` | agent registry | `Option` | 注入 agent auth 与 sandbox agent 读面。 |
| `WithGormDB` | func | `WithGormDB(db *gorm.DB) Option` | GORM DB | `Option` | admin / tenant / homepage video 使用。 |
| `WithStripeConfig` | func | `WithStripeConfig(cfg StripeConfig) Option` | Stripe 配置 | `Option` | 启用 Stripe 集成。 |
| `RegisterRoutes` | func | `RegisterRoutes(r *gin.Engine, opts ...Option)` | Gin engine、Option 列表 | 无 | 一次性挂载全部 HTTP 路由。 |
| `EmailMessage` | struct | `To []string`, `Subject string`, `PlainBody string`, `HTMLBody string` | N/A | N/A | API 层统一邮件消息体。 |
| `EmailSender` | interface | `Send(ctx context.Context, msg EmailMessage) error` | context、message | `error` | 邮件发送接口。 |
| `EmailSenderFunc` | func type | `func(ctx context.Context, msg EmailMessage) error` | 同上 | `error` | 函数适配器。 |
| `StripeConfig` | struct | `SecretKey`, `WebhookSecret`, `AllowedPriceIDs`, `FrontendURL` | N/A | N/A | Stripe 客户端构造参数。 |
| `XWorkmateVaultConfig` | struct | `Address`, `Token`, `Namespace`, `Mount`, `Timeout`, `HTTPClient` | N/A | N/A | XWorkmate Vault HTTP 客户端配置。 |
| `NewXWorkmateVaultService` | func | `NewXWorkmateVaultService(cfg XWorkmateVaultConfig) (xworkmateVaultService, error)` | Vault config | service、`error` | 根据配置决定返回 HTTP backend 或 `nil`。 |

**关键非导出 owner**

| 符号 | 说明 |
| --- | --- |
| `handler` | HTTP 层真正的状态拥有者，持有 store、session TTL、MFA challenge map、verification map、password reset map、OAuth exchange code map、metrics provider、token service、Vault service、Stripe client、agent registry 等。 |
| `respondError` | 标准错误信封输出函数，形如 `{"error":"code","message":"message"}`。 |
| `sanitizeUser` / `sanitizeSubscription` | 主 API 的稳定返回 shape 组装器。 |
| `buildXWorkmateProfileResponse` | XWorkmate profile / secret API 的统一响应组装器。 |

### `internal/store`（领域模型与持久化抽象）

**核心导出类型**

| 符号 | kind | 关键字段 / 方法 | 说明 |
| --- | --- | --- | --- |
| `User` | struct | `ID`, `Name`, `Email`, `Level`, `Role`, `Groups`, `Permissions`, `EmailVerified`, `PasswordHash`, `MFATOTPSecret`, `MFAEnabled`, `ProxyUUID`, `ProxyUUIDExpiresAt` | 主账号实体。 |
| `Subscription` | struct | `ID`, `UserID`, `Provider`, `PaymentMethod`, `Kind`, `PlanID`, `ExternalID`, `Status`, `Meta` | 订阅或账单关系。 |
| `Identity` | struct | `ID`, `UserID`, `Provider`, `ExternalID` | OAuth 身份映射。 |
| `Agent` | struct | `ID`, `Name`, `Groups`, `Healthy`, `LastHeartbeat`, `ClientsCount`, `SyncRevision` | controller 持久化的 agent 状态行。 |
| `TrafficStatCheckpoint` / `TrafficMinuteBucket` / `BillingLedgerEntry` / `AccountQuotaState` / `AccountBillingProfile` / `AccountPolicySnapshot` / `NodeHealthSnapshot` / `SchedulerDecision` | struct | 计费、流量、调度读模型 | 为 `/api/account/*` 与 `/api/admin/traffic/*` 提供事实层。 |
| `Tenant` / `TenantDomain` / `TenantMembership` / `XWorkmateProfile` / `XWorkmateSecretLocator` | struct | tenant / 域名 / 成员关系 / integration profile / secret locator 元数据 | 支撑 XWorkmate 多租户与 Vault locator 设计。 |
| `Store` | interface | 见下表 | 主业务持久化抽象。 |

**`Store` 接口方法分组**

- 用户与身份：
  - `CreateUser(ctx context.Context, user *User) error`
  - `GetUserByEmail(ctx context.Context, email string) (*User, error)`
  - `GetUserByID(ctx context.Context, id string) (*User, error)`
  - `GetUserByName(ctx context.Context, name string) (*User, error)`
  - `UpdateUser(ctx context.Context, user *User) error`
  - `CreateIdentity(ctx context.Context, identity *Identity) error`
  - `ListUsers(ctx context.Context) ([]User, error)`
  - `DeleteUser(ctx context.Context, id string) error`

- 订阅：
  - `UpsertSubscription(ctx context.Context, subscription *Subscription) error`
  - `ListSubscriptionsByUser(ctx context.Context, userID string) ([]Subscription, error)`
  - `CancelSubscription(ctx context.Context, userID, externalID string, cancelledAt time.Time) (*Subscription, error)`

- 黑名单：
  - `AddToBlacklist(ctx context.Context, email string) error`
  - `RemoveFromBlacklist(ctx context.Context, email string) error`
  - `IsBlacklisted(ctx context.Context, email string) (bool, error)`
  - `ListBlacklist(ctx context.Context) ([]string, error)`

- 会话：
  - `CreateSession(ctx context.Context, token, userID string, expiresAt time.Time) error`
  - `GetSession(ctx context.Context, token string) (string, time.Time, error)`
  - `DeleteSession(ctx context.Context, token string) error`

- Agent：
  - `UpsertAgent(ctx context.Context, agent *Agent) error`
  - `GetAgent(ctx context.Context, id string) (*Agent, error)`
  - `ListAgents(ctx context.Context) ([]*Agent, error)`
  - `DeleteAgent(ctx context.Context, id string) error`
  - `DeleteStaleAgents(ctx context.Context, staleThreshold time.Duration) (int, error)`

- 流量 / 计费 / 调度：
  - `UpsertTrafficStatCheckpoint(ctx context.Context, checkpoint *TrafficStatCheckpoint) error`
  - `GetTrafficStatCheckpoint(ctx context.Context, nodeID, accountUUID string) (*TrafficStatCheckpoint, error)`
  - `ListTrafficStatCheckpoints(ctx context.Context) ([]TrafficStatCheckpoint, error)`
  - `UpsertTrafficMinuteBucket(ctx context.Context, bucket *TrafficMinuteBucket) error`
  - `ListTrafficMinuteBucketsByAccount(ctx context.Context, accountUUID string, start, end time.Time) ([]TrafficMinuteBucket, error)`
  - `ListTrafficMinuteBuckets(ctx context.Context) ([]TrafficMinuteBucket, error)`
  - `InsertBillingLedgerEntry(ctx context.Context, entry *BillingLedgerEntry) error`
  - `ListBillingLedgerByAccount(ctx context.Context, accountUUID string, limit int) ([]BillingLedgerEntry, error)`
  - `UpsertAccountQuotaState(ctx context.Context, state *AccountQuotaState) error`
  - `GetAccountQuotaState(ctx context.Context, accountUUID string) (*AccountQuotaState, error)`
  - `UpsertAccountBillingProfile(ctx context.Context, profile *AccountBillingProfile) error`
  - `GetAccountBillingProfile(ctx context.Context, accountUUID string) (*AccountBillingProfile, error)`
  - `UpsertAccountPolicySnapshot(ctx context.Context, snapshot *AccountPolicySnapshot) error`
  - `GetLatestAccountPolicySnapshot(ctx context.Context, accountUUID string) (*AccountPolicySnapshot, error)`
  - `UpsertNodeHealthSnapshot(ctx context.Context, snapshot *NodeHealthSnapshot) error`
  - `ListLatestNodeHealthSnapshots(ctx context.Context) ([]NodeHealthSnapshot, error)`
  - `InsertSchedulerDecision(ctx context.Context, decision *SchedulerDecision) error`
  - `ListRecentSchedulerDecisions(ctx context.Context, limit int) ([]SchedulerDecision, error)`

- Tenant / XWorkmate：
  - `EnsureTenant(ctx context.Context, tenant *Tenant) error`
  - `EnsureTenantDomain(ctx context.Context, domain *TenantDomain) error`
  - `UpsertTenantMembership(ctx context.Context, membership *TenantMembership) error`
  - `ResolveTenantByHost(ctx context.Context, host string) (*Tenant, *TenantDomain, error)`
  - `ListTenantMembershipsByUser(ctx context.Context, userID string) ([]TenantMembership, error)`
  - `GetTenantMembership(ctx context.Context, tenantID, userID string) (*TenantMembership, error)`
  - `GetXWorkmateProfile(ctx context.Context, tenantID, userID, scope string) (*XWorkmateProfile, error)`
  - `UpsertXWorkmateProfile(ctx context.Context, profile *XWorkmateProfile) error`

**规范化与角色辅助函数**

- `NormalizeTenantEdition(value string) string`
- `NormalizeTenantMembershipRole(value string) string`
- `NormalizeTenantDomainKind(value string) string`
- `NormalizeTenantDomainStatus(value string) string`
- `NormalizeXWorkmateProfileScope(value string) string`
- `NormalizeXWorkmateSecretLocator(locator *XWorkmateSecretLocator)`
- `NormalizeHostname(value string) string`
- `IsSharedTenantHost(host string) bool`
- `GenerateRandomTenantDomain() (string, error)`
- `IsRootRole(role string) bool`
- `IsAdminRole(role string) bool`
- `IsOperatorRole(role string) bool`
- `NewMemoryStore() Store`
- `NewMemoryStoreWithSuperAdminCounting() Store`

### `internal/auth`（认证、OAuth、中间件）

**核心导出符号**

| 符号 | 签名 / 字段 | 参数 | 返回 | 说明 |
| --- | --- | --- | --- | --- |
| `TokenPair` | `PublicToken`, `AccessToken`, `RefreshToken`, `TokenType`, `ExpiresIn` | N/A | N/A | JWT 相关返回体。 |
| `Claims` | `UserID`, `Email`, `Roles`, `MFA`, `jwt.RegisteredClaims` | N/A | N/A | access token claims。 |
| `TokenConfig` | `PublicToken`, `RefreshSecret`, `AccessSecret`, `AccessExpiry`, `RefreshExpiry`, `Store` | N/A | N/A | token service 配置。 |
| `TokenService` | token / secret / expiry / store 成员 | N/A | N/A | JWT 与 session fallback 中心。 |
| `NewTokenService` | `NewTokenService(config TokenConfig) *TokenService` | config | `*TokenService` | 构造 token service。 |
| `SetStore` | `SetStore(st store.Store)` | store | 无 | 为 AuthMiddleware 注入 session store。 |
| `ValidatePublicToken` | `ValidatePublicToken(publicToken string) bool` | public token | `bool` | 校验 public token。 |
| `GeneratePublicToken` | `GeneratePublicToken(userID, email string, roles []string) string` | user identity | `string` | 返回配置中的 public token。 |
| `GenerateTokenPair` | `GenerateTokenPair(userID, email string, roles []string) (*TokenPair, error)` | user identity | token pair、error | 生成 access + refresh token。 |
| `ValidateAccessToken` | `ValidateAccessToken(accessToken string) (*Claims, error)` | JWT string | claims、error | 解析 access token。 |
| `RefreshAccessToken` | `RefreshAccessToken(refreshToken string) (string, error)` | refresh token | new access token、error | 刷新 access token。 |
| `GetAccessTokenExpiry` | `GetAccessTokenExpiry() time.Duration` | 无 | duration | 返回 access token TTL。 |
| `OAuthUserProfile` | `ID`, `Email`, `Name`, `Verified` | N/A | N/A | 统一 OAuth 用户 profile。 |
| `OAuthProvider` | `AuthCodeURL(state string) string`, `Exchange(ctx, code) (*oauth2.Token, error)`, `FetchProfile(ctx, token) (*OAuthUserProfile, error)`, `Name() string` | N/A | N/A | OAuth provider 抽象。 |
| `GitHubProvider` / `GoogleProvider` | provider struct | N/A | N/A | 具体 OAuth provider。 |
| `NewGitHubProvider` | `NewGitHubProvider(clientID, clientSecret, redirectURL string) *GitHubProvider` | OAuth config | provider | GitHub OAuth 构造。 |
| `NewGoogleProvider` | `NewGoogleProvider(clientID, clientSecret, redirectURL string) *GoogleProvider` | OAuth config | provider | Google OAuth 构造。 |
| `AuthMiddleware` | `func (s *TokenService) AuthMiddleware() gin.HandlerFunc` | token service receiver | Gin middleware | JWT 失败后 fallback 到 session store。 |
| `RequireActiveUser` | `RequireActiveUser(s store.Store) gin.HandlerFunc` | store | middleware | 拒绝 suspended user。 |
| `InternalAuthMiddleware` | `InternalAuthMiddleware() gin.HandlerFunc` | 无 | middleware | 校验 `X-Service-Token`。 |
| `RequireMFA` | `RequireMFA() gin.HandlerFunc` | 无 | middleware | 要求 MFA verified。 |
| `RequireRole` | `RequireRole(role string) gin.HandlerFunc` | role | middleware | 角色检查。 |
| `GetUserID` / `GetEmail` / `GetRoles` / `IsMFAVerified` | context getter | `*gin.Context` | 基础值 | 从 Gin context 中读取 auth state。 |
| `HTTPHandler` / `Wrap` | 轻量 handler adapter | handler | `gin.HandlerFunc` | 包内辅助适配器。 |

### `internal/service`（管理面服务）

**admin settings**

- `type AdminSettings struct`
  - 字段：`Version uint64`、`Matrix map[string]map[string]bool`
- `SetDB(d *gorm.DB)`
  - 参数：GORM DB
  - 返回：无
  - 作用：设置 service 层共享 DB 并清空缓存
- `GetAdminSettings(ctx context.Context) (AdminSettings, error)`
  - 返回：当前 permission matrix 与 version
- `SaveAdminSettings(ctx context.Context, payload AdminSettings) (AdminSettings, error)`
  - 返回：新版本 settings 或版本冲突错误

**homepage video**

- `type HomepageVideoEntry struct`
  - 字段：`DomainKey`, `VideoURL`, `PosterURL`
- `type HomepageVideoSettings struct`
  - 字段：`DefaultEntry HomepageVideoEntry`, `Overrides []HomepageVideoEntry`
- `GetHomepageVideoSettings(ctx context.Context) (HomepageVideoSettings, error)`
- `SaveHomepageVideoSettings(ctx context.Context, settings HomepageVideoSettings, updatedBy string) (HomepageVideoSettings, error)`
- `ResolveHomepageVideoEntry(ctx context.Context, host string) (HomepageVideoEntry, error)`

**user metrics**

- `type UserRepository interface`
  - `ListUsers(ctx context.Context) ([]UserRecord, error)`
- `type SubscriptionProvider interface`
  - `FetchSubscriptionStates(ctx context.Context, userIDs []string) (map[string]SubscriptionState, error)`
- `type UserRecord struct`
  - `ID`, `CreatedAt`, `Active`
- `type SubscriptionState struct`
  - `Active`, `ExpiresAt`
- `type UserMetricsService struct`
  - 依赖：`Users UserRepository`, `Subscriptions SubscriptionProvider`, `DailyPeriods`, `WeeklyPeriods`
- `Compute(ctx context.Context) (UserMetrics, error)`
  - 返回：`MetricsOverview` + `MetricsSeries`

### `internal/xrayconfig`（配置生成与同步）

| 符号 | 签名 / 字段 | 说明 |
| --- | --- | --- |
| `Definition` | `Base() (map[string]interface{}, error)` | Xray 基础模板抽象。 |
| `JSONDefinition` | `Raw []byte` | 以 JSON 文档实现 `Definition`。 |
| `Client` | `ID`, `Email`, `Flow` | Xray client 条目。 |
| `Generator` | `Definition`, `OutputPath`, `FileMode`, `Domain` | 根据 clients 生成配置文件。 |
| `Generate` | `Generate(clients []Client) error` | 渲染并原子写入文件。 |
| `Render` | `Render(clients []Client) ([]byte, error)` | 只返回 JSON，不写盘。 |
| `GormClientSource` | `DB *gorm.DB`, `Logger *slog.Logger` | 从 users 表读取 client。 |
| `NewGormClientSource` | `NewGormClientSource(db *gorm.DB) (*GormClientSource, error)` | 构造 GORM source。 |
| `ClientSource` | `ListClients(ctx context.Context) ([]Client, error)` | `PeriodicSyncer` 依赖的源接口。 |
| `PeriodicOptions` | `Logger`, `Interval`, `Source`, `Generators`, `ValidateCommand`, `RestartCommand`, `Runner`, `OnSync` | syncer 配置。 |
| `PeriodicSyncer` | 周期构造体 | 周期性收敛配置文件。 |
| `SyncResult` | `Clients`, `Error`, `CompletedAt` | 单次同步结果。 |
| `NewPeriodicSyncer` | `NewPeriodicSyncer(opts PeriodicOptions) (*PeriodicSyncer, error)` | 构造 syncer。 |
| `Start` | `Start(ctx context.Context) (func(context.Context) error, error)` | 启动后台循环并返回 stop 闭包。 |
| `DefaultDefinition` / `TCPDefinition` / `XHTTPDefinition` | 模板函数 | 返回预置 Xray 模板。 |
| `VLESSTCPScheme` / `VLESSXHTTPScheme` | URI 模板函数 | 供 sync config / node URL 使用。 |

### `internal/agentmode`（agent 运行时）

| 符号 | 签名 / 字段 | 参数 | 返回 | 说明 |
| --- | --- | --- | --- | --- |
| `ClientOptions` | `Timeout`, `InsecureSkipVerify`, `UserAgent`, `AgentID` | N/A | N/A | controller HTTP client 配置。 |
| `Client` | `baseURL`, `token`, `http`, `userAgent`, `agentID` | N/A | N/A | 认证 HTTP client。 |
| `NewClient` | `NewClient(baseURL, token string, opts ClientOptions) (*Client, error)` | controller URL、token、client options | client、error | 构造 controller 客户端。 |
| `ListClients` | `ListClients(ctx context.Context) (agentproto.ClientListResponse, error)` | context | response、error | 调用 `/api/agent-server/v1/users`。 |
| `ReportStatus` | `ReportStatus(ctx context.Context, report agentproto.StatusReport) error` | context、status report | error | 调用 `/api/agent-server/v1/status`。 |
| `Options` | `Logger`, `Agent config.Agent`, `Xray config.Xray` | N/A | N/A | agent mode 总配置。 |
| `Run` | `Run(ctx context.Context, opts Options) error` | context、options | error | 启动 agent 全链路：拉取 clients、生成 config、周期上报状态。 |
| `HTTPClientSource` | `client`, `tracker` | N/A | N/A | 把 controller API 适配成 `xrayconfig.ClientSource`。 |
| `NewHTTPClientSource` | `NewHTTPClientSource(client *Client, tracker *syncTracker) *HTTPClientSource` | client、tracker | source | 构造 HTTP source。 |
| `ListClients`（source） | `ListClients(ctx context.Context) ([]xrayconfig.Client, error)` | context | clients、error | 把 controller payload 转成 syncer 输入。 |

### `internal/agentserver`（controller 侧 agent registry）

| 符号 | 签名 / 字段 | 说明 |
| --- | --- | --- |
| `Credential` | `ID`, `Name`, `Token`, `Groups` | 配置层 agent 凭据。 |
| `Config` | `Credentials []Credential` | registry 构造参数。 |
| `Identity` | `ID`, `Name`, `Groups` | 认证成功后的 agent 身份。 |
| `StatusSnapshot` | `Agent Identity`, `Report agentproto.StatusReport`, `UpdatedAt time.Time` | agent 最近一次状态快照。 |
| `Registry` | credential digest、byID、statuses、sandboxAgents、store、logger | controller 内存注册表。 |
| `NewRegistry` | `NewRegistry(cfg Config) (*Registry, error)` | 构造并校验 credential 集。 |
| `SetStore` / `SetLogger` | 注入 store 与 logger | 持久化与日志配置。 |
| `Authenticate` | `Authenticate(token string) (*Identity, bool)` | 通过 token digest 查找 agent identity。 |
| `ReportStatus` | `ReportStatus(agent Identity, report agentproto.StatusReport)` | 更新内存快照并异步持久化到 store。 |
| `RegisterAgent` | `RegisterAgent(agentID string, groups []string) Identity` | 动态注册共享 token 下的 agent。 |
| `Load` | `Load(ctx context.Context) error` | 从 store 回填 agents / statuses。 |
| `Statuses` / `Agents` | 只读导出方法 | 返回当前 registry read model。 |
| `IsSandboxAgent` / `SetSandboxAgent` / `ClearSandboxAgents` | sandbox 标记管理 | 供 sandbox node binding 使用。 |

### `internal/agentproto`（controller-agent DTO）

- `type ClientListResponse struct`
  - 字段：`Clients []xrayconfig.Client`, `Total int`, `GeneratedAt time.Time`, `Revision string`
- `type StatusReport struct`
  - 字段：`AgentID`, `Healthy`, `Message`, `Users`, `SyncRevision`, `Xray XrayStatus`
- `type XrayStatus struct`
  - 字段：`Running`, `Clients`, `LastSync`, `ConfigHash`, `NodeID`, `Region`, `LineCode`, `PricingGroup`, `StatsEnabled`, `XrayRevision`

## English

### Recommended Reading Order

Read the current core path in this order:

1. `cmd/accountsvc/main.go`
2. `api/api.go`
3. `internal/store/store.go`
4. `internal/auth/*`
5. `internal/service/*`
6. `internal/xrayconfig/*`
7. `internal/agentserver/registry.go`
8. `internal/agentmode/*`
9. `internal/agentproto/types.go`

### `cmd/accountsvc` (runtime composition)

**Core responsibility**

- Loads config and chooses runtime mode: `server`, `agent`, or `server-agent`.
- Composes the primary store, GORM admin DB, mailer, token service, OAuth providers, agent registry, and xray syncer.
- Normalizes root / review / sandbox accounts plus RBAC schema.

**Key non-export owners**

| Symbol | Purpose | Main inputs | Main outputs |
| --- | --- | --- | --- |
| `rootCmd.RunE` | CLI entry point and mode dispatch | `configPath`, `logLevel` | Calls `runServer`, `runAgent`, or `runServerAndAgent` |
| `runServer(ctx, cfg, logger)` | Main server composition path | `context.Context`, `config.Config`, `*slog.Logger` | Gin server, background loops, injected dependencies |
| `runAgent(ctx, cfg, logger)` | Agent-mode composition path | same | `agentmode.Run(...)` |
| `openAdminSettingsDB(cfg.Store)` | Opens and migrates the GORM-backed admin DB | `config.Store` | `*gorm.DB`, cleanup function |
| `applyRBACSchema(ctx, db, driver)` | Creates and seeds RBAC / users / agents / sessions schema | `context.Context`, `*gorm.DB`, driver name | `error` |

### `api` (HTTP control layer)

**Files**

- `api/api.go`: main route registration plus auth / session / MFA / subscription / OAuth.
- `api/admin_*.go`: admin metrics, agent status, sandbox assume / bind, user operations.
- `api/accounting.go`: account usage / billing / policy / heartbeat reads.
- `api/xworkmate*.go`: tenant, profile, secret, and Vault-backed integration paths.
- `api/stripe.go`: checkout, portal, webhook.
- `api/homepage_video.go`, `api/config_sync.go`, `api/internal_*.go`: auxiliary control-plane endpoints.

**Core exported symbols**

- `type Option func(*handler)`
- `WithStore(st store.Store) Option`
- `WithSessionTTL(ttl time.Duration) Option`
- `WithEmailSender(sender EmailSender) Option`
- `WithEmailVerification(enabled bool) Option`
- `WithEmailVerificationTTL(ttl time.Duration) Option`
- `WithUserMetricsProvider(provider service.UserMetricsProvider) Option`
- `WithAgentStatusReader(reader agentStatusReader) Option`
- `WithPasswordResetTTL(ttl time.Duration) Option`
- `WithTokenService(tokenService *auth.TokenService) Option`
- `WithOAuthProviders(providers map[string]auth.OAuthProvider) Option`
- `WithServerPublicURL(url string) Option`
- `WithXrayConfigRenderer(renderer func(*store.User) (string, string, []string, error)) Option`
- `WithOAuthFrontendURL(url string) Option`
- `WithXWorkmateVaultService(vaultService xworkmateVaultService) Option`
- `WithAgentRegistry(registry agentRegistry) Option`
- `WithGormDB(db *gorm.DB) Option`
- `WithStripeConfig(cfg StripeConfig) Option`
- `RegisterRoutes(r *gin.Engine, opts ...Option)`
- `type EmailMessage struct`
- `type EmailSender interface`
- `type EmailSenderFunc`
- `type StripeConfig struct`
- `type XWorkmateVaultConfig struct`
- `NewXWorkmateVaultService(cfg XWorkmateVaultConfig) (xworkmateVaultService, error)`

**Key non-export owners**

- `handler`: the HTTP-layer state owner for store, session state, MFA challenges, email verification, password reset, OAuth exchange codes, metrics provider, token service, Vault service, Stripe client, and agent registry.
- `respondError`: emits the standard `{"error":"code","message":"message"}` envelope.
- `sanitizeUser` / `sanitizeSubscription`: stable user and subscription response shapers.
- `buildXWorkmateProfileResponse`: shared response builder for XWorkmate profile / secret APIs.

### `internal/store` (domain model and persistence abstraction)

**Key exported types**

- `User`
- `Subscription`
- `Identity`
- `Agent`
- `TrafficStatCheckpoint`
- `TrafficMinuteBucket`
- `BillingLedgerEntry`
- `AccountQuotaState`
- `AccountBillingProfile`
- `AccountPolicySnapshot`
- `NodeHealthSnapshot`
- `SchedulerDecision`
- `Tenant`
- `TenantDomain`
- `TenantMembership`
- `XWorkmateProfile`
- `XWorkmateSecretLocator`
- `Store`

**`Store` method groups**

- User / identity: `CreateUser`, `GetUserByEmail`, `GetUserByID`, `GetUserByName`, `UpdateUser`, `CreateIdentity`, `ListUsers`, `DeleteUser`
- Subscription: `UpsertSubscription`, `ListSubscriptionsByUser`, `CancelSubscription`
- Blacklist: `AddToBlacklist`, `RemoveFromBlacklist`, `IsBlacklisted`, `ListBlacklist`
- Session: `CreateSession`, `GetSession`, `DeleteSession`
- Agent: `UpsertAgent`, `GetAgent`, `ListAgents`, `DeleteAgent`, `DeleteStaleAgents`
- Traffic / billing / scheduler: `UpsertTrafficStatCheckpoint`, `GetTrafficStatCheckpoint`, `ListTrafficStatCheckpoints`, `UpsertTrafficMinuteBucket`, `ListTrafficMinuteBucketsByAccount`, `ListTrafficMinuteBuckets`, `InsertBillingLedgerEntry`, `ListBillingLedgerByAccount`, `UpsertAccountQuotaState`, `GetAccountQuotaState`, `UpsertAccountBillingProfile`, `GetAccountBillingProfile`, `UpsertAccountPolicySnapshot`, `GetLatestAccountPolicySnapshot`, `UpsertNodeHealthSnapshot`, `ListLatestNodeHealthSnapshots`, `InsertSchedulerDecision`, `ListRecentSchedulerDecisions`
- Tenant / XWorkmate: `EnsureTenant`, `EnsureTenantDomain`, `UpsertTenantMembership`, `ResolveTenantByHost`, `ListTenantMembershipsByUser`, `GetTenantMembership`, `GetXWorkmateProfile`, `UpsertXWorkmateProfile`

**Normalization and role helpers**

- `NormalizeTenantEdition`
- `NormalizeTenantMembershipRole`
- `NormalizeTenantDomainKind`
- `NormalizeTenantDomainStatus`
- `NormalizeXWorkmateProfileScope`
- `NormalizeXWorkmateSecretLocator`
- `NormalizeHostname`
- `IsSharedTenantHost`
- `GenerateRandomTenantDomain`
- `IsRootRole`
- `IsAdminRole`
- `IsOperatorRole`
- `NewMemoryStore`
- `NewMemoryStoreWithSuperAdminCounting`

### `internal/auth` (authentication, OAuth, middleware)

Core exported symbols:

- `TokenPair`
- `Claims`
- `TokenConfig`
- `TokenService`
- `NewTokenService(config TokenConfig) *TokenService`
- `SetStore(st store.Store)`
- `ValidatePublicToken(publicToken string) bool`
- `GeneratePublicToken(userID, email string, roles []string) string`
- `GenerateTokenPair(userID, email string, roles []string) (*TokenPair, error)`
- `ValidateAccessToken(accessToken string) (*Claims, error)`
- `RefreshAccessToken(refreshToken string) (string, error)`
- `GetAccessTokenExpiry() time.Duration`
- `OAuthUserProfile`
- `OAuthProvider`
- `GitHubProvider`
- `GoogleProvider`
- `NewGitHubProvider(clientID, clientSecret, redirectURL string) *GitHubProvider`
- `NewGoogleProvider(clientID, clientSecret, redirectURL string) *GoogleProvider`
- `AuthMiddleware() gin.HandlerFunc`
- `RequireActiveUser(s store.Store) gin.HandlerFunc`
- `InternalAuthMiddleware() gin.HandlerFunc`
- `RequireMFA() gin.HandlerFunc`
- `RequireRole(role string) gin.HandlerFunc`
- `GetUserID`, `GetEmail`, `GetRoles`, `IsMFAVerified`
- `HTTPHandler`
- `Wrap(handler HTTPHandler) gin.HandlerFunc`

### `internal/service` (admin-side services)

Admin settings:

- `AdminSettings`
- `SetDB`
- `GetAdminSettings`
- `SaveAdminSettings`

Homepage video:

- `HomepageVideoEntry`
- `HomepageVideoSettings`
- `GetHomepageVideoSettings`
- `SaveHomepageVideoSettings`
- `ResolveHomepageVideoEntry`

User metrics:

- `UserRepository`
- `SubscriptionProvider`
- `UserRecord`
- `SubscriptionState`
- `UserMetricsService`
- `UserMetricsProvider`
- `UserMetrics`
- `MetricsOverview`
- `MetricsSeries`
- `MetricsPoint`
- `Compute(ctx context.Context) (UserMetrics, error)`

### `internal/xrayconfig` (config generation and sync)

- `Definition`
- `JSONDefinition`
- `Client`
- `Generator`
- `Generate(clients []Client) error`
- `Render(clients []Client) ([]byte, error)`
- `GormClientSource`
- `NewGormClientSource(db *gorm.DB) (*GormClientSource, error)`
- `ClientSource`
- `PeriodicOptions`
- `PeriodicSyncer`
- `SyncResult`
- `NewPeriodicSyncer(opts PeriodicOptions) (*PeriodicSyncer, error)`
- `Start(ctx context.Context) (func(context.Context) error, error)`
- `DefaultDefinition`, `TCPDefinition`, `XHTTPDefinition`
- `VLESSTCPScheme`, `VLESSXHTTPScheme`

### `internal/agentmode` (agent runtime)

- `ClientOptions`
- `Client`
- `NewClient(baseURL, token string, opts ClientOptions) (*Client, error)`
- `ListClients(ctx context.Context) (agentproto.ClientListResponse, error)`
- `ReportStatus(ctx context.Context, report agentproto.StatusReport) error`
- `Options`
- `Run(ctx context.Context, opts Options) error`
- `HTTPClientSource`
- `NewHTTPClientSource(client *Client, tracker *syncTracker) *HTTPClientSource`
- `ListClients(ctx context.Context) ([]xrayconfig.Client, error)` on `HTTPClientSource`

### `internal/agentserver` (controller-side registry)

- `Credential`
- `Config`
- `Identity`
- `StatusSnapshot`
- `Registry`
- `NewRegistry(cfg Config) (*Registry, error)`
- `SetStore`
- `SetLogger`
- `Authenticate`
- `ReportStatus`
- `RegisterAgent`
- `Load`
- `Statuses`
- `Agents`
- `IsSandboxAgent`
- `SetSandboxAgent`
- `ClearSandboxAgents`

### `internal/agentproto` (controller-agent DTOs)

- `ClientListResponse`
- `StatusReport`
- `XrayStatus`
