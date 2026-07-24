# Releasing Pocket Money (Android)

Releases are cut by **CI on a tag push** — `.github/workflows/release-mobile.yml`
builds a signed APK (sideloading) and AAB (Play Console) on the GitHub runner
with Gradle (no EAS, no build quota) and publishes both as a GitHub Release.
The website's fixed download URL
(`releases/latest/download/pocket-money.apk`) picks up the latest release
automatically.

A [local build](#local-build-fallback) via `npm run build:apk` remains available
as a fallback / for quick on-device testing.

The old EAS workflows (`.github/workflows/android-release.yml`,
`android-publish.yml`) are **disabled** (manual `workflow_dispatch` only) — the
EAS free-tier queue sat for hours and repeatedly killed the release CD.

## Cut a release

1. **Bump the version** in `app/app.json` (on a branch → PR → merge to
   `master`):
   - `expo.version` → the new `X.Y.Z` (the tag must match it exactly, or the
     release job fails fast)
   - `expo.android.versionCode` → **+1** (Android requires an increasing
     integer to install an update over an older build)

2. **Tag the merged commit and push the tag:**

   ```bash
   git checkout master && git pull
   VER=$(node -p "require('./app/app.json').expo.version")
   git tag "v$VER" && git push origin "v$VER"
   ```

   CI takes it from there: verify (lint + API-types drift) → prebuild →
   Gradle `assembleRelease` + `bundleRelease` → signing-cert check → GitHub
   Release with `pocket-money.apk` + `pocket-money.aab` attached.

   > The APK asset MUST stay named `pocket-money.apk` — the website's
   > `releases/latest/download/pocket-money.apk` redirect depends on it.

3. **Smoke-test the pipeline without releasing:** run the workflow manually
   (`gh workflow run release-mobile.yml`) — a non-tag run uploads the APK/AAB
   as run artifacts and publishes nothing.

## Signing

Artifacts are signed with the **Pocket Money upload keystore**
(`~/keystores/pocket-money/upload.keystore`, alias `pocket-money-upload`,
credentials in `credentials.properties` beside it).

- **CI** gets it from four repo secrets: `ANDROID_KEYSTORE_BASE64`,
  `ANDROID_KEYSTORE_PASSWORD`, `ANDROID_KEY_ALIAS`, `ANDROID_KEY_PASSWORD`.
  The workflow pins the certificate SHA-256 (`EXPECTED_CERT_SHA256`) and fails
  before publishing if an artifact is signed by anything else — a
  wrongly-signed AAB uploaded to Play Console is unrecoverable.
- **Local builds** pick up the same key via `POCKET_MONEY_UPLOAD_*` properties
  in `~/.gradle/gradle.properties`. Without them (fresh clone), release builds
  fall back to the Expo template's debug keystore.
- The signing config is injected into the generated `android/` project on
  every `expo prebuild` by `app/plugins/withReleaseSigning.js` (the `android/`
  dir is gitignored — CNG).

⚠️ **Back up `~/keystores/pocket-money/` somewhere off this laptop** (password
manager, encrypted drive). Lose it and existing installs can never be updated
in place again; anyone who has it can sign updates as you.

One-time migration note: installs of **v1.0.3 or older** (EAS- or debug-key
signed) must be uninstalled once before an upload-key-signed build installs.

## API URL

The app talks only to the server URL baked in at bundle time:

- **CI** bakes the `EXPO_PUBLIC_API_URL` **repo variable** (currently the
  Fly.io deployment `https://pocket-money-srjn45.fly.dev/api/v1`); the release
  job fails fast if the variable is unset. Change it with
  `gh variable set EXPO_PUBLIC_API_URL --body "https://..."`.
- **Local builds** read `app/.env.local` (gitignored; template:
  `app/.env.local.example`) or the `EXPO_PUBLIC_API_URL` env var.

## Local build (fallback)

```bash
cd app
npm run build:apk          # → app/pocket-money.apk
adb install -r pocket-money.apk
```

Runs `scripts/build-apk.sh`: `expo prebuild -p android --clean` + `gradle
assembleRelease`. Needs JDK 21 + Android SDK at `$ANDROID_HOME` (falls back to
`~/Android/Sdk`). Signs with the upload keystore when
`~/.gradle/gradle.properties` is configured (it is, on this box), else the
debug key.

## Troubleshooting

- **`EXPO_PUBLIC_API_URL is not set`** (local) — create `app/.env.local`.
- **Release job fails at Gradle configure with `Process 'command 'node''
  finished with non-zero exit value 1`** — almost certainly the
  `gradle.properties` append glued onto the last line: `expo prebuild` writes
  the file WITHOUT a trailing newline; the workflow emits a newline before
  appending. Keep it that way.
- **Signing-cert mismatch step fails** — a repo secret points at the wrong
  keystore. Re-encode: `base64 -w0 ~/keystores/pocket-money/upload.keystore |
  gh secret set ANDROID_KEYSTORE_BASE64`.
- **"App not installed" on device** — installing over a differently-signed
  build (old EAS/debug-signed APK). Uninstall the old app first.
- **`INSTALL_FAILED_VERSION_DOWNGRADE`** — the device has a higher
  `versionCode`; bump `expo.android.versionCode` in `app.json`.
- **Tag-mismatch failure** — the tag `vX.Y.Z` must equal `expo.version`
  exactly; bump the version on master first, then re-tag.
