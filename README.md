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
├── backend/               # Go API server (also embeds the web SPA + migrations)
│   ├── cmd/server/        # Main entry point
│   ├── internal/          # Application code
│   │   └── web/           # Embedded Expo web export (go:embed) + SPA fallback
│   ├── migrations/        # Database migrations (embedded via iofs)
│   └── Makefile           # Build & dev commands
├── app/                   # React Native (Expo) app
│   ├── app/               # Screens and routes
│   └── src/               # Shared code
├── Dockerfile             # Multi-stage single-image build (web → go → alpine)
├── docker-compose.yml     # postgres + app (the single image)
└── .github/workflows/     # CI/CD pipelines
```

## Tech Stack

- **Backend**: Go 1.24, Gin, PostgreSQL 15, golang-migrate
- **Mobile**: React Native, Expo, TypeScript, expo-router
- **Auth**: JWT bearer tokens
- **CI/CD**: GitHub Actions, Docker

---

## Quick Start

The whole product — the web app **and** the API — ships as **one image**. You need
only Docker and a single secret.

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and Docker Compose

That's it. (Go and Node are only needed for [local development](#local-development).)

### Run it

```bash
git clone https://github.com/srjn45/pocket-money.git && cd pocket-money
export JWT_SECRET=$(openssl rand -hex 32)
docker compose up --build
```

Then open **http://localhost:8080**, register an account, and create a group.

- The **web app** is served at `http://localhost:8080`.
- The **API** is same-origin at `http://localhost:8080/api/v1`.
- **Health** check at `http://localhost:8080/health`.

How it works:

- **One image serves the web app + API.** The Expo web bundle is `go:embed`-ed into
  the Go server (with an SPA fallback), so there is no separate frontend server and
  no CORS to configure — the web app is same-origin with the API.
- **Migrations run automatically on boot** (embedded in the binary), so a fresh
  Postgres is set up on first start.
- **`JWT_SECRET` is the only required env var.** `docker compose up` fails fast with a
  clear message if it is unset. No base-URL config is needed — invite links default
  to the request host.

To stop and remove the volume:

```bash
docker compose down -v
```

---

## Local Development

