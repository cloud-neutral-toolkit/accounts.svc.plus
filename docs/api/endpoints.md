# 接口列表

## 公共

- `GET /healthz`：健康检查

## 账号认证（/api/auth）

- `POST /api/auth/register`：注册
- `POST /api/auth/register/send`：发送邮箱验证码
- `POST /api/auth/register/verify`：验证邮箱验证码
- `POST /api/auth/login`：登录
- `POST /api/auth/token/exchange`：public token 换取 access/refresh
- `POST /api/auth/token/refresh`：刷新 access token

### 需要会话（或受保护）

- `GET /api/auth/session`：获取当前会话用户
- `DELETE /api/auth/session`：注销
- `POST /api/auth/mfa/totp/provision`：申请 MFA TOTP secret
- `POST /api/auth/mfa/totp/verify`：验证 MFA TOTP
- `POST /api/auth/mfa/disable`：关闭 MFA
- `GET /api/auth/mfa/status`：查询 MFA 状态
- `POST /api/auth/password/reset`：发起密码重置（需要登录）
- `POST /api/auth/password/reset/confirm`：确认密码重置
- `GET /api/auth/subscriptions`：订阅列表
- `POST /api/auth/subscriptions`：订阅 upsert
- `POST /api/auth/subscriptions/cancel`：取消订阅
- `POST /api/auth/config/sync`：配置同步（当前返回未实现）
- `GET /api/auth/admin/settings`：获取权限矩阵
- `POST /api/auth/admin/settings`：更新权限矩阵
- `GET /api/auth/admin/users/metrics`：用户指标
- `GET /api/auth/admin/agents/status`：Agent 状态

> 说明：`/api/auth/admin/*` 需要管理员或运维角色。

## Agent API（/api/agent/v1）

- `GET /api/agent/v1/users`：获取 Xray 客户端列表
- `POST /api/agent/v1/status`：上报 Agent 状态

Agent 认证通过 `Authorization: Bearer <agent-token>`。
