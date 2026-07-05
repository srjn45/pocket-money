import { test, expect, type Browser, type BrowserContext, type Page } from '@playwright/test';
import {
  uniqueEmail,
  registerUser,
  createGroup,
  inviteAndCaptureToken,
  openTab,
  reloadInto,
  autoAcceptDialogs,
  headAddEntry,
} from '../support/pages';

const WEB_BASE = process.env.E2E_WEB_BASE ?? 'http://localhost:8081';

async function joinGroupAsMember(
  browser: Browser,
  token: string,
  name: string,
  password = 'member123!',
): Promise<{ ctx: BrowserContext; page: Page; email: string }> {
  const ctx = await browser.newContext();
  const pg = await ctx.newPage();
  await pg.goto(`${WEB_BASE}/invite?token=${token}`);
  await expect(pg.getByTestId('login-submit')).toBeVisible({ timeout: 15_000 });
  await pg.getByTestId('login-link-register').click();
  await expect(pg.getByTestId('register-submit')).toBeVisible({ timeout: 10_000 });
  const email = uniqueEmail();
  await pg.getByTestId('register-name').fill(name);
  await pg.getByTestId('register-email').fill(email);
  await pg.getByTestId('register-password').fill(password);
  await pg.getByTestId('register-confirm').fill(password);
  await pg.getByTestId('register-submit').click();
  await expect(pg.getByTestId('group-overview-root')).toBeVisible({ timeout: 20_000 });
  return { ctx, page: pg, email };
}

// T-CP: change password. A wrong current password returns 403 and MUST NOT log
// the user out (the fetch client force-logs-out only on 401). A correct change
// succeeds and the user stays on Profile.
test('T-CP: wrong current password shows error and keeps session; correct change succeeds', async ({
  page,
}) => {
  const email = uniqueEmail();
  const originalPass = 'original123!';
  const newPass = 'newpassword456!';

  await registerUser(page, 'CP User', email, originalPass);

  // Navigate to Profile (register lands on the Dashboard tab).
  await openTab(page, /profile/i);
  await expect(page.getByTestId('profile-root')).toBeVisible({ timeout: 10_000 });

  await page.getByTestId('profile-change-password').click();

  // Wrong current password → 403.
  await page.getByTestId('cp-current-password').fill('wrongpassword');
  await page.getByTestId('cp-new-password').fill(newPass);
  await page.getByTestId('cp-confirm-password').fill(newPass);
  await page.getByTestId('cp-submit').click();

  // Error toast appears (proves the 403 was received), and the user is STILL
  // logged in — not bounced to /login. This is the 403-not-401 guarantee.
  await expect(page.getByTestId('toast-root')).toBeVisible({ timeout: 10_000 });
  await expect(page.getByTestId('profile-root')).toBeVisible();
  await expect(page.getByTestId('login-submit')).not.toBeVisible();

  // Correct current password → success, still logged in.
  await page.getByTestId('cp-current-password').fill(originalPass);
  await page.getByTestId('cp-new-password').fill(newPass);
  await page.getByTestId('cp-confirm-password').fill(newPass);
  await page.getByTestId('cp-submit').click();

  await expect(page.getByTestId('toast-root')).toBeVisible({ timeout: 10_000 });
  await expect(page.getByTestId('profile-root')).toBeVisible();
  await expect(page.getByTestId('login-submit')).not.toBeVisible();
});

// T-LEAVE: a member with a non-zero balance is blocked from leaving (409 toast,
// still in the group). After the head settles the balance to zero, the member
// can leave and lands back on the dashboard.
test('T-LEAVE: leave blocked by balance (409) → settle → leave succeeds', async ({ page, browser }) => {
  await registerUser(page, 'Head Leave', uniqueEmail());
  await createGroup(page, `LeaveFam-${Date.now()}`);

  const token = await inviteAndCaptureToken(page);
  const { ctx: memberCtx, page: memberPage } = await joinGroupAsMember(browser, token, 'Member Leave');
  autoAcceptDialogs(memberPage); // window.confirm() on web — auto-accept the leave prompt

  try {
    // Head gives the member a non-zero balance: an adjustment credit is auto-approved.
    await reloadInto(page, 'group-overview-root');
    const memberCard = page.getByTestId(/^member-card-/).first();
    await expect(memberCard).toBeVisible({ timeout: 10_000 });
    await memberCard.click();
    await expect(page.getByTestId('member-detail-root')).toBeVisible({ timeout: 10_000 });
    await headAddEntry(page, 'adjustment', '50', 'credit');
    await expect(page.getByTestId('toast-root')).toBeVisible({ timeout: 10_000 });

    // Member: attempt to leave → blocked (409), still in the group.
    await memberPage.reload();
    await expect(memberPage.getByTestId('group-overview-root')).toBeVisible({ timeout: 20_000 });
    await memberPage.getByTestId('group-leave-button').click();
    await expect(memberPage.getByTestId('toast-root')).toBeVisible({ timeout: 10_000 });
    await expect(memberPage.getByTestId('group-overview-root')).toBeVisible();

    // Head: settle the member's balance back to zero (settlement debit of ₹50).
    await headAddEntry(page, 'settlement', '50');
    await expect(page.getByTestId('toast-root')).toBeVisible({ timeout: 10_000 });

    // Member: leave now succeeds → lands on the dashboard.
    await memberPage.reload();
    await expect(memberPage.getByTestId('group-overview-root')).toBeVisible({ timeout: 20_000 });
    await memberPage.getByTestId('group-leave-button').click();
    await expect(memberPage.getByTestId('dashboard-root')).toBeVisible({ timeout: 20_000 });
  } finally {
    await memberCtx.close();
  }
});
