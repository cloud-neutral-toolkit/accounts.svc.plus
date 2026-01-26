# XControl Account Service 文档

本文档集覆盖 `accounts.svc.plus` 账号服务的安装、配置、使用、架构、运维与贡献指南。内容基于当前代码与配置模板整理，便于在不同环境快速落地。

## 快速入口

- 新手：`getting-started/introduction.md`、`getting-started/quickstart.md`
- 架构：`architecture/overview.md`、`architecture/components.md`
- 配置与使用：`usage/config.md`、`usage/deployment.md`、`usage/examples.md`
- API：`api/overview.md`、`api/endpoints.md`、`api/errors.md`
- 运维：`operations/monitoring.md`、`operations/troubleshooting.md`

## 文档结构

- `getting-started/`：10 分钟跑起来
- `architecture/`：Why > How 的架构说明
- `usage/`：如何配置与使用
- `api/`：接口说明与错误约定
- `integrations/`：数据库、云、邮件与第三方对接
- `advanced/`：性能、安全、扩展
- `development/`：开发与贡献
- `operations/`：监控、日志、备份与排障
- `governance/`：许可证与发布流程
- `appendix/`：FAQ、术语表与参考资料

> 备注：仓库内已有 `docs/SMTP_GMAIL_SETUP.md`（Cloud Run + Gmail SMTP），在 `integrations/cloud.md` 中有交叉引用。
