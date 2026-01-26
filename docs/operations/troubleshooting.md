# 常见问题排查

## 登录成功后提示 session invalid

- 会话存储在内存中，重启服务会导致失效
- 多实例环境会话无法跨实例共享

## 开启 auth.enable 后所有接口 401

- JWT 中间件需要 `Authorization: Bearer <access-token>`
- 但多数接口仍使用会话 token 校验
- 建议暂时关闭 `auth.enable` 或调整业务逻辑

## 邮件验证未生效

- `smtp.host` 为空或为 `*.example.com` 会自动关闭验证
- 检查 SMTP 配置与网络连通性

## CORS 跨域失败

- 检查 `server.allowedOrigins`
- 若未配置，服务会尝试使用 `publicUrl` 或默认本地地址

## 数据库连接失败

- `config/account.yaml` 默认端口为 5432
- Makefile 脚本默认端口为 15432
- 确保二者一致或按需调整
