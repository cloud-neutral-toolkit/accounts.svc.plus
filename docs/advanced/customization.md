# 二次开发与定制

## 配置模板

- `config/*.yaml` 可作为环境模板
- 容器中可通过 `CONFIG_TEMPLATE` + `envsubst` 渲染

## 自定义 Xray 模板

- 设置 `xray.sync.templatePath`
- 模板必须包含 `inbounds[0].settings.clients` 数组

## 扩展存储

- 实现 `internal/store.Store` 接口
- 在 `store.New` 中注册新驱动

## 邮件发送

- SMTP 逻辑集中在 `internal/mailer`
- 可替换为 API 驱动（SendGrid 等）

## API 扩展

- 路由集中于 `api/` 与 `cmd/accountsvc/main.go`
- 建议保持统一错误结构
