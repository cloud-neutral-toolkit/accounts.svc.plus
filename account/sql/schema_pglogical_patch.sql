-- =========================================
-- schema_pglogical_patch.sql
-- Apply pglogical-aware defaults on top of the base business schema.
-- Run only when the cluster enables pglogical for bidirectional writes.
-- =========================================

-- Align origin_node defaults with pglogical node name when available.
ALTER TABLE public.users
  ALTER COLUMN origin_node SET DEFAULT COALESCE(current_setting('pglogical.node_name', true), 'local');

ALTER TABLE public.identities
  ALTER COLUMN origin_node SET DEFAULT COALESCE(current_setting('pglogical.node_name', true), 'local');

ALTER TABLE public.sessions
  ALTER COLUMN origin_node SET DEFAULT COALESCE(current_setting('pglogical.node_name', true), 'local');

ALTER TABLE public.admin_settings
  ALTER COLUMN origin_node SET DEFAULT COALESCE(current_setting('pglogical.node_name', true), 'local');
