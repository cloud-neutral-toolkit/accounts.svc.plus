# 核心概念

以下概念来自当前代码与数据库结构，帮助理解账户服务的核心模型。

## 用户（User）

用户包含：
- `username` / `email` / `password`
- `role` 与 `level`：`admin` / `operator` / `user`
- `groups` / `permissions`
- MFA 状态（TOTP）与邮箱验证状态

## 会话（Session）

- 登录成功后产生会话 token
- 会话默认保存在进程内存（非持久化）
- 客户端可使用 `xc_session` Cookie 或 `Authorization: Bearer <token>`

## 邮件验证

- SMTP 未配置或使用 `*.example.com` 时自动关闭验证
- 启用后，注册需要邮箱验证码

## MFA（TOTP）

- 通过 `/api/auth/mfa/totp/provision` 生成 TOTP 秘钥
- 通过 `/api/auth/mfa/totp/verify` 完成验证并启用 MFA
- 可通过 `/api/auth/mfa/disable` 关闭

## 订阅（Subscription）

- 订阅信息保存在 `subscriptions` 表
- 支持 upsert 与 cancel
- 用于运营侧的订阅状态统计

## 管理权限矩阵（Admin Settings）

- 用于模块级权限开关
- 存储在 `admin_settings` 表
- 通过 `GET/POST /api/auth/admin/settings` 读取/更新

## Agent / Xray 同步

- Controller（账号服务）暴露 `/api/agent-server/v1` 接口
- Agent 定时拉取用户列表生成 Xray 配置
- Agent 上报健康状态供管理员查看

## 数据导入导出与同步

- `migratectl export/import`：YAML 快照导入导出
- `syncctl push/pull/mirror`：通过 SSH 进行跨环境同步
- `sql/` 下提供 pgsync 与 pglogical 同步脚本
