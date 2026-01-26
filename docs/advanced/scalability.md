# 高可用与水平扩展

## 当前限制

- 会话存储在内存中，无法在多实例间共享
- 多实例部署会导致用户在不同实例间会话失效

## 建议方向

- 引入集中式会话存储（如 Redis）
- API 层保持无状态，通过负载均衡扩展
- 数据库采用主从或 pglogical 多主架构

## Agent 扩展

- Controller 通过配置 `agents.credentials` 支持多 Agent
- Agent 状态通过内存 Registry 维护
