# 架构总览

Account Service 是一个单体 Go 服务，提供账号、计费与运营相关能力，同时可作为 Xray Controller 管理 Agent。

## 逻辑架构（文字版）

```
Client
  └─ HTTP API (Gin)
       ├─ Session / MFA / Email verification
       ├─ Subscription & Admin Settings
       ├─ Usage / Billing aggregation
       ├─ Agent Controller (/api/agent-server/v1)
       └─ Token Service (optional)
           │
           ├─ Store (memory / postgres)
           ├─ Admin Settings DB (GORM, same DSN)
           ├─ SMTP Sender
           └─ Xray Config Sync
```

## 核心数据流

1) 用户注册/登录
- API 校验输入 → Store 持久化用户 → 生成会话 token
- 可选：发送邮件验证码/密码重置邮件

2) 管理权限矩阵
- `admin_settings` 表保存模块与角色的开关
- 通过 GORM 读写，内置缓存避免频繁查询

3) Xray 同步（Controller + Agent）
- Controller: 暴露用户列表与 Agent 状态接口
- Agent: 定时拉取用户列表生成 Xray 配置，并上报状态

4) 数据同步
- `traffic_minute_buckets` 保存分钟级流量快照，是 usage / billing 的基础事实表
- `billing_ledger` 保存计费分录，供前端和后台页面读取
- `account_quota_states`、`account_policy_snapshots`、`node_health_snapshots` 和 `scheduler_decisions` 共同构成控制平面的状态层
- 所有 usage / billing 响应都来自 PostgreSQL，不依赖 Prometheus

5) 工具同步
- `migratectl`：导入/导出 YAML 快照
- `syncctl`：通过 SSH 在不同环境间同步

## 关键边界

- 会话存储为进程内存（不可横向扩展）
- SMTP 未配置时自动关闭邮件验证
- JWT Token Service 为可选能力，需与会话机制配合使用
