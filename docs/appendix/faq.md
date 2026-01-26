# FAQ

## Q: 为什么注册不需要邮箱验证码？
A: 当 `smtp.host` 为空或使用 `*.example.com` 时，系统自动关闭邮件验证。

## Q: 会话为什么会丢失？
A: 会话存储在进程内存中，重启或多实例会导致失效。

## Q: 如何启用多区域同步？
A: 参见 `sql/readme.md`，可使用 pgsync 或 pglogical。

## Q: auth.enable 打开后无法访问接口？
A: JWT 中间件会生效，但多数接口仍依赖会话 token，请参考 `api/auth.md`。
