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

// T4: Head logs a chore entry for the member (pending) → head approves it.
test('T4: head logs chore pending then approves', async ({ page, browser }) => {
  await registerUser(page, 'Head T4', uniqueEmail());
  await createGroup(page, `LedgerFam-${Date.now()}`);

  const token = await inviteAndCaptureToken(page);
  const { ctx: memberCtx } = await joinGroupAsMember(browser, token, 'Member T4');

  try {
    // Head: open member detail for the first member card.
    const memberCard = page.getByTestId(/^member-card-/).first();
    await expect(memberCard).toBeVisible({ timeout: 10_000 });
    await memberCard.click();
    await expect(page.getByTestId('member-detail-root')).toBeVisible({ timeout: 10_000 });

    // Head: tap "Add entry" — opens entry sheet.
    await page.getByTestId('member-add-entry-button').click();

    // Fill amount in the sheet (TextInput with testID or inputmode=numeric).
    const amountInput = page.locator('[data-testid="entry-amount"], input[inputmode="numeric"]').first();
    await expect(amountInput).toBeVisible({ timeout: 8_000 });
    await amountInput.fill('200');

    // Submit the entry form.
    await page.getByRole('button', { name: /save|submit|add/i }).last().click();

    // The ledger row for the new entry should show an approve button (pending).
    const approveBtn = page.getByTestId(/^ledger-approve-/).first();
    await expect(approveBtn).toBeVisible({ timeout: 15_000 });

    // Head approves — button should disappear.
    await approveBtn.click();
    await expect(approveBtn).not.toBeVisible({ timeout: 10_000 });
  } finally {
    await memberCtx.close();
  }
});

// T4-REJECT: Head rejects a pending entry — reject button disappears.
test('T4-REJECT: head rejects a pending chore entry', async ({ page, browser }) => {
  await registerUser(page, 'Head T4R', uniqueEmail());
  await createGroup(page, `RejectFam-${Date.now()}`);

  const token = await inviteAndCaptureToken(page);
  const { ctx: memberCtx } = await joinGroupAsMember(browser, token, 'Member T4R');

  try {
    const memberCard = page.getByTestId(/^member-card-/).first();
    await expect(memberCard).toBeVisible({ timeout: 10_000 });
    await memberCard.click();
    await expect(page.getByTestId('member-detail-root')).toBeVisible({ timeout: 10_000 });
    await page.getByTestId('member-add-entry-button').click();

    const amountInput = page.locator('[data-testid="entry-amount"], input[inputmode="numeric"]').first();
    await expect(amountInput).toBeVisible({ timeout: 8_000 });
    await amountInput.fill('300');
    await page.getByRole('button', { name: /save|submit|add/i }).last().click();

    const rejectBtn = page.getByTestId(/^ledger-reject-/).first();
    await expect(rejectBtn).toBeVisible({ timeout: 15_000 });
    await rejectBtn.click();
    await expect(rejectBtn).not.toBeVisible({ timeout: 10_000 });
  } finally {
    await memberCtx.close();
  }
});
