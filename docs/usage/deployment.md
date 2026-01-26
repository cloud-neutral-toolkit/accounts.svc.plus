# 部署方式

## 本地或 VM

推荐通过 Makefile 与脚本执行：

```bash
make init-db
make build
make start
```

默认启动脚本 `scripts/start.sh` 使用 `config/account.yaml`。

## Docker

详见 `getting-started/installation.md`。

## Cloud Run

仓库内的 Cloud Run 配置：
- `deploy/gcp/cloud-run/service.yaml`
- `config/account.cloudrun.yaml`

特点：
- 通过 `entrypoint.sh` + `CONFIG_TEMPLATE` 注入配置
- 附带 stunnel sidecar，用于安全连接数据库
- SMTP 凭据通过 Secret 注入

## stunnel（数据库连接）

- 模板：`deploy/stunnel-account-db-client.conf` / `deploy/stunnel-account-db-server.conf`
- Cloud Run 示例：`deploy/gcp/cloud-run/stunnel.conf`

适合在数据库仅允许本地或专线访问的场景。
