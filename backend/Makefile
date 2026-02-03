.PHONY: build test run clean lint tidy test-up test-down test-all test-coverage \
        docker-build docker-run dev dev-up dev-down help

# Default target
.DEFAULT_GOAL := help

# ============================================================================
# HELP
# ============================================================================

help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*##"; printf ""} /^[a-zA-Z_-]+:.*?##/ { printf "  %-20s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# ============================================================================
# BUILD
# ============================================================================

build: ## Build the server binary
	go build -o bin/server ./cmd/server

docker-build: ## Build Docker image
	docker build -t pocket-money-backend:latest .

docker-run: docker-build ## Build and run Docker image
	docker run -p 8080:8080 \
		-e DATABASE_URL="postgres://pocket:pocket@host.docker.internal:5432/pocket_money?sslmode=disable" \
		-e JWT_SECRET="dev-secret" \
		pocket-money-backend:latest

# ============================================================================
# DEVELOPMENT
# ============================================================================

run: ## Run the server locally (requires env vars)
	go run ./cmd/server

dev-up: ## Start development database
	docker compose -f ../docker-compose.yml up -d postgres
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 2
	@until docker exec pocket_money_db pg_isready -U pocket -d pocket_money > /dev/null 2>&1; do \
		echo "Waiting for database..."; \
		sleep 2; \
	done
	@echo "Database is ready!"

dev-down: ## Stop development database
	docker compose -f ../docker-compose.yml down

dev: dev-up ## Start development environment (DB + server)
	@echo "Starting server..."
	DATABASE_URL="postgres://pocket:pocket@localhost:5432/pocket_money?sslmode=disable" \
	JWT_SECRET="dev-secret-key" \
	PORT="8080" \
	CORS_ORIGINS="*" \
	go run ./cmd/server

dev-full: ## Start full stack with Docker Compose
	docker compose -f ../docker-compose.yml up --build

dev-full-down: ## Stop full stack
	docker compose -f ../docker-compose.yml down

# ============================================================================
# TESTING
# ============================================================================

test: ## Run unit tests
	go test -v ./...

test-up: ## Start test database
	docker compose -f docker-compose.test.yml up -d
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 2
	@until docker exec pocket_money_test_db pg_isready -U test -d pocket_money_test > /dev/null 2>&1; do \
		echo "Waiting for database..."; \
		sleep 2; \
	done
	@echo "Test database is ready!"

test-down: ## Stop test database
	docker compose -f docker-compose.test.yml down -v

test-all: test-up ## Run all tests (unit + integration) with setup/teardown
	@echo "Running all tests..."
	@go test -v -p 1 -tags=integration ./... ; \
	status=$$? ; \
	echo "Stopping test database..." ; \
	$(MAKE) test-down ; \
	exit $$status

test-coverage: ## Run tests with coverage report
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@echo ""
	@echo "=== Coverage Summary ==="
	go tool cover -func=coverage.out
	@echo ""
	@echo "Generating HTML report: coverage.html"
	go tool cover -html=coverage.out -o coverage.html

test-integration: test-up ## Run only integration tests
	@go test -v -p 1 -tags=integration ./... ; \
	status=$$? ; \
	$(MAKE) test-down ; \
	exit $$status

# ============================================================================
# CODE QUALITY
# ============================================================================

lint: ## Run linter
	golangci-lint run ./...

lint-fix: ## Run linter with auto-fix
	golangci-lint run --fix ./...

fmt: ## Format code
	go fmt ./...
	goimports -w .

vet: ## Run go vet
	go vet ./...

# ============================================================================
# DEPENDENCIES
# ============================================================================

tidy: ## Tidy and verify dependencies
	go mod tidy
	go mod verify

deps: ## Download dependencies
	go mod download

# ============================================================================
# CLEANUP
# ============================================================================

clean: ## Clean build artifacts and coverage files
	rm -rf bin/
	rm -f coverage.out coverage.html

clean-all: clean dev-down test-down ## Clean everything including Docker containers
	@echo "Cleaned all artifacts and stopped containers"
