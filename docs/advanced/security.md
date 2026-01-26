# 安全模型

## 身份与凭证

- 密码使用 bcrypt 哈希
- MFA 使用 TOTP（6 位，30s 窗口）
- 会话 token 存在内存中

## TLS 与传输安全

- Server TLS 配置：`server.tls.*`
- SMTP TLS 配置：`smtp.tls.mode`
- Agent 访问支持 TLS 校验开关

## 访问控制

- 会话鉴权：`Authorization` 或 Cookie
- 管理接口需要管理员或运维角色
- Agent 使用预共享 token 验证

## 安全注意事项

- API 显式拒绝在 query 参数中传递敏感信息
- 生产环境请使用真实 SMTP 配置并妥善管理 secrets
- 建议限制 CORS `allowedOrigins`
