# 云部署与集成

## GCP Cloud Run

- 配置文件：`deploy/gcp/cloud-run/service.yaml`
- 配置模板：`config/account.cloudrun.yaml`
- 构建脚本：`scripts/cloudrun-build.sh`
- 部署脚本：`scripts/cloudrun-deploy.sh`

Cloud Run 模板包含 stunnel sidecar，用于连接数据库。

## Secrets 与环境变量

示例环境变量：
- `DB_HOST` / `DB_PORT` / `DB_NAME`
- `POSTGRES_USER` / `POSTGRES_PASSWORD`
- `SMTP_HOST` / `SMTP_PORT` / `SMTP_USERNAME` / `SMTP_PASSWORD`

## SMTP Gmail 参考

- 指南文件：`docs/SMTP_GMAIL_SETUP.md`
- 场景：Cloud Run + Gmail SMTP

> 若不需要邮件功能，直接置空 SMTP 配置即可。
