# API 设计原则

- REST 风格 + JSON
- 大多数接口使用错误结构：`{"error":"code","message":"..."}`
- 少数接口直接返回 `{"error":"..."}`
- 默认以会话 token 作为认证方式

## 基础路径

- 健康检查：`GET /healthz`
- 用户 API：`/api/auth/*`
- Agent API：`/api/agent-server/v1/*`

## 认证方式

- 会话 token：`Authorization: Bearer <session-token>` 或 `xc_session` Cookie
- JWT（可选）：启用 `auth.enable` 后，对 `/api/auth/*` 保护路由增加 JWT 校验

> 注意：当前实现中，大部分保护路由仍依赖会话 token。若开启 JWT 中间件，需确保请求同时满足会话逻辑（详见 `api/auth.md` 的说明）。
