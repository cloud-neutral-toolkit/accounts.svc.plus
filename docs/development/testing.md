# Testing Guide / 测试策略

## 中文

### 文档校验基线

本仓库当前最可靠的文档一致性校验命令是：

```bash
go test ./...
```

原因：

- 它直接覆盖当前 Go 模块里的可编译包。
- 文档中涉及的包名、导出符号、接口签名、请求结构和行为路径都必须至少与这个基线不冲突。
- 本次工程文档补齐以源码为唯一事实源，因此文档更新完成后应重新执行这条命令确认基线仍为绿色。

### 常用命令

```bash
go test ./...
make test
make integration-test
```

说明：

- `make test` 是仓库对 `go test` 的脚本封装。
- `make integration-test` 运行 `tests/e2e/superadmin-login/run-test-scripts.sh`，依赖本地数据库与初始化流程。

### 当前包级测试面

| 包 | 当前状态 | 说明 |
| --- | --- | --- |
| `account/api` | 有测试 | 覆盖注册、登录、MFA、session、sync config、admin settings、xworkmate、accounting、homepage video 等主 API 路径。 |
| `account/internal/agentserver` | 有测试 | 覆盖 registry 构造、认证、状态上报。 |
| `account/internal/mailer` | 有测试 | 覆盖 TLS mode 解析与 sender 构造。 |
| `account/internal/service` | 有测试 | 覆盖 user metrics 聚合行为。 |
| `account/internal/store` | 有测试 | 覆盖 memory / postgres 相关关键行为与 xworkmate 归一化。 |
| `account/internal/xrayconfig` | 有测试 | 覆盖 generator、gorm source、periodic syncer。 |
| `cmd/*`、`internal/auth`、`internal/agentmode`、`internal/syncer`、`internal/utils`、`internal/model`、`config`、`sql` | 暂无测试文件 | 文档应以源码对读为主，并明确哪些行为缺少直接测试保护。 |

### 文档更新后的核对清单

1. `RegisterRoutes` 中出现的 endpoint，文档必须存在且路径、方法、鉴权方式一致。
2. `internal/store.Store`、`auth.TokenService`、`service.UserMetricsService`、`xrayconfig.PeriodicSyncer` 等关键接口的签名必须与源码逐字对齐。
3. 对于暂无测试文件的包，文档必须避免推测性表述，只记录源码中可直接确认的行为。

## English

### Documentation Verification Baseline

The most reliable verification command for documentation alignment in this repository is:

```bash
go test ./...
```

Why this is the baseline:

- It covers all buildable Go packages in the current module.
- Package names, exported symbols, interface signatures, request structs, and behavior described in the docs must remain compatible with this baseline.
- This documentation refresh uses source code as the only source of truth, so the baseline should be rerun after every doc update.

### Common Commands

```bash
go test ./...
make test
make integration-test
```

Notes:

- `make test` is the repository wrapper around Go tests.
- `make integration-test` runs `tests/e2e/superadmin-login/run-test-scripts.sh` and depends on local database setup plus initialization steps.

### Current Package-Level Test Surface

| Package | Current status | Notes |
| --- | --- | --- |
| `account/api` | Tested | Covers registration, login, MFA, session, sync config, admin settings, xworkmate, accounting, and homepage video paths. |
| `account/internal/agentserver` | Tested | Covers registry construction, authentication, and status reporting. |
| `account/internal/mailer` | Tested | Covers TLS mode parsing and sender construction. |
| `account/internal/service` | Tested | Covers user metrics aggregation behavior. |
| `account/internal/store` | Tested | Covers key memory / postgres behavior plus xworkmate normalization. |
| `account/internal/xrayconfig` | Tested | Covers generator, GORM source, and periodic syncer behavior. |
| `cmd/*`, `internal/auth`, `internal/agentmode`, `internal/syncer`, `internal/utils`, `internal/model`, `config`, `sql` | No direct test files | Documentation for these packages should stay source-driven and avoid speculative claims. |

### Post-Update Checklist

1. Every endpoint registered in `RegisterRoutes` must exist in the docs with matching path, method, and auth mode.
2. Key signatures such as `internal/store.Store`, `auth.TokenService`, `service.UserMetricsService`, and `xrayconfig.PeriodicSyncer` must match the source exactly.
3. For packages without direct tests, the docs must record only behavior that is directly visible in source.
