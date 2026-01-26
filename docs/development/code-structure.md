# 代码组织说明

- `cmd/`：可执行程序入口
- `api/`：HTTP 接口与业务控制层
- `internal/store/`：数据存储层（内存/PG）
- `internal/auth/`：JWT 令牌与鉴权中间件
- `internal/mailer/`：SMTP 邮件发送
- `internal/agentmode/`：Agent 模式实现
- `internal/agentserver/`：Controller 侧 Agent 管理
- `internal/xrayconfig/`：Xray 配置生成与同步
- `internal/service/`：管理员设置与指标聚合
- `sql/`：数据库 schema 与同步脚本
- `scripts/`：构建与运维脚本
