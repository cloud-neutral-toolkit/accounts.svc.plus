# VLESS QR Code 500 Error - 诊断报告

## 问题总结

用户登录后访问 `/panel` 页面,VLESS QR 码显示错误:
- 前端错误提示: "无法获取您的 UUID"
- 浏览器控制台错误: `[VLESS] Cannot build URI: node is undefined`
- API 错误: `/api/agent/nodes` 返回 500 Internal Server Error

## 根本原因分析

### 1. Agent Registry 配置问题

**关键映射关系**:
- `INTERNAL_SERVICE_TOKEN` (accounts.svc.plus 环境变量) ⟷ `apiToken` (agent 配置)
- 两者必须完全一致才能认证成功

**accounts.svc.plus 配置逻辑** (main.go:644-673):
```go
var agentRegistry *agentserver.Registry
if len(cfg.Agents.Credentials) > 0 {
    // 使用配置文件中的 credentials
    agentRegistry, err = agentserver.NewRegistry(...)
} else if token := os.Getenv("INTERNAL_SERVICE_TOKEN"); token != "" {
    // Fallback: 使用环境变量 INTERNAL_SERVICE_TOKEN
    agentRegistry, err = agentserver.NewRegistry(agentserver.Config{
        Credentials: []agentserver.Credential{{
            ID:     "internal-agent",
            Name:   "Internal Agent",
            Token:  token,  // ← 这里使用 INTERNAL_SERVICE_TOKEN
            Groups: []string{"internal"},
        }},
    })
}
```

**问题**: 
- accounts.svc.plus 部署在 Cloud Run 上
- **必须**设置环境变量 `INTERNAL_SERVICE_TOKEN=uTvryFvAbz6M5sRtmTaSTQY6otLZ95hneBsWqXu+35I=`
- 这个值必须与 agent 的 `apiToken` 完全一致
- 如果没有设置,`agentRegistry` 为 `nil`,导致无法接收 agent 心跳

### 2. Agent 配置正确

**agent.svc.plus 配置** (hk-xhttp.svc.plus `/etc/agent/account-agent.yaml`):
```yaml
agent:
  id: "hk-xhttp.svc.plus"
  controllerUrl: "https://accounts-svc-plus-266500572462.asia-northeast1.run.app"
  apiToken: "uTvryFvAbz6M5sRtmTaSTQY6otLZ95hneBsWqXu+35I="  # ← 必须与 INTERNAL_SERVICE_TOKEN 一致
  statusInterval: 1m
```

✅ Agent 配置正确,`apiToken` 值为 `uTvryFvAbz6M5sRtmTaSTQY6otLZ95hneBsWqXu+35I=`

### 3. API 流程分析

**正常流程**:
```
1. Agent (hk-xhttp.svc.plus) 
   → POST /api/agent-server/v1/status
   → 发送心跳和状态

2. accounts.svc.plus 
   → agentRegistry.ReportStatus()
   → 存储 agent 状态

3. 用户访问 /api/agent/nodes
   → listAgentNodes()
   → registeredNodeMetadata(h.agentStatusReader)
   → agentRegistry.Statuses()
   → 返回节点列表
```

**当前问题流程**:
```
1. Agent 正常发送心跳 ✅

2. accounts.svc.plus (Cloud Run)
   → agentRegistry = nil ❌
   → 无法存储 agent 状态

3. 用户访问 /api/agent/nodes
   → h.agentStatusReader = nil
   → registeredNodeMetadata() 返回 (nil, nil)
   → hosts = [] (空数组)
   → 返回 500 错误或空数组
```

## 解决方案

### 方案 1: 设置 Cloud Run 环境变量 (推荐)

在 Cloud Run 部署配置中添加环境变量:

```bash
gcloud run services update accounts-svc-plus \
  --region=asia-northeast1 \
  --set-env-vars="INTERNAL_SERVICE_TOKEN=uTvryFvAbz6M5sRtmTaSTQY6otLZ95hneBsWqXu+35I="
```

或通过 Cloud Run Console:
1. 打开 https://console.cloud.google.com/run
2. 选择 `accounts-svc-plus` 服务
3. 点击 "EDIT & DEPLOY NEW REVISION"
4. 在 "Variables & Secrets" 标签页添加:
   - Name: `INTERNAL_SERVICE_TOKEN`
   - Value: `uTvryFvAbz6M5sRtmTaSTQY6otLZ95hneBsWqXu+35I=`
5. 点击 "DEPLOY"

### 方案 2: 使用配置文件

创建 `config.yaml` 并在 Cloud Run 中挂载:

```yaml
agents:
  credentials:
    - id: "hk-xhttp.svc.plus"
      name: "HK XHTTP Proxy"
      token: "uTvryFvAbz6M5sRtmTaSTQY6otLZ95hneBsWqXu+35I="
      groups: ["production"]
```

## 验证步骤

### 1. 检查 Cloud Run 环境变量

```bash
gcloud run services describe accounts-svc-plus \
  --region=asia-northeast1 \
  --format="value(spec.template.spec.containers[0].env)"
```

### 2. 检查 Agent 心跳

SSH 到 hk-xhttp.svc.plus:
```bash
ssh root@hk-xhttp.svc.plus
journalctl -u agent-svc-plus -f
```

应该看到类似输出:
```
agent status updated agent=hk-xhttp.svc.plus healthy=true clients=X
```

### 3. 测试 API

```bash
# 获取 session token
TOKEN="your-xc_session-cookie"

# 测试 nodes API
curl -H "Cookie: xc_session=$TOKEN" \
  https://console.svc.plus/api/agent/nodes | jq '.'
```

预期输出:
```json
[
  {
    "name": "HK XHTTP Proxy",
    "address": "hk-xhttp.svc.plus",
    "transport": "xhttp",
    "uri_scheme_tcp": "vless://...",
    "uri_scheme_xhttp": "vless://..."
  }
]
```

## 前端改进

已完成以下改进:

### 1. 精确的错误提示

现在会显示具体缺失的变量:
- ❌ UUID 缺失
- ❌ 节点数据缺失 (无法从服务器获取代理节点列表)
- ❌ 有效节点缺失
- ❌ Transport 类型缺失
- ❌ URI Scheme 缺失 (tcp/xhttp)

### 2. 代码改动

文件: `console.svc.plus/src/modules/extensions/builtin/user-center/components/VlessQrCard.tsx`

- 添加了详细的错误分支判断
- 每个错误都有明确的标题和说明
- 帮助用户快速定位问题

## 数据库 Schema

已检查 SQL 文件:
- ✅ `sql/20260204_rbac_root_constraints.sql` - RBAC schema 更新
- ✅ `sql/schema.sql` - 主 schema
- ✅ 无需额外的 schema 更新

RBAC 相关表会在应用启动时自动创建 (main.go:431-510)

## 下一步行动

1. **立即执行**: 在 Cloud Run 中设置 `INTERNAL_SERVICE_TOKEN` 环境变量
2. **验证**: 重新部署后测试 `/api/agent/nodes` API
3. **监控**: 检查 agent 心跳日志确认连接正常
4. **测试**: 在浏览器中验证 VLESS QR 码生成

## 相关文件

- Agent 配置: `/etc/agent/account-agent.yaml` (hk-xhttp.svc.plus)
- Agent 环境变量: `agent.svc.plus/.env`
- Accounts 主程序: `accounts.svc.plus/cmd/accountsvc/main.go`
- API 处理: `accounts.svc.plus/api/user_agents.go`
- 前端组件: `console.svc.plus/src/modules/extensions/builtin/user-center/components/VlessQrCard.tsx`
