# Agent Persistence Implementation Plan

## 目标

将 agent 注册信息持久化到 PostgreSQL,并实现自动清理下线/失效的 agent。

## 当前状态

- ✅ Agent 通过共享 token 认证
- ✅ Agent 自报 ID 并动态注册到内存 registry
- ❌ Agent 信息未持久化,服务重启后丢失
- ❌ 没有自动清理下线 agent 的机制

## 数据库 Schema

### 新增 `agents` 表

```sql
CREATE TABLE IF NOT EXISTS public.agents (
    id TEXT PRIMARY KEY,                      -- Agent ID (e.g., "hk-xhttp.svc.plus")
    name TEXT NOT NULL DEFAULT '',            -- Display name
    groups TEXT[] NOT NULL DEFAULT '{}',      -- Agent groups (e.g., {"internal"})
    healthy BOOLEAN NOT NULL DEFAULT false,   -- Last reported health status
    last_heartbeat TIMESTAMPTZ,               -- Last successful heartbeat time
    clients_count INTEGER NOT NULL DEFAULT 0, -- Number of Xray clients
    sync_revision TEXT,                       -- Last sync revision
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agents_last_heartbeat ON public.agents(last_heartbeat);
CREATE INDEX IF NOT EXISTS idx_agents_healthy ON public.agents(healthy);
```

### 迁移脚本

**文件**: `sql/20260205_agents_table.sql`

```sql
-- Agent registration and health tracking
CREATE TABLE IF NOT EXISTS public.agents (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    groups TEXT[] NOT NULL DEFAULT '{}',
    healthy BOOLEAN NOT NULL DEFAULT false,
    last_heartbeat TIMESTAMPTZ,
    clients_count INTEGER NOT NULL DEFAULT 0,
    sync_revision TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agents_last_heartbeat ON public.agents(last_heartbeat);
CREATE INDEX IF NOT EXISTS idx_agents_healthy ON public.agents(healthy);

COMMENT ON TABLE public.agents IS 'Registered agents with health tracking';
COMMENT ON COLUMN public.agents.id IS 'Self-reported agent ID';
COMMENT ON COLUMN public.agents.last_heartbeat IS 'Last successful heartbeat timestamp';
```

## 代码修改

### 1. Store Interface 扩展

**文件**: `internal/store/store.go`

添加 agent 相关方法:

```go
// Agent represents a registered agent
type Agent struct {
    ID            string
    Name          string
    Groups        []string
    Healthy       bool
    LastHeartbeat *time.Time
    ClientsCount  int
    SyncRevision  string
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

// Store interface 添加方法
type Store interface {
    // ... existing methods ...
    
    // Agent management
    UpsertAgent(ctx context.Context, agent *Agent) error
    GetAgent(ctx context.Context, id string) (*Agent, error)
    ListAgents(ctx context.Context) ([]*Agent, error)
    DeleteAgent(ctx context.Context, id string) error
    DeleteStaleAgents(ctx context.Context, staleThreshold time.Duration) (int, error)
}
```

### 2. PostgreSQL Store 实现

**文件**: `internal/store/postgres.go`

```go
func (s *PostgresStore) UpsertAgent(ctx context.Context, agent *Agent) error {
    query := `
        INSERT INTO agents (id, name, groups, healthy, last_heartbeat, clients_count, sync_revision, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, now())
        ON CONFLICT (id) DO UPDATE SET
            name = EXCLUDED.name,
            groups = EXCLUDED.groups,
            healthy = EXCLUDED.healthy,
            last_heartbeat = EXCLUDED.last_heartbeat,
            clients_count = EXCLUDED.clients_count,
            sync_revision = EXCLUDED.sync_revision,
            updated_at = now()
    `
    _, err := s.db.ExecContext(ctx, query,
        agent.ID,
        agent.Name,
        pq.Array(agent.Groups),
        agent.Healthy,
        agent.LastHeartbeat,
        agent.ClientsCount,
        agent.SyncRevision,
    )
    return err
}

func (s *PostgresStore) ListAgents(ctx context.Context) ([]*Agent, error) {
    query := `
        SELECT id, name, groups, healthy, last_heartbeat, clients_count, sync_revision, created_at, updated_at
        FROM agents
        ORDER BY id
    `
    rows, err := s.db.QueryContext(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var agents []*Agent
    for rows.Next() {
        var a Agent
        err := rows.Scan(
            &a.ID,
            &a.Name,
            pq.Array(&a.Groups),
            &a.Healthy,
            &a.LastHeartbeat,
            &a.ClientsCount,
            &a.SyncRevision,
            &a.CreatedAt,
            &a.UpdatedAt,
        )
        if err != nil {
            return nil, err
        }
        agents = append(agents, &a)
    }
    return agents, rows.Err()
}

func (s *PostgresStore) DeleteStaleAgents(ctx context.Context, staleThreshold time.Duration) (int, error) {
    query := `
        DELETE FROM agents
        WHERE last_heartbeat < $1 OR last_heartbeat IS NULL
    `
    cutoff := time.Now().Add(-staleThreshold)
    result, err := s.db.ExecContext(ctx, query, cutoff)
    if err != nil {
        return 0, err
    }
    count, _ := result.RowsAffected()
    return int(count), nil
}
```

