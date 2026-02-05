-- Agent registration and health tracking
-- Migration: 20260205_agents_table.sql

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

COMMENT ON TABLE public.agents IS 'Registered agents with health tracking and automatic cleanup';
COMMENT ON COLUMN public.agents.id IS 'Self-reported agent ID from StatusReport.agentId';
COMMENT ON COLUMN public.agents.last_heartbeat IS 'Last successful heartbeat timestamp, used for stale agent cleanup';
COMMENT ON COLUMN public.agents.groups IS 'Agent groups inherited from authentication credential';