For iterating on the backend or the app directly (requires
[Go 1.24+](https://golang.org/dl/), [Node.js 22+](https://nodejs.org/), and
[golangci-lint](https://golangci-lint.run/usage/install/)):

```bash
cd backend

# Start only the database
make dev-up

# Run the server (with hot reload via go run)
make dev

# Stop the database when done
make dev-down
```

Run the Expo app against the local backend from `app/` (`npm start`), pointing it at
the API with `EXPO_PUBLIC_API_URL` (e.g. `http://localhost:8080/api/v1`).

---

## Native App (Android APK)

Producing an installable Android APK is an **operator step**: it needs an **Expo
account** (`npx expo login`). This repo ships the build config (`app/eas.json`) and this
guide — **not** a pre-built binary. Follow the steps below to produce one.

> **⚠️ The API URL is baked into the binary at build time — it cannot be changed after
> install.** A native build inlines `EXPO_PUBLIC_API_URL` into the JS bundle inside the
> APK. Before you build, point it at **your** server (the same host the single image from
> [Quick Start](#quick-start) serves; the API lives at `…/api/v1`). Two ways:
>
> - Edit `app/eas.json` → `build.preview.env.EXPO_PUBLIC_API_URL` to your server
>   (e.g. `https://money.example.com/api/v1`), **or**
> - store it as an EAS environment variable so it isn't committed:
>   `eas env:create --name EXPO_PUBLIC_API_URL --value https://money.example.com/api/v1`.
>
> The value committed in `eas.json` is a **placeholder** (`https://your-server.example.com/api/v1`);
> leaving it unchanged ships an APK that talks to nothing. EAS cloud builds do **not** read
> your shell's `EXPO_PUBLIC_*` — the value must live in `eas.json` (or an EAS env var).

### Cloud build (Expo account) — primary path

```bash
cd app
npm install
npx expo login                 # operator's Expo credentials (operator step)
npx eas build --platform android --profile preview
```

EAS runs the build in the cloud and returns an **install URL / QR** and a downloadable
**APK** (internal distribution). Install it by opening the URL on the device, or transfer
the `.apk` and allow "install unknown apps".

### No-account local build — the alternative

Build directly on a machine with the Android SDK + JDK and a connected device/emulator,
**no Expo account required**:

```bash
cd app
npm install
EXPO_PUBLIC_API_URL=https://money.example.com/api/v1 npx expo run:android
```

Here Metro runs on your machine and **does** read the shell/`.env` value (unlike the cloud
build), so set `EXPO_PUBLIC_API_URL` inline as shown. This prebuilds the native project
(`app/android/`, gitignored), compiles a debug APK, and installs it on the connected
device/emulator.

**Prerequisites:** Android Studio / Android SDK, JDK 17, and a device with USB debugging
enabled (or a running emulator).

- iOS analogue for QA: `npx expo run:ios` (simulator) — no Apple account needed for the
  simulator; a provisioned device needs an Apple Developer account.
- Third option: `npx eas build --platform android --profile preview --local` runs the EAS
  build locally (still needs the Android toolchain, but no cloud).

### After install — QA

Run **[`docs/qa-device-checklist.md`](docs/qa-device-checklist.md)** on the device to sign
off the golden path (register → create INR + EUR groups → add member by email → chores /
base / loan → statement → record payment → notifications bell). This on-device sign-off is
the native-acceptance gate — there is no emulator in CI.

> **iOS internal distribution** additionally needs Apple provisioning (an Apple Developer
> account) — an operator step; for local iOS QA use `npx expo run:ios` (simulator).

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
| `EXPO_PUBLIC_API_URL` | Backend API URL (e.g., `http://192.168.1.x:8080/api/v1`). For a **native build** this is **build-time** — baked into the APK from the `eas.json` profile `env` (see [Native App (Android APK)](#native-app-android-apk)); for the local `npx expo run:android` path it is read from the shell/`.env`. For the web single image it isn't needed — the web app is same-origin with the API at `/api/v1`. |

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

## Backups & Restore

The Postgres volume (`postgres_data`) is the **only** copy of the family's money ledger. Back it up regularly. All commands run `pg_dump`/`pg_restore` inside the existing `postgres` container — no Postgres client tools needed on the host.

```bash
# Create a compressed, timestamped dump in ./backups/ and prune to the newest 14
make backup

# Verify the newest dump restores cleanly into a throwaway db (live db untouched)
make backup-verify

# Verify a specific dump
make backup-verify BACKUP=backups/pocket_money_20260705_020000.dump

# Restore a dump into the LIVE db (prompts for 'yes' before overwriting)
make restore BACKUP=backups/pocket_money_20260705_020000.dump

# Skip the confirmation prompt (for scripts/CI)
make restore BACKUP=backups/pocket_money_20260705_020000.dump FORCE=1
```

**Nightly cron** (keeps the newest 14 dumps, logs to `backups/backup.log`):

```cron
# m h dom mon dow   nightly Pocket Money DB backup (keeps newest 14), logs to backups/backup.log
0 2 * * *  cd /home/<user>/pocket-money && PATH=/usr/local/bin:/usr/bin:/bin /usr/bin/make backup >> /home/<user>/pocket-money/backups/backup.log 2>&1
```

Replace `/home/<user>/pocket-money` with the actual deploy path. Confirm `which make` and `which docker` match the `PATH` above. To retain more dumps, add `KEEP=30` to the cron line.

Optional weekly verify (confirms the newest dump is always restorable):

```cron
30 2 * * 0  cd /home/<user>/pocket-money && PATH=/usr/local/bin:/usr/bin:/bin /usr/bin/make backup-verify >> /home/<user>/pocket-money/backups/verify.log 2>&1
```

**Notes:**

- `./backups/*.dump` is gitignored — dumps contain family financial data and must never be committed. Only `backups/.gitkeep` is tracked.
- Override credentials on the command line if needed: `make backup DB_USER=… DB_NAME=…`
- Off-machine/offsite copies (rsync to NAS, cloud storage) are a future ops task and not part of this setup.

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
