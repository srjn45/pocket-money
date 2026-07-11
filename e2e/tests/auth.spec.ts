import { test, expect } from '@playwright/test';
import { uniqueEmail, registerUser, openTab } from '../support/pages';

// Auth/nav basics. The group-creation, add-by-email/shadow/claim, and EUR-currency
// journeys that used to live here (T2 / T2-EUR / T3) are absorbed by
// golden-path.spec.ts; this file keeps only the register/login/empty-dashboard
// invariants (V3-4.4 §2).

// T1: Register → auto-login → lands on dashboard (no separate login step).
test('T1: register auto-logs-in and lands on dashboard', async ({ page }) => {
  const email = uniqueEmail();
  await registerUser(page, 'Test User', email);
  await expect(page.getByTestId('dashboard-root')).toBeVisible();
  // Should NOT have gone through /login first — dashboard appears directly.
  expect(page.url()).not.toContain('/login');
});

// T-LOGIN: Login with bad credentials shows an error, with good ones succeeds.
test('T-LOGIN: wrong password shows error; correct password enters app', async ({ page }) => {
  const email = uniqueEmail();
  // First register the user.
  await registerUser(page, 'Login Test', email);
  // Log out (register lands on the Dashboard tab → go to Profile first).
  await openTab(page, 'profile');
  await expect(page.getByTestId('profile-logout')).toBeVisible({ timeout: 10_000 });
  await page.getByTestId('profile-logout').click();
  await expect(page.getByTestId('login-submit')).toBeVisible({ timeout: 15_000 });

  // Wrong password.
  await page.getByTestId('login-email').fill(email);
  await page.getByTestId('login-password').fill('wrongpassword');
  await page.getByTestId('login-submit').click();
  // Should stay on login page (no dashboard visible).
  await expect(page.getByTestId('login-submit')).toBeVisible({ timeout: 10_000 });
  await expect(page.getByTestId('dashboard-root')).not.toBeVisible();

  // Correct password. Reload first for a clean form: re-filling and immediately
  // resubmitting on the same screen can race the previous failed attempt's state
  // and resend the stale password.
  await page.reload();
  await expect(page.getByTestId('login-submit')).toBeVisible({ timeout: 15_000 });
  await page.getByTestId('login-email').fill(email);
  await page.getByTestId('login-password').fill('password123');
  await page.getByTestId('login-submit').click();
  await expect(page.getByTestId('dashboard-root')).toBeVisible({ timeout: 15_000 });
});

// T-DASHBOARD-EMPTY: Brand-new user has no groups → dashboard shows empty state.
test('T-DASHBOARD-EMPTY: new user sees dashboard empty state', async ({ page }) => {
  await registerUser(page, 'Empty Dashboard', uniqueEmail());
  await expect(page.getByTestId('dashboard-empty')).toBeVisible();
});
