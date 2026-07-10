// Build-level feature flags (master-plan-v3 D8).
//
// Invite-link sharing is HIDDEN for MVP: add-by-email (§3.1) is the sole visible
// membership mechanism. The invite/join *code* is kept (deep-link route, API
// client, join handlers) so the feature can be re-enabled without a rebuild of
// logic — flip this flag at build time:
//   EXPO_PUBLIC_INVITES_ENABLED=true npx expo export --platform web
// Unset/anything-but-"true" ⇒ OFF (the default, and what CI/e2e build with).
export const INVITES_ENABLED = process.env.EXPO_PUBLIC_INVITES_ENABLED === 'true';
