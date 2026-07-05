import { test, expect, type BrowserContext } from '@playwright/test';
import { uniqueEmail, registerUser, createGroup, inviteAndCaptureToken, openTab } from '../support/pages';

// T1: Register → auto-login → lands on dashboard (no separate login step).
test('T1: register auto-logs-in and lands on dashboard', async ({ page }) => {
  const email = uniqueEmail();
  await registerUser(page, 'Test User', email);
  await expect(page.getByTestId('dashboard-root')).toBeVisible();
  // Should NOT have gone through /login first — dashboard appears directly.
  expect(page.url()).not.toContain('/login');
});

// T2: Create group → invite button renders the overview.
test('T2: create group shows overview with invite button', async ({ page }) => {
  const email = uniqueEmail();
  const groupName = `Family-${Date.now()}`;
  await registerUser(page, 'Head User', email);
  await createGroup(page, groupName);
  await expect(page.getByTestId('group-overview-root')).toBeVisible();
  await expect(page.getByTestId('group-invite-button')).toBeVisible();
});

// T3: Register via invite → lands inside the group (member view).
test('T3: invite link → register → lands inside group', async ({ page, browser }) => {
  // ── Head registers and creates a group ────────────────────────────────────
  const headEmail = uniqueEmail();
  await registerUser(page, 'Head T3', headEmail);
  await createGroup(page, `InviteFamily-${Date.now()}`);

  // ── Head generates an invite token (captured from the API response) ───────
  const token = await inviteAndCaptureToken(page);

  // ── Second context: member navigates to the invite link ───────────────────
  const memberCtx: BrowserContext = await browser.newContext();
  const memberPage = await memberCtx.newPage();

  try {
    const webBase = process.env.E2E_WEB_BASE ?? 'http://localhost:8081';
    await memberPage.goto(`${webBase}/invite?token=${token}`);

    // invite.tsx: no auth → saves token → redirects to /login.
    await expect(memberPage.getByTestId('login-submit')).toBeVisible({ timeout: 15_000 });

    // Navigate to register form.
    await memberPage.getByTestId('login-link-register').click();
    await expect(memberPage.getByTestId('register-submit')).toBeVisible({ timeout: 10_000 });

    const memberEmail = uniqueEmail();
    await memberPage.getByTestId('register-name').fill('Member T3');
    await memberPage.getByTestId('register-email').fill(memberEmail);
    await memberPage.getByTestId('register-password').fill('member123!');
    await memberPage.getByTestId('register-confirm').fill('member123!');
    await memberPage.getByTestId('register-submit').click();

    // After registration the pending invite token is consumed → member lands
    // inside the group (group-overview-root visible as the member view).
    await expect(memberPage.getByTestId('group-overview-root')).toBeVisible({ timeout: 20_000 });

    // Member view should show leave button, not invite button.
    await expect(memberPage.getByTestId('group-leave-button')).toBeVisible();
    await expect(memberPage.getByTestId('group-invite-button')).not.toBeVisible();
  } finally {
    await memberCtx.close();
  }
});

// T-LOGIN: Login with bad credentials shows an error, with good ones succeeds.
test('T-LOGIN: wrong password shows error; correct password enters app', async ({ page }) => {
  const email = uniqueEmail();
  // First register the user.
  await registerUser(page, 'Login Test', email);
  // Log out (register lands on the Dashboard tab → go to Profile first).
  await openTab(page, /profile/i);
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

  // Correct password.
  await page.getByTestId('login-password').fill('password123');
  await page.getByTestId('login-submit').click();
  await expect(page.getByTestId('dashboard-root')).toBeVisible({ timeout: 15_000 });
});

// T-DASHBOARD-EMPTY: Brand-new user has no groups → dashboard shows empty state.
test('T-DASHBOARD-EMPTY: new user sees dashboard empty state', async ({ page }) => {
  await registerUser(page, 'Empty Dashboard', uniqueEmail());
  await expect(page.getByTestId('dashboard-empty')).toBeVisible();
});
