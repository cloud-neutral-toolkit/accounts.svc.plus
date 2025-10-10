-- =========================================
-- schema_pglogical_region_cn.sql
-- pglogical configuration for CN node
-- PostgreSQL 16+, 双向复制 (provider + subscriber)
-- =========================================

-- 🧭 清理旧节点（可安全重入）
DO $$
BEGIN
  PERFORM pglogical.drop_subscription('sub_from_global', true);
  EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
  PERFORM pglogical.drop_node('node_cn');
  EXCEPTION WHEN others THEN NULL;
END $$;

-- =========================================
-- 创建本节点 (Provider)
-- =========================================
SELECT pglogical.create_node(
  node_name := 'node_cn',
  dsn := 'host=cn-homepage.svc.plus port=5432 dbname=account user=pglogical password=xxxx'
);

-- =========================================
-- 定义复制集
-- =========================================
SELECT pglogical.create_replication_set('rep_all');
SELECT pglogical.replication_set_add_all_tables('rep_all', ARRAY['public']);

-- =========================================
-- 创建订阅 (订阅 Global 节点)
-- =========================================
SELECT pglogical.create_subscription(
  subscription_name := 'sub_from_global',
  provider_dsn := 'host=global-homepage.svc.plus port=5432 dbname=account user=pglogical password=xxx',
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
