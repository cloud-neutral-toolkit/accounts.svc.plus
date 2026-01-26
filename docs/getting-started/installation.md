# 安装方式

本项目支持本地编译运行、Docker 容器运行，以及 Cloud Run 部署。以下步骤基于仓库现有脚本与配置。

## 本地安装（Go）

前置条件：
- Go 1.25.1（见 `go.mod`）
- PostgreSQL（若使用 `store.driver=postgres`）

编译与运行：

```bash
make build
./xcontrol-account --config config/account.yaml
```

或直接运行：

```bash
go run ./cmd/accountsvc/main.go --config config/account.yaml
```

## Docker

仓库包含多阶段 Dockerfile，运行时使用 `entrypoint.sh` 生成配置：

```bash
docker build -t accounts-svc-plus .

docker run --rm -p 8080:8080 \
  -e CONFIG_TEMPLATE=/app/config/account.yaml \
  -e CONFIG_PATH=/etc/xcontrol/account.yaml \
  accounts-svc-plus
```

说明：
- `entrypoint.sh` 会将 `CONFIG_TEMPLATE` 通过 `envsubst` 渲染成 `CONFIG_PATH`
- 若设置了 `PORT` 环境变量，脚本会自动修改 `server.addr`

## Cloud Run

参考文件：
- `deploy/gcp/cloud-run/service.yaml`
- `deploy/gcp/cloud-run/stunnel.conf`
- `config/account.cloudrun.yaml`

构建与部署脚本：

```bash
GCP_PROJECT=your-project make cloudrun-build
GCP_PROJECT=your-project make cloudrun-deploy
```

Cloud Run 部署模板已包含：
- 主应用容器（Account API）
- Stunnel Sidecar（用于数据库安全隧道）
- SMTP Secret 环境变量注入示例
