.PHONY: up down migrate migrate-down test run build clean help e2e e2e-up e2e-down

# Default target
help:
	@echo "Aurum Finance Workspace - Available targets:"
	@echo ""
	@echo "  up           Start infrastructure (Postgres, Kafka)"
	@echo "  down         Stop infrastructure"
	@echo "  migrate      Apply database migrations"
	@echo "  migrate-down Rollback last migration"
	@echo "  test         Run all tests"
	@echo "  e2e          Run e2e tests (app in Docker)"
	@echo "  e2e-up       Start e2e environment"
	@echo "  e2e-down     Stop e2e environment"
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

# E2E Testing
e2e-up:
	docker compose -f docker-compose.e2e.yml up -d --build
	@echo "Waiting for services to be healthy..."
	@until docker compose -f docker-compose.e2e.yml exec -T aurum wget -q --spider http://localhost:8080/health 2>/dev/null; do \
		sleep 1; \
	done
	@echo "Services are ready!"

e2e-down:
	docker compose -f docker-compose.e2e.yml down -v

e2e: e2e-up
	@echo "Running e2e tests..."
	AURUM_BASE_URL=http://localhost:8080 go test -v -tags=e2e ./features/...
	@$(MAKE) e2e-down
