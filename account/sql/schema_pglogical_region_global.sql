-- =========================================
-- schema_pglogical_region_global.sql
-- pglogical configuration for GLOBAL node
-- PostgreSQL 16+, 双向复制 (provider + subscriber)
-- =========================================

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
  PERFORM pglogical.drop_subscription('sub_from_cn', true);
  EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
  PERFORM pglogical.drop_node('node_global');
  EXCEPTION WHEN others THEN NULL;
END $$;

-- =========================================
-- 创建本节点 (Provider)
-- =========================================
SELECT pglogical.create_node(
  node_name := 'node_global',
  dsn := 'host=global-homepage.svc.plus port=5432 dbname=account user=pglogical password=xxxx'
);

-- =========================================
-- 定义复制集
-- =========================================
SELECT pglogical.create_replication_set('rep_all');
SELECT pglogical.replication_set_add_all_tables('rep_all', ARRAY['public']);

-- =========================================
-- 创建订阅 (订阅 CN 节点)
-- =========================================
SELECT pglogical.create_subscription(
  subscription_name := 'sub_from_cn',
  provider_dsn := 'host=cn-homepage.svc.plus port=5432 dbname=account user=pglogical password=xxxx',
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
