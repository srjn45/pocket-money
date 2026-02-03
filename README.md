# Pocket Money

[![CI](https://github.com/srjn45/pocket-money/actions/workflows/ci.yml/badge.svg)](https://github.com/srjn45/pocket-money/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/srjn45/pocket-money/branch/main/graph/badge.svg)](https://codecov.io/gh/srjn45/pocket-money)
[![Go Version](https://img.shields.io/badge/Go-1.24-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/License-PolyForm%20NC-green.svg)](LICENSE)

A family-oriented chore tracking and pocket money management app.

## Overview

Pocket Money helps families track chores completed by children and manage their earnings. Parents (heads) can:
- Create groups for their family
- Define chores with monetary values
- Review and approve completed chores
- Record cash payouts (settlements)

Children (members) can:
- Log completed chores
- View their earnings balance
- Track their settlement history

## Project Structure

```
pocket-money/
├── backend/               # Go API server
│   ├── cmd/server/        # Main entry point
│   ├── internal/          # Application code
│   ├── migrations/        # Database migrations
│   ├── Dockerfile         # Container build
│   └── Makefile           # Build & dev commands
├── app/                   # React Native (Expo) app
│   ├── app/               # Screens and routes
│   └── src/               # Shared code
├── docker-compose.yml     # Development environment
└── .github/workflows/     # CI/CD pipelines
```

## Tech Stack

- **Backend**: Go 1.24, Gin, PostgreSQL 15, golang-migrate
- **Mobile**: React Native, Expo, TypeScript, expo-router
- **Auth**: JWT bearer tokens
- **CI/CD**: GitHub Actions, Docker

---

## Quick Start

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and Docker Compose
- [Go 1.24+](https://golang.org/dl/) (for local development)
- [Node.js 18+](https://nodejs.org/) (for mobile app)
- [golangci-lint](https://golangci-lint.run/usage/install/) (for linting)

### Option 1: Docker Compose (Recommended)

Start the entire stack with one command:

```bash
# Start database and backend
docker compose up -d

# View logs
docker compose logs -f backend

# Stop everything
docker compose down
```

The API will be available at `http://localhost:8080`.

### Option 2: Local Development

```bash
cd backend

# Start only the database
make dev-up

# Run the server (with hot reload via go run)
make dev

# Stop the database when done
make dev-down
```

---

## Development Guide

### Backend Commands

All commands should be run from the `backend/` directory:

```bash
cd backend

# Show all available commands
make help
```

#### Building

```bash
make build          # Build binary to bin/server
make docker-build   # Build Docker image
```

#### Running

```bash
make run            # Run server (requires env vars set)
make dev            # Start DB + run server with dev config
make dev-full       # Start entire stack via Docker Compose
```

#### Testing

```bash
make test           # Run unit tests only
make test-all       # Run all tests (starts DB, runs tests, stops DB)
make test-coverage  # Run tests with coverage report
make test-integration  # Run only integration tests
```

#### Code Quality

```bash
make lint           # Run linter
make lint-fix       # Run linter with auto-fix
make fmt            # Format code
make vet            # Run go vet
```

#### Cleanup

```bash
make clean          # Remove build artifacts
make clean-all      # Remove artifacts + stop all containers
```

---

## Environment Variables

### Backend

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | Yes | - | PostgreSQL connection string |
| `JWT_SECRET` | Yes | - | Secret key for JWT signing (min 32 chars recommended) |
| `PORT` | No | `8080` | Server port |
| `CORS_ORIGINS` | No | `*` | Comma-separated allowed origins |

#### Example `.env` file

```bash
DATABASE_URL=postgres://pocket:pocket@localhost:5432/pocket_money?sslmode=disable
JWT_SECRET=your-super-secret-key-at-least-32-characters
PORT=8080
CORS_ORIGINS=http://localhost:3000,http://localhost:8081
```

### Mobile App

| Variable | Description |
|----------|-------------|
| `EXPO_PUBLIC_API_URL` | Backend API URL (e.g., `http://192.168.1.x:8080/api/v1`) |

---

## Testing

### Unit Tests

Unit tests don't require a database and test individual functions:

```bash
cd backend
make test
```

### Integration Tests

Integration tests require a PostgreSQL database and test the full stack:

```bash
cd backend

# Automatic: starts DB, runs tests, stops DB
make test-all

# Manual control
make test-up        # Start test database
make test-integration
make test-down      # Stop test database
```

### Coverage Report

```bash
cd backend
make test-coverage

# View HTML report
open coverage.html  # macOS
xdg-open coverage.html  # Linux
```

---

## CI/CD Pipeline

The project uses GitHub Actions for continuous integration. Every push triggers:

1. **Lint**: Runs `golangci-lint` to check code quality
2. **Test**: Runs unit and integration tests with coverage
3. **Build**: Builds binary and Docker image

### Pipeline Status

Check the [Actions tab](https://github.com/srjn45/pocket-money/actions) for build status.

### Required Secrets

For Codecov integration, add these secrets to your GitHub repository:
- `CODECOV_TOKEN`: Your Codecov upload token

---

## API Documentation

### Health Check

```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

### Authentication

```bash
# Register
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"secret123","name":"John"}'

# Login
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"secret123"}'
# Returns: {"token":"eyJ...","user":{...}}

# Use token for authenticated requests
curl http://localhost:8080/api/v1/auth/me \
  -H "Authorization: Bearer eyJ..."
```

See [backend/README.md](backend/README.md) for complete API documentation.

---

## Mobile App

```bash
cd app

# Install dependencies
npm install

# Set API URL
export EXPO_PUBLIC_API_URL="http://127.0.0.1:8080/api/v1"

# Start development server
npm start

# Run on specific platform
npm run ios      # iOS simulator
npm run android  # Android emulator
npm run web      # Web browser
```

---

## Database Migrations

Migrations run automatically when the server starts. To run manually:

```bash
cd backend

# Migrations are in backend/migrations/
ls migrations/

# The server runs migrations on startup via:
# db.RunMigrations(cfg.DatabaseURL)
```

---

## Troubleshooting

### Database Connection Issues

```bash
# Check if PostgreSQL is running
docker ps | grep postgres

# Check database logs
docker logs pocket_money_db

# Verify connection string
psql "postgres://pocket:pocket@localhost:5432/pocket_money"
```

### Port Already in Use

```bash
# Find process using port 8080
lsof -i :8080

# Kill it or use a different port
PORT=9090 make dev
```

### Docker Build Fails

```bash
# Clean Docker cache
docker builder prune

# Rebuild without cache
docker compose build --no-cache
```

---

## License

This project uses dual licensing:
- **Non-commercial use**: [PolyForm Noncommercial License 1.0.0](LICENSE)
- **Commercial use**: Contact the copyright holder for a commercial license

See [COMMERCIAL_LICENSE.md](COMMERCIAL_LICENSE.md) for details.
