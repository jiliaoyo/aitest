PG_URL ?= postgres://$(USER)@localhost:5432/ai_shuati_dev?sslmode=disable
export DATABASE_URL = $(PG_URL)

.PHONY: help migrate seed server worker dev test test-integration build tidy

help:
	@echo "migrate  - 应用数据库迁移"
	@echo "seed     - 写入演示数据"
	@echo "server   - 启动 API（内嵌 worker：RUN_WORKER=true make server）"
	@echo "worker   - 独立 worker 进程"
	@echo "dev      - 迁移 + seed + 启动 API（含内嵌 worker）"
	@echo "test     - 运行 Go 测试"
	@echo "test-integration - 在 TEST_DATABASE_URL 指定的专用库运行集成测试"

migrate:
	go run ./cmd/migrate -url "$(PG_URL)" -dir migrations

seed:
	DATABASE_URL="$(PG_URL)" go run ./cmd/seed

server:
	DATABASE_URL="$(PG_URL)" go run ./cmd/server

worker:
	DATABASE_URL="$(PG_URL)" WORKER_ID=worker-standalone go run ./cmd/worker

dev:
	DATABASE_URL="$(PG_URL)" RUN_WORKER=true WORKER_ID=dev-worker go run ./cmd/server

test:
	go test ./...

test-integration:
	@test -n "$(TEST_DATABASE_URL)" || (echo "请设置 TEST_DATABASE_URL 为专用测试数据库"; exit 1)
	go run ./cmd/migrate -url "$(TEST_DATABASE_URL)" -dir migrations
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -tags=integration ./...

build:
	go build ./...

tidy:
	go mod tidy
