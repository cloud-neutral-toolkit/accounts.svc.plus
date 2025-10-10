APP_NAME := xcontrol-account
MAIN_FILE := ./cmd/accountsvc/main.go
PORT ?= 8080
OS := $(shell uname -s)
SCHEMA_FILE := ./sql/schema.sql
MIGRATION_FILES := $(shell ls -1 sql/migrations/*.up.sql 2>/dev/null | sort)


DB_NAME    := account
DB_USER    := shenlan
DB_PASS    := password
DB_HOST    := 127.0.0.1
DB_PORT    := 5432
DB_URL     := postgres://$(DB_USER):$(DB_PASS)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable

ACCOUNT_EXPORT_FILE ?= account-export.yaml
ACCOUNT_IMPORT_FILE ?= account-export.yaml
ACCOUNT_EMAIL_KEYWORD ?=

SUPERADMIN_USERNAME ?= Admin
SUPERADMIN_PASSWORD ?= ChangeMe
SUPERADMIN_EMAIL    ?= admin@svc.plus

export PATH := /usr/local/go/bin:$(PATH)

.PHONY: all init init-go init-db migrate-db dump-schema build start stop restart clean help dev test create-super-admin account-export account-import

all: build

init: init-go init-db

init-go:
	@echo ">>> 初始化 Go 依赖环境"
	@if command -v go >/dev/null 2>&1; then \
		echo "Go 已安装"; \
	else \
		echo "安装 Go"; \
		if [ "$(OS)" = "Darwin" ]; then \
			brew install go@1.24 && brew link --overwrite --force go@1.24; \
		else \
			sudo apt-get update && sudo apt-get install -y golang; \
		fi; \
	fi
	@if curl -s --max-time 5 https://goproxy.cn >/dev/null; then \
		echo "使用国内镜像: goproxy.cn"; \
		go env -w GOPROXY=https://goproxy.cn,direct; \
	else \
		echo "国内镜像不可用，使用默认: proxy.golang.org"; \
		go env -w GOPROXY=https://proxy.golang.org,direct; \
	fi
	@echo ">>> 执行 go mod tidy"
	go mod tidy

init-db:
	@echo ">>> 初始化数据库 schema"
	@if [ ! -f $(SCHEMA_FILE) ]; then \
	        echo "未找到数据库 schema 文件: $(SCHEMA_FILE)"; \
	        exit 1; \
	fi
	@if ! command -v psql >/dev/null 2>&1; then \
	        echo "未检测到 psql，请先安装 PostgreSQL 客户端"; \
	        exit 1; \
	        fi
	@echo "使用数据库连接: $(DB_URL)"
	@psql "$(DB_URL)" -v ON_ERROR_STOP=1 -f $(SCHEMA_FILE)

# =========================================
# migrate-db target
# =========================================
verify-db:
	@echo ">>> 执行数据库 schema verify  (golang-migrate)"
	@go run ./cmd/migratectl/main.go verify --schema sql/schema.sql --dsn "$(DB_URL)" --dir account/sql/migrations
# =========================================
# migrate-db target
# =========================================
migrate-db:
	@echo ">>> 执行数据库 schema 迁移 (golang-migrate)"
	@echo "--------------------------------------------"
	@echo "使用数据库连接: $(DB_URL)"
	@if ! command -v go >/dev/null 2>&1; then \
	        echo "未检测到 go，请先安装 Go 环境"; \
	        exit 1; \
	fi
	@go run ./cmd/migratectl/main.go migrate --dsn "$(DB_URL)" --dir sql/migrations

account-export:
	@echo ">>> 导出账号数据为 YAML"
	@if [ -z "$(ACCOUNT_EXPORT_FILE)" ]; then \
	        echo "请通过 ACCOUNT_EXPORT_FILE 指定导出文件路径"; \
	        exit 1; \
	fi
	@if ! command -v go >/dev/null 2>&1; then \
	        echo "未检测到 go，请先安装 Go 环境"; \
	        exit 1; \
	fi
	@go run ./cmd/migratectl/main.go export --dsn "$(DB_URL)" --output "$(ACCOUNT_EXPORT_FILE)" $(if $(strip $(ACCOUNT_EMAIL_KEYWORD)),--email "$(ACCOUNT_EMAIL_KEYWORD)")

account-import:
	@echo ">>> 从 YAML 导入账号数据"
	@if [ -z "$(ACCOUNT_IMPORT_FILE)" ]; then \
	        echo "请通过 ACCOUNT_IMPORT_FILE 指定导入文件路径"; \
	        exit 1; \
	fi
	@if ! command -v go >/dev/null 2>&1; then \
	        echo "未检测到 go，请先安装 Go 环境"; \
	        exit 1; \
	fi
	@if [ ! -f "$(ACCOUNT_IMPORT_FILE)" ]; then \
	        echo "未找到导入文件: $(ACCOUNT_IMPORT_FILE)"; \
	        exit 1; \
	fi
	@go run ./cmd/migratectl/main.go import --dsn "$(DB_URL)" --file "$(ACCOUNT_IMPORT_FILE)"

# 删除数据库（安全防呆）
drop-db:
	@echo "⚠️  即将删除数据库 $(DB_NAME) ..."
	@read -p "确定要删除数据库 $(DB_NAME)? [y/N] " confirm && \
	if [ "$$confirm" = "y" ] || [ "$$confirm" = "Y" ]; then \
		echo ">>> 正在删除数据库 $(DB_NAME) ..."; \
		psql -h $(DB_HOST) -U $(DB_USER) -p $(DB_PORT) -d postgres -c "DROP DATABASE IF EXISTS $(DB_NAME);" && \
		echo ">>> 数据库已删除"; \
	else \
		echo "取消删除"; \
	fi

# 重新创建数据库（drop + init）
reinit-db:
	@make drop-db
	@echo ">>> 创建新数据库 $(DB_NAME)"
	psql -h $(DB_HOST) -U $(DB_USER) -p $(DB_PORT) -d postgres -c "CREATE DATABASE $(DB_NAME) OWNER $(DB_USER);"
	@make init-db

dump-schema:
	@echo ">>> 导出数据库 schema 快照"
	@if ! command -v pg_dump >/dev/null 2>&1; then \
	        echo "未检测到 pg_dump，请先安装 PostgreSQL 客户端"; \
	        exit 1; \
	Fi
	@pg_dump -s -O -x "$(DB_URL)" > $(SCHEMA_FILE)

build:
	@echo ">>> 编译 $(APP_NAME)"
	go build -o $(APP_NAME) $(MAIN_FILE)

start: build
	@echo ">>> 运行 postgresql on port 5432 (后台运行)"
	brew services start postgresql
	@echo ">>> 运行 $(APP_NAME) on port $(PORT) (后台运行)"
	@./xcontrol-account --config  config/account.yaml

stop:
	@echo ">>> 停止 $(APP_NAME)"
	@if [ -f $(APP_NAME).pid ]; then \
		kill `cat $(APP_NAME).pid` >/dev/null 2>&1 || true; \
		rm $(APP_NAME).pid; \
	else \
		echo "未找到运行中的进程"; \
	fi

restart: stop start

update: build
	systemctl stop xcontrol-account.service
	cp xcontrol-account /usr/bin/
	systemctl restart xcontrol-account.service

clean:
	@echo ">>> 清理构建产物"
	rm -f $(APP_NAME) $(APP_NAME).log $(APP_NAME).pid

help:
	@echo " XControl Account Service Makefile"
	@echo ""
	@echo "make init       初始化开发环境（包含 Go 依赖与数据库 schema）"
	@echo "make dump-schema 仅导出数据库 schema 快照"
	@echo "make migrate-db  在保留数据的情况下应用 schema 迁移"
	@echo "make account-export [ACCOUNT_EXPORT_FILE=...] [ACCOUNT_EMAIL_KEYWORD=...]  导出账号数据为 YAML"
	@echo "make account-import ACCOUNT_IMPORT_FILE=...  从 YAML 文件导入账号数据"
	@echo "make build      编译 account service 可执行文件"
	@echo "make start      后台运行 account service (默认端口: $(PORT))"
	@echo "make stop       停止运行 account service"
	@echo "make restart    重启 account service"
	@echo "make test       运行账号服务相关单元测试"
	@echo "make dev        开发模式运行 (自动检测 air，如无则用 go run)"
	@echo "make clean      清理构建产物"
	@echo "make create-super-admin SUPERADMIN_USERNAME=... SUPERADMIN_PASSWORD=... [SUPERADMIN_EMAIL=...]  创建超级管理员"

test:
	@echo ">>> 运行单元测试"
	go test ./...

dev:
	@echo ">>> 开发模式运行 $(APP_NAME) (热重载) on port $(PORT)"
	@if command -v air >/dev/null; then \
	        PORT=$(PORT) air -c .air.toml; \
	else \
	        echo "未检测到 air，直接运行 go run"; \
	        PORT=$(PORT) go run $(MAIN_FILE); \
	fi

create-super-admin:
	@if [ -z "$(SUPERADMIN_USERNAME)" ]; then \
	        echo "请通过 SUPERADMIN_USERNAME 指定超级管理员用户名"; \
	        exit 1; \
	fi
	@if [ -z "$(SUPERADMIN_PASSWORD)" ]; then \
	        echo "请通过 SUPERADMIN_PASSWORD 指定超级管理员密码"; \
	        exit 1; \
	fi
	@echo ">>> 创建超级管理员 $(SUPERADMIN_USERNAME)"
	@go run ./cmd/createadmin/main.go \
	        --driver postgres \
	        --dsn "$(DB_URL)" \
	        --username "$(SUPERADMIN_USERNAME)" \
	        --password "$(SUPERADMIN_PASSWORD)" \
	        --email "$(SUPERADMIN_EMAIL)"
