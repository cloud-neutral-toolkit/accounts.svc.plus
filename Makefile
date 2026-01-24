# =========================================
# 📦 XControl Account Service Makefile
# =========================================

APP_NAME    := xcontrol-account
MAIN_FILE   := ./cmd/accountsvc/main.go
PORT        ?= 8080
OS          := $(shell uname -s)

DB_NAME     := account
DB_USER     := shenlan
DB_PASS     := password
DB_HOST     := 127.0.0.1
DB_PORT     := 5432
DB_URL      := postgres://$(DB_USER):$(DB_PASS)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable

REPLICATION_MODE ?= pgsync

DB_ADMIN_USER ?= $(DB_USER)
DB_ADMIN_PASS ?= $(DB_PASS)

GCP_PROJECT ?=
GCP_REGION ?= asia-northeast1
CLOUD_RUN_SERVICE ?= accounts-svc-plus
CLOUD_RUN_SERVICE_YAML ?= deploy/gcp/cloud-run/service.yaml
CLOUD_RUN_STUNNEL_CONF ?= deploy/gcp/cloud-run/stunnel.conf
CLOUD_RUN_IMAGE ?= $(GCP_REGION)-docker.pkg.dev/$(GCP_PROJECT)/cloud-run-source-deploy/accounts.svc.plus/accounts-svc-plus:latest

SCHEMA_FILE := ./sql/schema.sql
PGLOGICAL_INIT_FILE := ./sql/schema_pglogical_init.sql
PGLOGICAL_PATCH_FILE := ./sql/schema_pglogical_patch.sql
PGLOGICAL_REGION_FILE := ./sql/schema_pglogical_region.sql

ACCOUNT_EXPORT_FILE ?= account-export.yaml
ACCOUNT_IMPORT_FILE ?= account-export.yaml
ACCOUNT_EMAIL_KEYWORD ?=
ACCOUNT_SYNC_CONFIG ?= config/sync.yaml
SUPERADMIN_USERNAME ?= Admin
SUPERADMIN_PASSWORD ?= ChangeMe
SUPERADMIN_EMAIL    ?= admin@svc.plus

export PATH := /usr/local/go/bin:$(PATH)
export APP_NAME MAIN_FILE PORT OS \
	DB_NAME DB_USER DB_PASS DB_HOST DB_PORT DB_URL \
	REPLICATION_MODE DB_ADMIN_USER DB_ADMIN_PASS \
	GCP_PROJECT GCP_REGION CLOUD_RUN_SERVICE CLOUD_RUN_SERVICE_YAML CLOUD_RUN_STUNNEL_CONF CLOUD_RUN_IMAGE \
	SCHEMA_FILE PGLOGICAL_INIT_FILE PGLOGICAL_PATCH_FILE PGLOGICAL_REGION_FILE \
	ACCOUNT_EXPORT_FILE ACCOUNT_IMPORT_FILE ACCOUNT_EMAIL_KEYWORD ACCOUNT_SYNC_CONFIG \
	SUPERADMIN_USERNAME SUPERADMIN_PASSWORD SUPERADMIN_EMAIL \
	ACCOUNT_IMPORT_MERGE ACCOUNT_IMPORT_MERGE_STRATEGY ACCOUNT_IMPORT_DRY_RUN \
	ACCOUNT_IMPORT_MERGE_ALLOWLIST ACCOUNT_IMPORT_EXTRA_FLAGS

# =========================================
# 🧩 基础命令
# =========================================

.PHONY: all init build clean start stop restart dev test help \
	init-go init-db init-db-core init-db-replication init-db-pglogical \
	reinit-pglogical account-sync-push account-sync-pull account-sync-mirror create-db-user db-reset \
	cloudrun-build cloudrun-deploy cloudrun-stunnel

all: build

help:
	@echo "🧭 XControl Account Service Makefile"
	@echo "make init               初始化 Go 环境与数据库"
	@echo "make init-db            执行数据库 schema（支持 REPLICATION_MODE=pgsync|pglogical）"
	@echo "make create-db-user     创建数据库用户并授权"
	@echo "make db-reset           重置整个 PostgreSQL 集群 (危险操作!)"
	@echo "make migrate-db         执行数据库迁移"
	@echo "make dump-schema        导出数据库 schema"
	@echo "make account-export     导出账号数据为 YAML"
	@echo "make account-import     从 YAML 导入账号数据"
	@echo "make create-super-admin 创建超级管理员"
	@echo "make reinit-db          重置业务 schema (不涉及 pglogical)"
	@echo "make reinit-pglogical   重新初始化 pglogical schema"
	@echo "make dev                热重载开发模式"
	@echo "make clean              清理构建产物"
	@echo "make cloudrun-build     构建并推送 Cloud Run 镜像"
	@echo "make cloudrun-deploy    部署 Cloud Run Service"
	@echo "make cloudrun-stunnel   更新 Cloud Run stunnel 配置 secret"

# =========================================
# 🧰 初始化
# =========================================

init: init-go init-db

init-go:
	@bash scripts/init-go.sh

init-db:
	@bash scripts/init-db.sh

init-db-core:
	@bash scripts/init-db-core.sh

init-db-replication:
	@bash scripts/init-db-replication.sh

init-db-pglogical:
	@bash scripts/init-db-pglogical.sh

# =========================================
# 🧠 PGLogical 双节点初始化
# =========================================

init-pglogical-region:
	@bash scripts/init-pglogical-region.sh

init-pglogical-region-cn:
	@bash scripts/init-pglogical-region-cn.sh

init-pglogical-region-global:
	@bash scripts/init-pglogical-region-global.sh

# =========================================
# 📦 数据库迁移与管理
# =========================================

create-db-user:
	@bash scripts/create-db-user.sh

migrate-db:
	@bash scripts/migrate-db.sh

dump-schema:
	@bash scripts/dump-schema.sh

db-reset:
	@bash scripts/db-reset.sh

drop-db:
	@bash scripts/drop-db.sh

reset-public-schema:
	@bash scripts/reset-public-schema.sh

reinit-db:
	@bash scripts/reinit-db.sh

reinit-pglogical:
	@bash scripts/reinit-pglogical.sh

# =========================================
# 💾 账号导入导出
# =========================================

account-export:
	@bash scripts/account-export.sh

account-import:
	@bash scripts/account-import.sh

account-sync-push:
	@bash scripts/account-sync-push.sh

account-sync-pull:
	@bash scripts/account-sync-pull.sh

account-sync-mirror:
	@bash scripts/account-sync-mirror.sh

create-super-admin:
	@bash scripts/create-super-admin.sh

# =========================================
# ⚙️ 编译与运行
# =========================================

build: init-go
	@bash scripts/build.sh

upgrade: build
	@bash scripts/upgrade.sh

start: build
	@bash scripts/start.sh

stop:
	@bash scripts/stop.sh

restart: stop start

test:
	@bash scripts/test.sh

clean:
	@bash scripts/clean.sh

dev:
	@bash scripts/dev.sh

# =========================================
# ☁️ GCP Cloud Run
# =========================================

cloudrun-build:
	@bash scripts/cloudrun-build.sh

cloudrun-deploy:
	@bash scripts/cloudrun-deploy.sh

cloudrun-stunnel:
	@bash scripts/cloudrun-stunnel.sh
