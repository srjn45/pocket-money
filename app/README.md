# Pocket Money App

React Native (Expo) mobile app for the Pocket Money application.

## Prerequisites

- Node.js 18+
- npm or yarn
- Expo CLI (`npm install -g expo-cli`)
- For iOS: Xcode (Mac only)
- For Android: Android Studio with emulator or physical device

## Setup

1. Install dependencies:
   ```bash
   npm install
   ```

2. Configure API URL (optional):
   Create a `.env` file or set the environment variable:
   ```bash
   EXPO_PUBLIC_API_URL=http://192.168.1.x:8080/api/v1
   ```
   Replace with your backend server's LAN IP address.

## Development

### Start Development Server
```bash
npm start
```

### Run on Platforms
```bash
# iOS
npm run ios

# Android
npm run android

# Web
npm run web
```

## Deep Linking

The app supports deep links for invite tokens:

- Web: `http://your-domain/invite?token=xxx`
- Mobile: `pocketmoney://invite?token=xxx`

## Features

- User authentication (register/login)
- Create and manage groups
- Invite members via shareable links
- Manage chores with amounts
- Track ledger entries (earnings)
- Approve/reject pending entries (head only)
- Record settlements (payouts)
- View per-member balances

## Project Structure

```
app/
├── app/                    # Expo Router screens
│   ├── (auth)/            # Auth screens (login, register)
│   ├── (app)/             # Main app screens
│   │   ├── groups/        # Groups list and detail
│   │   │   └── [id]/      # Group detail tabs
│   │   └── profile.tsx    # User profile
│   ├── invite.tsx         # Invite handling
│   └── _layout.tsx        # Root layout
├── src/
│   ├── api.ts             # API client
│   ├── auth-context.tsx   # Auth state management
│   ├── storage.ts         # Token storage
│   └── components/        # Shared UI components
└── assets/                # Images and icons
```

## Troubleshooting

### API Connection Issues
- Ensure the backend is running and accessible
- Check that `EXPO_PUBLIC_API_URL` is set correctly
- For physical devices, use LAN IP (not localhost)
- Verify CORS is configured on the backend

### Deep Links Not Working
- Ensure the app scheme is configured in `app.json`
- For Android, rebuild the app after changing the scheme
- For iOS, the scheme should work automatically in Expo Go
