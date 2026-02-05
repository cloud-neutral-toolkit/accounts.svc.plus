-- Migration: Add unique constraint on sessions.token
-- Date: 2026-02-05
-- Description: Adds a unique index on the sessions.token column to support
--              the ON CONFLICT clause in session upsert operations.

CREATE UNIQUE INDEX IF NOT EXISTS sessions_token_uk ON public.sessions (token);
