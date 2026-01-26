# Quickstart（最小可运行示例）

以下步骤使用内存存储启动服务，适合本地快速验证 API 行为。

## 1. 准备最小配置

创建 `config/local.yaml`：

```yaml
mode: "server"
log:
  level: info
server:
  addr: ":8080"
store:
  driver: "memory"
  dsn: ""
session:
  ttl: 24h
smtp:
  host: ""
  port: 587
  username: ""
  password: ""
  from: ""
  replyTo: ""
  timeout: 10s
  tls:
    mode: "auto"
    insecureSkipVerify: false
```

说明：
- `store.driver=memory` 可跳过数据库
- `smtp.host=""` 会关闭邮件验证

## 2. 启动服务

```bash
go run ./cmd/accountsvc/main.go --config config/local.yaml
```

## 3. 健康检查

```bash
curl http://localhost:8080/healthz
```

## 4. 注册与登录

```bash
# 注册
curl -X POST http://localhost:8080/api/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"demo","email":"demo@example.com","password":"Passw0rd!"}'

# 登录
curl -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"identifier":"demo@example.com","password":"Passw0rd!"}'
```

成功登录后返回 `token`，后续可使用：

```bash
curl -H 'Authorization: Bearer <session-token>' http://localhost:8080/api/auth/session
```

> 注意：会话保存在进程内存，重启服务会使会话失效。
