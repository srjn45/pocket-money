# Pocket Money Backend

Go backend API for the Pocket Money application.

## Prerequisites

- Go 1.22+
- PostgreSQL 15+
- Docker (for running tests)

## Setup

1. Copy the environment variables:
   ```bash
   export DATABASE_URL="postgres://user:password@localhost:5432/pocket_money?sslmode=disable"
   export JWT_SECRET="your-secret-key"
   export PORT="8080"
   export CORS_ORIGINS="http://localhost:3000,http://localhost:8081"
   ```

2. Run migrations and start the server:
   ```bash
   make run
   ```

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| DATABASE_URL | Yes | - | PostgreSQL connection string |
| JWT_SECRET | Yes | - | Secret key for JWT signing |
| PORT | No | 8080 | Server port |
| CORS_ORIGINS | No | * | Comma-separated allowed origins |

## Development

### Build
```bash
make build
```

### Run Tests
```bash
make test
```

### Run Integration Tests
```bash
make test-integration
```

### Start Test Database
```bash
make test-up
```

### Stop Test Database
```bash
make test-down
```

## API Endpoints

### Auth
- `POST /api/v1/auth/register` - Register new user
- `POST /api/v1/auth/login` - Login
- `GET /api/v1/auth/me` - Get current user (authenticated)

### Groups
- `POST /api/v1/groups` - Create group
- `GET /api/v1/groups` - List user's groups
- `GET /api/v1/groups/:id` - Get group details
- `GET /api/v1/groups/:id/members` - List group members
- `POST /api/v1/groups/:id/invite` - Generate invite (head only)
- `POST /api/v1/groups/join` - Join group with token

### Chores
- `GET /api/v1/groups/:id/chores` - List chores
- `POST /api/v1/groups/:id/chores` - Create chore (head only)
- `PATCH /api/v1/chores/:id` - Update chore (head only)
- `DELETE /api/v1/chores/:id` - Delete chore (head only)

### Ledger
- `GET /api/v1/groups/:id/ledger` - List ledger entries
- `POST /api/v1/groups/:id/ledger` - Create ledger entry
- `POST /api/v1/ledger/:id/approve` - Approve entry (head only)
- `POST /api/v1/ledger/:id/reject` - Reject entry (head only)
- `GET /api/v1/groups/:id/pending` - List pending entries (head only)
- `GET /api/v1/groups/:id/balance` - Get member balances

### Settlements
- `GET /api/v1/groups/:id/settlements` - List settlements
- `POST /api/v1/groups/:id/settlements` - Create settlement (head only)
