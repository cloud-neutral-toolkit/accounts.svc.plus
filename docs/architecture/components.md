# 组件职责边界

## 应用入口
- `cmd/accountsvc`：主服务入口（server/agent/server-agent）
- `cmd/createadmin`：创建/更新超级管理员
- `cmd/migratectl`：数据库迁移与导入导出
- `cmd/syncctl`：跨环境同步工具

## API 层
- `api/`：HTTP 路由、请求校验、响应与错误格式
- `api/admin_*`：管理员指标与 Agent 状态接口

## 认证与安全
- `internal/auth`：JWT 令牌服务与中间件
- `internal/store`：密码哈希与角色等级规范化

## 数据存储
- `internal/store`：Store 接口与内存/PostgreSQL 实现
- `sql/`：schema 与 pgsync/pglogical 相关脚本

## 邮件与通知
- `internal/mailer`：SMTP 发送器与 TLS 模式支持

## Xray 相关
- `internal/xrayconfig`：Xray 配置生成与同步
- `internal/agentmode`：Agent 控制循环（拉取用户、上报状态）
- `internal/agentserver`：Controller 侧 Agent 注册与状态管理

## 服务层
- `internal/service`：管理员权限矩阵与用户指标聚合
