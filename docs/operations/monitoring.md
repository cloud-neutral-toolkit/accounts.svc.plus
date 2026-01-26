# 监控

当前服务提供以下监控入口：

- `GET /healthz`：存活检查
- `GET /api/auth/admin/users/metrics`：用户指标（需管理员）
- 日志输出：stdout

暂未提供 Prometheus/OpenTelemetry 指标，需要时可在此扩展。
