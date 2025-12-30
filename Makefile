.PHONY: up down migrate migrate-down test run build clean help

# Default target
help:
	@echo "Aurum Finance Workspace - Available targets:"
	@echo ""
	@echo "  up           Start infrastructure (Postgres, Kafka)"
	@echo "  down         Stop infrastructure"
	@echo "  migrate      Apply database migrations"
	@echo "  migrate-down Rollback last migration"
	@echo "  test         Run all tests"
	@echo "  run          Run the main service"
	@echo "  build        Build all binaries"
	@echo "  clean        Remove build artifacts"

# Infrastructure
up:
	docker compose up -d

down:
	docker compose down

# Database
migrate:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

migrate-version:
	go run ./cmd/migrate version

# Development
run:
	go run ./cmd/aurum

# Testing
test:
	go test ./...

test-verbose:
	go test -v ./...

# Build
build:
	go build -o bin/aurum ./cmd/aurum
	go build -o bin/migrate ./cmd/migrate

clean:
	rm -rf bin/

# Dependencies
deps:
	go mod tidy
	go mod download
