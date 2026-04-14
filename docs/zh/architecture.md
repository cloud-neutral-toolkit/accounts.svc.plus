# 架构

本页是共享双语架构细页的中文入口。

## 当前架构一句话

`accounts.svc.plus` 是一个以 Gin 为入口的 Go 服务，同时承担：

- 账号与会话管理，
- 管理员控制面，
- agent / Xray 配置控制，
- 使用量与计费读面。

真实启动链路集中在 `cmd/accountsvc/main.go`：主 `store.Store`、GORM 管理面 DB、可选 mailer、可选 token service、agent registry、可选 Xray periodic syncer 都在这里装配完成，然后统一进入 `api.RegisterRoutes`。

## 建议阅读顺序

1. [架构总览](../architecture/overview.md)
   看主启动链路、请求路径、session 状态持有、agent 上报与 Xray 配置生成。
2. [组件职责图](../architecture/components.md)
   看按包划分的 owning responsibility、输入输出与依赖方向。
3. [设计决策](../architecture/design-decisions.md)
   看当前真实取舍，而不是只看结构。

## 当前架构主题

- Session first，JWT optional。
- `store.Store` 是主业务持久化抽象边界。
- GORM 只用于 admin settings、homepage video、sandbox binding、tenant / XWorkmate 模型。
- `agentserver.Registry` 负责把 agent 凭据、身份和状态投影成控制面读视图。
- Xray 配置以 `xrayconfig.Generator` + `PeriodicSyncer` 为核心收敛模型。

## 相关页面

- [设计](design.md)
- [开发手册](developer-guide.md)
- [API 总览](../api/overview.md)
