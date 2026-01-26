# 认证与鉴权

## 会话认证（默认）

1) 登录 `POST /api/auth/login` 成功后返回：
- `token`：会话 token
- `expiresAt`
- `user`

2) 客户端后续请求携带：
- `Authorization: Bearer <session-token>` 或
- Cookie `xc_session=<session-token>`

## 邮件验证

- 发送验证码：`POST /api/auth/register/send`
- 验证并注册：`POST /api/auth/register/verify`

当 SMTP 未配置或使用示例域名时，邮箱验证会自动关闭。

## MFA（TOTP）

- 申请 secret：`POST /api/auth/mfa/totp/provision`
- 验证并启用：`POST /api/auth/mfa/totp/verify`
- 关闭 MFA：`POST /api/auth/mfa/disable`

登录接口在部分场景会返回 `mfaToken`，用于后续验证。

## JWT 令牌服务（可选）

启用 `auth.enable: true` 后提供：
- `POST /api/auth/token/exchange`：使用 `public_token` 换取 access/refresh
- `POST /api/auth/token/refresh`：刷新 access token

注意事项：
- `token/exchange` 需要调用方提供 `user_id/email/roles`
- 当前版本多数保护路由仍使用会话 token，JWT 仅作为中间件校验存在
- 若开启 JWT，中间件要求 `Authorization: Bearer <access-token>`，但业务逻辑仍可能需要会话 token

建议：若主要使用会话认证，请将 `auth.enable` 设为 `false`。
