.PHONY: build test run clean lint tidy test-up test-down test-integration

# Build the server binary
build:
	go build -o bin/server ./cmd/server

# Run all tests (unit tests only)
test:
	go test -v ./...

# Run the server
run:
	go run ./cmd/server

# Clean build artifacts
clean:
	rm -rf bin/

# Run linter (requires golangci-lint)
lint:
	golangci-lint run ./...

# Tidy dependencies
tidy:
	go mod tidy

# Start test database container
test-up:
	docker-compose -f docker-compose.test.yml up -d
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 3
	@until docker exec pocket_money_test_db pg_isready -U test -d pocket_money_test; do \
		echo "Waiting for database..."; \
		sleep 2; \
	done
	@echo "Database is ready!"

# Stop test database container
test-down:
	docker-compose -f docker-compose.test.yml down -v

# Run integration tests (starts DB, runs tests, stops DB)
test-integration: test-up
	go test -v -tags=integration ./... || (make test-down && exit 1)
	make test-down