### 3. Registry 持久化集成

**文件**: `internal/agentserver/registry.go`

修改 `RegisterAgent` 和 `ReportStatus` 方法,添加数据库持久化:

```go
type Registry struct {
    mu          sync.RWMutex
    credentials map[[32]byte]Identity
    byID        map[string]Identity
    statuses    map[string]StatusSnapshot
    store       store.Store  // 新增: 数据库 store
}

func (r *Registry) RegisterAgent(agentID string, groups []string) Identity {
    r.mu.Lock()
    defer r.mu.Unlock()

    // Check if agent already registered in memory
    if identity, exists := r.byID[agentID]; exists {
        return identity
    }

    // Create new identity
    identity := Identity{
        ID:     agentID,
        Name:   agentID,
        Groups: groups,
    }

    r.byID[agentID] = identity

    // Persist to database (async, non-blocking)
    if r.store != nil {
        go func() {
            ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
            defer cancel()
            
            agent := &store.Agent{
                ID:     agentID,
                Name:   agentID,
                Groups: groups,
            }
            if err := r.store.UpsertAgent(ctx, agent); err != nil {
                // Log error but don't fail the registration
                slog.Warn("failed to persist agent", "agent", agentID, "err", err)
            }
        }()
    }

    return identity
}

func (r *Registry) ReportStatus(agent Identity, report agentproto.StatusReport) {
    r.mu.Lock()
    defer r.mu.Unlock()

    r.statuses[agent.ID] = StatusSnapshot{
        Agent:     agent,
        Report:    report,
        UpdatedAt: time.Now().UTC(),
    }

    // Update database with health status (async)
    if r.store != nil {
        go func() {
            ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
            defer cancel()
            
            now := time.Now()
            dbAgent := &store.Agent{
                ID:            agent.ID,
                Name:          agent.Name,
                Groups:        agent.Groups,
                Healthy:       report.Healthy,
                LastHeartbeat: &now,
                ClientsCount:  report.Xray.Clients,
                SyncRevision:  report.SyncRevision,
            }
            if err := r.store.UpsertAgent(ctx, dbAgent); err != nil {
                slog.Warn("failed to update agent status", "agent", agent.ID, "err", err)
            }
        }()
    }
}
```

### 4. 自动清理 Stale Agents

**文件**: `cmd/accountsvc/main.go`

添加后台清理任务:

```go
// 在 main 函数中启动清理任务
if agentRegistry != nil && st != nil {
    go runAgentCleanup(ctx, st, logger)
}

func runAgentCleanup(ctx context.Context, st store.Store, logger *slog.Logger) {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()

    staleThreshold := 10 * time.Minute // Agent 超过 10 分钟未心跳视为下线

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
            count, err := st.DeleteStaleAgents(cleanupCtx, staleThreshold)
            cancel()

            if err != nil {
                logger.Warn("failed to cleanup stale agents", "err", err)
            } else if count > 0 {
                logger.Info("cleaned up stale agents", "count", count)
            }
        }
    }
}
```

## 实施步骤

1. **创建迁移脚本** ✅
   - 创建 `sql/20260205_agents_table.sql`
   - 在本地和生产环境运行迁移

2. **扩展 Store Interface**
   - 添加 `Agent` 结构体
   - 添加 agent 管理方法到 `Store` interface

3. **实现 PostgreSQL Store 方法**
   - `UpsertAgent()` - 插入或更新 agent
   - `ListAgents()` - 列出所有 agent
   - `DeleteStaleAgents()` - 删除过期 agent

4. **修改 Registry**
   - 添加 `store` 字段
   - 在 `RegisterAgent()` 中持久化
   - 在 `ReportStatus()` 中更新心跳时间

5. **添加清理任务**
   - 实现 `runAgentCleanup()` 后台任务
   - 每 5 分钟清理一次超过 10 分钟未心跳的 agent

6. **测试**
   - 测试 agent 注册和心跳
   - 测试服务重启后 agent 恢复
   - 测试 agent 下线后自动清理

## 配置参数

可以通过环境变量配置:

- `AGENT_CLEANUP_INTERVAL` - 清理任务执行间隔 (默认: 5m)
- `AGENT_STALE_THRESHOLD` - Agent 失效阈值 (默认: 10m)

## 优势

1. **持久化**: Agent 信息在服务重启后保留
2. **自动清理**: 下线 agent 自动删除,避免数据库膨胀
3. **健康监控**: 可以查询 agent 健康状态和最后心跳时间
4. **审计**: 可以追踪 agent 注册和下线历史

## 注意事项

1. **异步持久化**: 数据库操作异步执行,不阻塞心跳响应
2. **失败容忍**: 数据库写入失败不影响 agent 功能
3. **内存优先**: 内存 registry 仍然是主要数据源,数据库作为备份
4. **清理策略**: 10 分钟未心跳视为下线,可根据实际情况调整
