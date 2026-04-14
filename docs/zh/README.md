# 账号与身份服务文档

当前文档集已经从占位摘要补齐为“实现级工程文档”。详细页统一落在共享双语页面 `docs/architecture/*`、`docs/api/*`、`docs/development/*`，而 `docs/zh/*` 继续承担中文入口页角色。

## 当前覆盖范围

- 从 `cmd/accountsvc/main.go` 出发的系统设计、启动链路、agent registry、Xray sync 与运行时边界。
- `api`、`internal/store`、`internal/auth`、`internal/service`、`internal/xrayconfig`、`internal/agentmode`、`internal/agentserver`、`internal/agentproto` 的包级职责与类型所有权。
- HTTP 接口的请求字段、返回字段、鉴权方式、owner handler file、错误约定与依赖对象。

## 推荐阅读路径

### 如果你想先看架构

1. [架构](architecture.md)
2. [启动与运行时主链路](../architecture/overview.md)
3. [组件职责图](../architecture/components.md)
4. [设计决策](../architecture/design-decisions.md)

### 如果你想先看 API 与接口契约

1. [开发手册](developer-guide.md)
2. [API 总览](../api/overview.md)
3. [认证与鉴权](../api/auth.md)
4. [接口矩阵](../api/endpoints.md)
5. [错误约定](../api/errors.md)

### 如果你想先看代码结构

1. [开发手册](developer-guide.md)
2. [代码结构参考](../development/code-structure.md)
3. [测试基线](../development/testing.md)

## 核心入口页

- [架构](architecture.md)
- [设计](design.md)
- [部署](deployment.md)
- [使用手册](user-guide.md)
- [开发手册](developer-guide.md)
- [Vibe Coding 参考](vibe-coding-reference.md)

## 说明

- 详细子系统页不再拆分为 `docs/zh/api/*` 之类的新目录，而是集中维护在共享双语细页中。
- 当前文档与源码一致性的验证基线是 `go test ./...`，详见 [测试基线](../development/testing.md)。
