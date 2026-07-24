#!/usr/bin/env bash
#
# Build a release Android APK locally — no EAS cloud queue.
#
# The EAS free-tier build queue can sit for HOURS, which repeatedly killed the
# release CD (v1.0.2, v1.0.3). This builds the same preview-profile APK on this
# machine instead: `expo prebuild` generates the native android/ project, then
# Gradle assembles a release APK.
#
# Signing: plugins/withReleaseSigning.js injects a release signingConfig that
# uses the Pocket Money upload keystore when the POCKET_MONEY_UPLOAD_* Gradle
# properties are present (set in ~/.gradle/gradle.properties on this box,
# keystore at ~/keystores/pocket-money/). This matches what CI releases are
# signed with (.github/workflows/release-mobile.yml), so local and CI builds
# update over each other in place. Without those properties the build falls
# back to the template's well-known debug keystore (fresh clones still build).
# Installing over an APK signed with a DIFFERENT key (old EAS builds, or
# debug-signed ↔ upload-signed) needs a one-time uninstall.
#
# Usage:
#   app/scripts/build-apk.sh            # API URL from .env.production (or .env.local override)
#   EXPO_PUBLIC_API_URL=https://... app/scripts/build-apk.sh
#
# Prereqs (already set up on this box, see ~/.zshrc): JDK 21, Android SDK at
# $ANDROID_HOME with a platform + build-tools, and adb on PATH.
set -euo pipefail

# Always operate from the app/ directory regardless of where we're invoked.
cd "$(dirname "$0")/.."
APP_DIR="$(pwd)"

# --- API URL baked into the APK (metro inlines EXPO_PUBLIC_* at bundle time) ---
# Precedence: real env var > .env.local (gitignored override) > .env.production
# (canonical, committed). The Gradle release bundling loads the .env files
# itself; we only resolve here to fail fast and echo what will be baked.
if [ -z "${EXPO_PUBLIC_API_URL:-}" ] && [ -f .env.local ]; then
  # shellcheck disable=SC1091
  set -a; . ./.env.local; set +a
fi
if [ -z "${EXPO_PUBLIC_API_URL:-}" ] && [ -f .env.production ]; then
  # shellcheck disable=SC1091
  set -a; . ./.env.production; set +a
fi
if [ -z "${EXPO_PUBLIC_API_URL:-}" ]; then
  echo "ERROR: EXPO_PUBLIC_API_URL is not set." >&2
  echo "  Expected app/.env.production (committed) to provide it, or override" >&2
  echo "  via app/.env.local / the env var, e.g." >&2
  echo "  EXPO_PUBLIC_API_URL=https://money.example.com/api/v1 app/scripts/build-apk.sh" >&2
  exit 1
fi
export EXPO_PUBLIC_API_URL

VER="$(node -p "require('./app.json').expo.version")"
CODE="$(node -p "require('./app.json').expo.android.versionCode || 1")"
echo "==> Building Pocket Money v${VER} (versionCode ${CODE})"
echo "==> API URL baked in: ${EXPO_PUBLIC_API_URL}"

# --- Generate the native android/ project (regenerated clean every build) ---
echo "==> expo prebuild (android)…"
npx expo prebuild --platform android --clean

# prebuild rewrites the "android"/"ios" npm scripts to the run:* variants; we
# keep the dev-server variants, so restore them and leave the tree clean.
node -e "
  const fs=require('fs'),f='package.json',j=JSON.parse(fs.readFileSync(f));
  j.scripts.android='expo start --android';
  j.scripts.ios='expo start --ios';
  fs.writeFileSync(f, JSON.stringify(j,null,2)+'\n');
"

# --- Assemble the release APK ---
# Gradle needs the Android SDK location. Prefer an already-exported ANDROID_HOME
# (set in ~/.zshrc); fall back to the default install path so the build also
# works from a non-interactive shell.
export ANDROID_HOME="${ANDROID_HOME:-$HOME/Android/Sdk}"
export ANDROID_SDK_ROOT="${ANDROID_SDK_ROOT:-$ANDROID_HOME}"
if [ ! -d "$ANDROID_HOME/platforms" ]; then
  echo "ERROR: Android SDK not found at ANDROID_HOME=$ANDROID_HOME" >&2
  echo "  Install the Android SDK, or set ANDROID_HOME to your SDK path." >&2
  exit 1
fi
echo "==> gradle assembleRelease… (ANDROID_HOME=$ANDROID_HOME)"
( cd android && ./gradlew --no-daemon assembleRelease )

# --- Collect the artifact under a stable name ---
APK_SRC="android/app/build/outputs/apk/release/app-release.apk"
OUT="pocket-money.apk"
cp "$APK_SRC" "$OUT"

echo ""
echo "✅ Built v${VER} → ${APP_DIR}/${OUT}"
echo "   Install on a USB-connected device:  adb install -r \"${APP_DIR}/${OUT}\""
echo "   (Installing over an APK signed with a different key — e.g. an old EAS or"
echo "    debug-signed build — needs a one-time uninstall first.)"
