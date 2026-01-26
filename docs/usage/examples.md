# 常见使用场景

> 示例默认假设 `auth.enable=false`，并使用会话 token 进行认证。

## 注册（邮件验证关闭）

```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"demo","email":"demo@example.com","password":"Passw0rd!"}'
```

## 邮件验证码注册（SMTP 已启用）

```bash
# 发送验证码
curl -X POST http://localhost:8080/api/auth/register/send \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com"}'

# 验证并完成注册
curl -X POST http://localhost:8080/api/auth/register/verify \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com","code":"123456"}'
```

## 登录并获取会话

```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"identifier":"demo@example.com","password":"Passw0rd!"}'
```

返回示例（字段可能包含 `mfaToken`）：

```json
{
  "message": "login successful",
  "token": "<session-token>",
  "expiresAt": "<timestamp>",
  "user": {"id":"..."}
}
```

## 会话查询

```bash
curl -H 'Authorization: Bearer <session-token>' \
  http://localhost:8080/api/auth/session
```

## MFA 绑定

```bash
# 申请 TOTP secret
curl -X POST http://localhost:8080/api/auth/mfa/totp/provision \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <session-token>' \
  -d '{}'

# 验证 TOTP
curl -X POST http://localhost:8080/api/auth/mfa/totp/verify \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <session-token>' \
  -d '{"mfaToken":"<mfa-token>","totpCode":"123456"}'
```

## 订阅 Upsert

```bash
curl -X POST http://localhost:8080/api/auth/subscriptions \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <session-token>' \
  -d '{"externalId":"sub_001","provider":"stripe","kind":"subscription","status":"active"}'
```

## 管理设置更新

```bash
curl -X POST http://localhost:8080/api/auth/admin/settings \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <session-token>' \
  -d '{"version":1,"matrix":{"billing":{"admin":true,"operator":false}}}'
```
