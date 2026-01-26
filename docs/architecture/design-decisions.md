# 关键设计取舍

## 会话存储为内存
- 优点：实现简单、无额外依赖
- 代价：重启丢失会话，无法横向扩展

## 主业务使用原生 SQL + pgx
- `internal/store/postgres.go` 使用 `database/sql` + pgx
- 优点：可控的 SQL 与更清晰的 schema 兼容逻辑
- 代价：代码量较高

## Admin Settings 使用 GORM
- 管理权限矩阵更新频率较低
- 使用 GORM 简化结构映射与事务处理

## 邮件验证与 SMTP 可选
- 未配置 SMTP 或使用示例域名时禁用验证
- 避免测试环境误发送邮件

## Agent 认证为预共享 Token
- 通过配置中的 token 哈希验证 Agent
- 适合私有网络与受控部署场景

## Xray 配置生成方式
- 定期从用户列表生成配置文件并原子写入
- 支持自定义模板与验证/重启命令

## JWT Token Service 作为可选能力
- 提供 public/access/refresh 机制
- 当前版本仍以会话 token 为主（详见 `api/auth.md`）
