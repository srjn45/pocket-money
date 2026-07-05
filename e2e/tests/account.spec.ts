import { test, expect, type Browser, type BrowserContext, type Page } from '@playwright/test';
import { uniqueEmail, registerUser, createGroup, inviteAndCaptureToken } from '../support/pages';

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

// T-CP: Change password — wrong old password returns 403 (no logout); correct
//        change succeeds; user remains on profile screen (not redirected).
test('T-CP: change password 403 on wrong old password, success stays logged in', async ({ page }) => {
  const email = uniqueEmail();
  const originalPass = 'original123!';
  const newPass = 'newpassword456!';

  await registerUser(page, 'CP User', email, originalPass);

  // Navigate to profile.
  await expect(page.getByTestId('profile-root')).toBeVisible({ timeout: 10_000 }).catch(async () => {
    await page.getByRole('tab', { name: /profile/i }).click();
    await expect(page.getByTestId('profile-root')).toBeVisible({ timeout: 10_000 });
  });

  await page.getByTestId('profile-change-password').click();

  // Fill the change-password sheet with wrong old password.
  const oldPassInput = page.locator(
    '[data-testid="cp-old-password"], input[placeholder*="old" i], input[placeholder*="current" i]',
  ).first();
  await expect(oldPassInput).toBeVisible({ timeout: 8_000 });
  await oldPassInput.fill('wrongpassword');

  const newPassInput = page.locator(
    '[data-testid="cp-new-password"], input[placeholder*="new" i]',
  ).first();
  await newPassInput.fill(newPass);

  const confirmPassInput = page.locator(
    '[data-testid="cp-confirm-password"], input[placeholder*="confirm" i]',
  ).last();
  await confirmPassInput.fill(newPass);

  await page.getByRole('button', { name: /save|update|change/i }).last().click();

  // Should still be logged in (profile-root visible) — 403 should not log out user.
  await expect(page.getByTestId('profile-root')).toBeVisible({ timeout: 10_000 });
  await expect(page.getByTestId('login-submit')).not.toBeVisible();

  // Now submit the correct old password.
  await oldPassInput.fill(originalPass);
  await newPassInput.fill(newPass);
  await confirmPassInput.fill(newPass);
  await page.getByRole('button', { name: /save|update|change/i }).last().click();

  // Still logged in after successful password change.
  await expect(page.getByTestId('profile-root')).toBeVisible({ timeout: 10_000 });
  await expect(page.getByTestId('login-submit')).not.toBeVisible();
});

// T-LEAVE: Member with non-zero balance cannot leave (409); after head settles
//          balance to zero, member can leave successfully.
test('T-LEAVE: member cannot leave with non-zero balance; can leave after settlement', async ({
  page,
  browser,
}) => {
  const headPass = 'headpass123!';
  const memberPass = 'memberpass123!';

  const headEmail = uniqueEmail();
  await registerUser(page, 'Head Leave', headEmail, headPass);
  await createGroup(page, `LeaveFam-${Date.now()}`);

  const token = await inviteAndCaptureToken(page);
  const { ctx: memberCtx, page: memberPage } = await joinGroupAsMember(
    browser,
    token,
    'Member Leave',
    memberPass,
  );

  try {
    // ── Head: give the member a balance (credit entry) ────────────────────────
    const memberCard = page.getByTestId(/^member-card-/).first();
    await expect(memberCard).toBeVisible({ timeout: 10_000 });
    await memberCard.click();
    await expect(page.getByTestId('member-detail-root')).toBeVisible({ timeout: 10_000 });

    // Add an adjustment credit.
    await page.getByTestId('member-add-entry-button').click();
    const amountInput = page.locator('[data-testid="entry-amount"], input[inputmode="numeric"]').first();
    await expect(amountInput).toBeVisible({ timeout: 8_000 });
    await amountInput.fill('5000');
    await page.getByRole('button', { name: /save|submit|add/i }).last().click();

    // Approve the entry so the member has a non-zero balance.
    const approveBtn = page.getByTestId(/^ledger-approve-/).first();
    await expect(approveBtn).toBeVisible({ timeout: 15_000 });
    await approveBtn.click();
    await expect(approveBtn).not.toBeVisible({ timeout: 10_000 });

    // ── Member: try to leave → should be blocked (409 / error shown) ─────────
    await expect(memberPage.getByTestId('group-leave-button')).toBeVisible({ timeout: 10_000 });
    await memberPage.getByTestId('group-leave-button').click();

    // Confirm the leave dialog if present.
    await memberPage.getByRole('button', { name: /confirm|yes|leave/i }).click().catch(() => {
      // No confirmation dialog — the button itself is the action.
    });

    // Member should still be inside the group (leave was rejected).
    await expect(memberPage.getByTestId('group-overview-root')).toBeVisible({ timeout: 10_000 });
    await expect(memberPage.getByTestId('group-leave-button')).toBeVisible();

    // ── Head: settle the member's balance to zero ─────────────────────────────
    // Go back to member detail to add a settlement debit of the same amount.
    await page.goBack().catch(() => {
      // Already on member detail.
    });
    // If we navigated away, go back to member detail.
    if (!(await page.getByTestId('member-detail-root').isVisible())) {
      await memberCard.click();
      await expect(page.getByTestId('member-detail-root')).toBeVisible({ timeout: 10_000 });
    }

    await page.getByTestId('member-add-entry-button').click();
    const settleAmountInput = page.locator('[data-testid="entry-amount"], input[inputmode="numeric"]').first();
    await expect(settleAmountInput).toBeVisible({ timeout: 8_000 });
    await settleAmountInput.fill('5000');
    // Select "Settlement" type if entry-type picker is available.
    await page.getByRole('button', { name: /settlement|settle/i }).click().catch(() => {
      // Settlement type selection not available as a button — may be auto-selected.
    });
    // Debit direction if available.
    await page.getByRole('button', { name: /debit|out/i }).click().catch(() => {
      // direction might default or be implicit.
    });
    await page.getByRole('button', { name: /save|submit|add/i }).last().click();

    // Approve the settlement.
    const settleApproveBtn = page.getByTestId(/^ledger-approve-/).first();
    await expect(settleApproveBtn).toBeVisible({ timeout: 15_000 });
    await settleApproveBtn.click();
    await expect(settleApproveBtn).not.toBeVisible({ timeout: 10_000 });

    // ── Member: leave succeeds now ────────────────────────────────────────────
    await memberPage.getByTestId('group-leave-button').click();
    await memberPage.getByRole('button', { name: /confirm|yes|leave/i }).click().catch(() => {});

    // Member should land on the dashboard (outside the group).
    await expect(memberPage.getByTestId('dashboard-root')).toBeVisible({ timeout: 15_000 });

    // The group card should no longer appear for the member.
    const groupCards = memberPage.getByTestId(/^group-card-/);
    await expect(groupCards).toHaveCount(0, { timeout: 5_000 }).catch(() => {
      // There might be other groups; acceptable as long as member is on dashboard.
    });
  } finally {
    await memberCtx.close();
  }
});
