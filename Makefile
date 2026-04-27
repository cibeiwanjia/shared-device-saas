# ============================================
# Shared Device SaaS - Makefile
# ============================================

# 数据库配置（可通过环境变量覆盖）
DB_HOST     ?= 115.190.17.186
DB_PORT     ?= 3306
DB_USER     ?= root
DB_PASS     ?= 4ay1nkal3u8ed77y
DB_NAME     ?= shared_device_user

# golang-migrate DSN 格式
MIGRATE_DSN = mysql://$(DB_USER):$(DB_PASS)@tcp($(DB_HOST):$(DB_PORT))/$(DB_NAME)

# 迁移文件目录
MIGRATIONS_DIR = migrations

# Go 编译目标
APP_NAME     = shared-device-saas
USER_SERVICE = app/user/cmd/user

# ============================================
# 数据库迁移命令
# ============================================

# 安装 golang-migrate CLI（macOS）
.PHONY: migrate-install
migrate-install:
	@which migrate > /dev/null 2>&1 && echo "migrate already installed: $$(migrate -version)" || \
		brew install golang-migrate

# 执行所有未运行的迁移（升级）
.PHONY: migrate-up
migrate-up:
	@migrate -path $(MIGRATIONS_DIR) -database "$(MIGRATE_DSN)" up

# 回滚最近 N 次迁移（默认回滚 1 次）
.PHONY: migrate-down
migrate-down:
	@migrate -path $(MIGRATIONS_DIR) -database "$(MIGRATE_DSN)" down $(if $(N),$(N),1)

# 查看当前迁移版本
.PHONY: migrate-version
migrate-version:
	@migrate -path $(MIGRATIONS_DIR) -database "$(MIGRATE_DSN)" version

# 强制设置迁移版本号（修复脏状态时用）
.PHONY: migrate-force
migrate-force:
	@if [ -z "$(V)" ]; then echo "Usage: make migrate-force V=000003"; exit 1; fi
	@migrate -path $(MIGRATIONS_DIR) -database "$(MIGRATE_DSN)" force $(V)

# 创建新的迁移文件（用法: make migrate-create NAME=add_xxx）
.PHONY: migrate-create
migrate-create:
	@if [ -z "$(NAME)" ]; then echo "Usage: make migrate-create NAME=add_xxx"; exit 1; fi
	@migrate create -ext sql -dir $(MIGRATIONS_DIR) -seq $(NAME)

# 回滚到指定版本再升级（用法: make migrate-rebase V=000003）
.PHONY: migrate-rebase
migrate-rebase:
	@if [ -z "$(V)" ]; then echo "Usage: make migrate-rebase V=000003"; exit 1; fi
	@migrate -path $(MIGRATIONS_DIR) -database "$(MIGRATE_DSN)" down $(shell echo $$(($$(migrate -path $(MIGRATIONS_DIR) -database "$(MIGRATE_DSN)" version 2>/dev/null | grep -o '[0-9]*') - $(V) + 1)))
	@migrate -path $(MIGRATIONS_DIR) -database "$(MIGRATE_DSN)" up

# ============================================
# Go 编译 & 运行
# ============================================

# 编译 user 服务
.PHONY: build
build:
	@go build -o bin/$(APP_NAME) ./$(USER_SERVICE)

# 编译所有服务
.PHONY: build-all
build-all:
	@go build ./...

# 运行 user 服务
.PHONY: run
run:
	@go run ./$(USER_SERVICE)

# 代码检查
.PHONY: vet
vet:
	@go vet ./...

# Wire 生成依赖注入代码（user 服务）
.PHONY: wire
wire:
	@cd app/user/cmd/user && wire

# 生成 proto 代码
.PHONY: proto
proto:
	@cd api && buf generate

# ============================================
# 开发辅助
# ============================================

# 一键初始化开发环境：迁移 + 编译
.PHONY: dev-setup
dev-setup: migrate-up build
	@echo "Dev environment ready! Run 'make run' to start."

# 清理编译产物
.PHONY: clean
clean:
	@rm -rf bin/

# 帮助信息
.PHONY: help
help:
	@echo "Shared Device SaaS - Available Commands"
	@echo ""
	@echo "Database Migration:"
	@echo "  make migrate-install     - Install golang-migrate CLI"
	@echo "  make migrate-up          - Run all pending migrations"
	@echo "  make migrate-down        - Rollback last migration (use N=3 for 3 steps)"
	@echo "  make migrate-version     - Show current migration version"
	@echo "  make migrate-force V=3   - Force set version (fix dirty state)"
	@echo "  make migrate-create NAME=add_xxx  - Create new migration files"
	@echo ""
	@echo "Build & Run:"
	@echo "  make build               - Build user service"
	@echo "  make build-all           - Build all services"
	@echo "  make run                 - Run user service"
	@echo "  make vet                 - Run go vet"
	@echo "  make wire                - Regenerate wire DI"
	@echo "  make proto               - Generate proto code"
	@echo ""
	@echo "Dev Workflow:"
	@echo "  make dev-setup           - Migrate + build (first-time setup)"
	@echo "  make clean               - Remove build artifacts"
