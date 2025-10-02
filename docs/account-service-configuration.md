# Account Service 配置指南

本文档说明账号服务可用的配置项、加载顺序以及示例，方便在不同环境中快速调整运行参数。

## 1. 配置加载策略

账号服务入口（`account/cmd/accountsvc/main.go`）会调用 `config.Load` 读取 YAML 配置，并允许通过命令行参数覆盖默认路径。当未提供配置文件时，服务会以零值启动，此时可结合环境变量填充关键字段。

当前推荐的覆盖顺序如下：

1. **命令行参数**：用于指定配置文件路径或运行模式。
2. **配置文件**：默认从 `account/config/account.yaml` 读取，适合提交到仓库或挂载到容器内。
3. **代码默认值**：`config.Config` 结构体中的零值，保证最小可运行。

> 注：目前服务尚未内置环境变量映射逻辑，如需按环境注入配置，可在部署流程中提前生成 YAML 文件或扩展 `config.Load`。

## 2. 配置字段参考

`account/config/config.go` 定义了配置结构，主要包含以下几个部分：

```yaml
log:
  level: info            # 可选：debug、info、warn、error

server:
  addr: ":8080"          # 监听地址
  readTimeout: 15s        # 读取超时
  writeTimeout: 15s       # 写入超时
  tls:                    # 启用 HTTPS 时的证书配置
    certFile: "/etc/ssl/certs/account.pem"
    keyFile: "/etc/ssl/private/account.key"
    clientCAFile: ""      # （可选）双向 TLS CA
    redirectHttp: false    # 当启用 TLS 时是否同时监听 HTTP 做 301 重定向

store:
  driver: "postgres"      # 可选：memory、postgres
  dsn: "postgres://user:pass@db:5432/account?sslmode=disable"
  maxOpenConns: 30
  maxIdleConns: 10

session:
  ttl: 24h                # 登录会话有效期
```

**TLS 提示**：当 `certFile` 和 `keyFile` 都非空时，`accountsvc` 会调用 `ListenAndServeTLS` 启动 HTTPS。如果同时希望保留 80 端口，可将 `redirectHttp` 置为 `true`，服务会开启一个额外的明文监听，将请求 301 重定向到 HTTPS。

**MFA 相关接口**：账号服务在 `/api/auth/mfa/*` 下提供 MFA 绑定与验证接口，默认无需额外配置即可使用，但生产环境建议将 `server.tls` 打开，确保 MFA 秘钥与 TOTP 码在传输过程中被加密。

## 3. 配置示例

### 3.1 开发环境（HTTP + 内存存储）

```yaml
log:
  level: debug
server:
  addr: ":8080"
  readTimeout: 0s
  writeTimeout: 0s
store:
  driver: "memory"
session:
  ttl: 8h
```

### 3.2 生产环境（PostgreSQL + HTTPS + MFA）

```yaml
log:
  level: info
server:
  addr: ":8443"
  readTimeout: 15s
  writeTimeout: 15s
  tls:
    certFile: "/etc/ssl/certs/account.pem"
    keyFile: "/etc/ssl/private/account.key"
    redirectHttp: true
store:
  driver: "postgres"
  dsn: "postgres://account:strongpass@db:5432/account?sslmode=require"
  maxOpenConns: 50
  maxIdleConns: 10
session:
  ttl: 24h
```

在生产环境中，建议通过 Kubernetes Secret、Vault 等方式挂载证书文件，并使用 `redirectHttp` 确保历史链接能够自动切换到 HTTPS。

## 4. 配置校验与回滚

- 启动时若启用 PostgreSQL，请确保 `dsn` 可用，否则服务会在初始化阶段返回错误。
- TLS 文件路径错误会导致启动失败，建议在 CI/CD 中加入探针验证。
- 通过 Git 管理配置文件，配合版本标签可实现快速回滚。

## 5. 与其他模块的协同

- 登录会话 TTL 会同步影响 `/api/auth/login`、`/api/auth/session` 等接口返回的 cookie 过期时间。
- 新增的 MFA 接口（`/api/auth/mfa/totp/provision`、`/api/auth/mfa/totp/verify`、`/api/auth/mfa/status`）在 HTTPS 环境下可与前端 MFA 向导配合使用，保证首次登录后必须完成绑定。
- 如果部署了前端 Next.js 应用，请确保其 `.env` 中的 `ACCOUNT_API_BASE` 指向启用了 TLS 的账号服务地址。

随着服务演进，请在更新配置结构或新字段时同步维护本文档。
