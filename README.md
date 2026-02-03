# Pocket Money

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

This is a monorepo containing:

```
pocket-money/
├── backend/           # Go API server
│   ├── cmd/server/   # Main entry point
│   ├── internal/     # Application code
│   ├── migrations/   # Database migrations
│   └── README.md     # Backend documentation
├── app/              # React Native (Expo) app
│   ├── app/          # Screens and routes
│   ├── src/          # Shared code
│   └── README.md     # App documentation
└── docs/             # Project documentation
```

## Quick Start

### Backend

```bash
cd backend

# Set environment variables
export DATABASE_URL="postgres://user:pass@localhost:5432/pocket_money"
export JWT_SECRET="your-secret"

# Run the server
make run
```

### Mobile App

```bash
cd app

# Install dependencies
npm install

# Set API URL (use your server's LAN IP)
export EXPO_PUBLIC_API_URL="http://192.168.1.x:8080/api/v1"

# Start Expo
npm start
```

## Tech Stack

- **Backend**: Go, Gin, PostgreSQL, golang-migrate
- **Mobile**: React Native, Expo, TypeScript, expo-router
- **Auth**: JWT bearer tokens

## License

This project uses dual licensing:
- **Non-commercial use**: [PolyForm Noncommercial License 1.0.0](LICENSE)
- **Commercial use**: Contact the copyright holder for a commercial license

See [COMMERCIAL_LICENSE.md](COMMERCIAL_LICENSE.md) for details.
