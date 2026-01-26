# 错误码约定

常见格式（多数接口）：

```json
{"error":"code","message":"human readable message"}
```

## 常见错误码（节选）

- `invalid_request`：请求体错误
- `missing_credentials` / `credentials_in_query`
- `invalid_email`
- `password_too_short`
- `email_already_exists` / `name_already_exists`
- `invalid_session` / `session_token_required`
- `mfa_code_required` / `invalid_mfa_code`
- `token_service_unavailable`
- `invalid_public_token` / `invalid_refresh_token`
- `subscription_not_found`
- `agent_status_unavailable`

少数接口仅返回 `{"error":"..."}`。具体返回以接口实现为准，可在 `api/` 中查阅。
