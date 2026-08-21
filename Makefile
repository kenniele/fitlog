SHELL := /bin/bash

APP_NAME := fitlog
# Keep Go discovery out of the separately managed Node dependency tree. Some
# npm packages contain incidental .go files that are not part of FitLog.
PKG := ./cmd/... ./internal/... ./migrations/...
MIGRATIONS_DIR := ./migrations

# Loaded from .env when present; values used by migrate-up.
ifneq (,$(wildcard .env))
include .env
export
endif

.PHONY: help build build-all run test test-all lint fmt tidy migrate-up migrate-down migrate-status demo-seed web-install web-dev web-test web-lint web-build docker-up docker-down docker-logs clean deploy deploy-logs

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

build: ## Build the fitlog binary
	go build -o bin/$(APP_NAME) ./cmd/fitlog

build-all: build web-build ## Build the Go service and Control Center

run: ## Run the server locally (loads .env)
	go run ./cmd/fitlog server

test: ## Run unit tests
	go test -race -count=1 $(PKG)

test-all: test web-test ## Run backend and frontend tests

lint: ## Run golangci-lint
	golangci-lint run

fmt: ## go fmt
	go fmt $(PKG)

tidy: ## go mod tidy
	go mod tidy

migrate-up: ## Apply migrations
	CGO_ENABLED=0 go run ./cmd/fitlog migrate

migrate-down: ## Roll back one migration
	go run github.com/pressly/goose/v3/cmd/goose@v3.27.1 -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" down

migrate-status: ## Show migration status
	go run github.com/pressly/goose/v3/cmd/goose@v3.27.1 -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" status

demo-seed: ## Insert missing rows from the deterministic 90-day Control Center demo
	go run ./cmd/fitlog demo-seed

web-install: ## Install the locked Control Center dependencies
	npm --prefix web ci

web-dev: ## Run the Control Center development server
	npm --prefix web run dev

web-test: ## Run Control Center tests
	npm --prefix web run test

web-lint: ## Lint the Control Center
	npm --prefix web run lint

web-build: ## Build the production Control Center bundle
	npm --prefix web run build

docker-up: ## Start local PostgreSQL for host development
	docker compose -f deployments/docker-compose.yml up -d --wait --wait-timeout 30 postgres

docker-down: ## Stop docker compose stack
	docker compose -f deployments/docker-compose.yml down

docker-logs: ## Tail logs from the local compose stack
	docker compose -f deployments/docker-compose.yml logs -f

clean: ## Remove build artifacts
	rm -rf bin/ dist/ web/.next/ web/coverage/

# Deploy: `make deploy DEPLOY_HOST=fitlog@1.2.3.4`
deploy: ## Pull images, migrate, and start the production stack
	@if [ -z "$(DEPLOY_HOST)" ]; then echo "set DEPLOY_HOST=user@ip"; exit 1; fi
	ssh $(DEPLOY_HOST) 'set -euo pipefail; cd ~/fitlog; git pull --ff-only; COMPOSE="docker compose --env-file $$PWD/.env -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml"; $$COMPOSE pull; $$COMPOSE up -d --wait --wait-timeout 30 postgres; $$COMPOSE run --rm --no-deps -T app migrate </dev/null; $$COMPOSE up -d --force-recreate app web caddy; $$COMPOSE logs --tail=20 app web caddy'

deploy-logs: ## Tail app logs on the remote host
	@if [ -z "$(DEPLOY_HOST)" ]; then echo "set DEPLOY_HOST=user@ip"; exit 1; fi
	ssh $(DEPLOY_HOST) 'cd ~/fitlog && docker compose --env-file $$PWD/.env -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml logs -f --tail=100 app web caddy'
