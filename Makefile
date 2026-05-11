SHELL := /bin/bash

APP_NAME := fitlog
PKG := ./...
MIGRATIONS_DIR := ./migrations

# Loaded from .env when present; values used by migrate-up.
ifneq (,$(wildcard .env))
include .env
export
endif

.PHONY: help build run test lint fmt tidy migrate-up migrate-down migrate-status docker-up docker-down docker-logs clean

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

build: ## Build the fitlog binary
	go build -o bin/$(APP_NAME) ./cmd/fitlog

run: ## Run the server locally (loads .env)
	go run ./cmd/fitlog server

test: ## Run unit tests
	go test -race -count=1 $(PKG)

lint: ## Run golangci-lint
	golangci-lint run

fmt: ## go fmt
	go fmt $(PKG)

tidy: ## go mod tidy
	go mod tidy

migrate-up: ## Apply migrations
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" up

migrate-down: ## Roll back one migration
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" down

migrate-status: ## Show migration status
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" status

docker-up: ## Start postgres (and app) via docker compose
	docker compose -f deployments/docker-compose.yml up -d

docker-down: ## Stop docker compose stack
	docker compose -f deployments/docker-compose.yml down

docker-logs: ## Tail app logs
	docker compose -f deployments/docker-compose.yml logs -f app

clean: ## Remove build artifacts
	rm -rf bin/ dist/
