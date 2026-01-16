-- =========================================
-- schema_pglogical_region.sql
-- pglogical configuration template for regional nodes
-- PostgreSQL 16+, 双向复制 (provider + subscriber)
-- 在运行本脚本前，请确保已执行 schema.sql 与 schema_pglogical_patch.sql。
-- =========================================

\if :{?NODE_NAME}
\else
\echo 'ERROR: 未设置 NODE_NAME 变量。请通过 -v NODE_NAME=... 传入节点名称。'
\quit 1
\endif

\if :{?NODE_DSN}
\else
\echo 'ERROR: 未设置 NODE_DSN 变量。请通过 -v NODE_DSN=... 传入当前节点 DSN。'
\quit 1
\endif

\if :{?SUBSCRIPTION_NAME}
\else
\echo 'ERROR: 未设置 SUBSCRIPTION_NAME 变量。请通过 -v SUBSCRIPTION_NAME=... 传入订阅名称。'
\quit 1
\endif

\if :{?PROVIDER_DSN}
\else
\echo 'ERROR: 未设置 PROVIDER_DSN 变量。请通过 -v PROVIDER_DSN=... 传入 Provider DSN。'
\quit 1
\endif

-- 🏗️ 确保 pglogical schema 及扩展存在
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_namespace WHERE nspname = 'pglogical'
  ) THEN
    EXECUTE format('CREATE SCHEMA pglogical AUTHORIZATION %I', current_user);
  END IF;
END;
$$;

CREATE EXTENSION IF NOT EXISTS pglogical WITH SCHEMA pglogical;

-- 🧭 清理旧节点（可安全重入）
DO $$
BEGIN
  PERFORM pglogical.drop_subscription(:'SUBSCRIPTION_NAME', true);
  EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
  PERFORM pglogical.drop_node(:'NODE_NAME');
  EXCEPTION WHEN others THEN NULL;
END $$;

-- =========================================
-- 创建本节点 (Provider)
-- =========================================
SELECT pglogical.create_node(
  node_name := :'NODE_NAME',
  dsn := :'NODE_DSN'
);

-- =========================================
-- 定义复制集
-- =========================================
SELECT pglogical.create_replication_set('rep_all');
SELECT pglogical.replication_set_add_all_tables('rep_all', ARRAY['public']);

-- =========================================
-- 创建订阅 (订阅远端节点)
-- =========================================
SELECT pglogical.create_subscription(
  subscription_name := :'SUBSCRIPTION_NAME',
  provider_dsn := :'PROVIDER_DSN',
  replication_sets := ARRAY['rep_all'],
  synchronize_structure := false,
  synchronize_data := true,
  forward_origins := '{}'
);

-- =========================================
-- 验证状态
-- =========================================
-- 运行以下命令检查同步是否正常：
-- SELECT * FROM pglogical.show_subscription_status();
-- 若 status = 'replicating' 表示复制成功。
-- =========================================
