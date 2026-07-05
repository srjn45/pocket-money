import { test, expect, type Browser, type BrowserContext, type Page } from '@playwright/test';
import {
  uniqueEmail,
  registerUser,
  createGroup,
  inviteAndCaptureToken,
  openTab,
  requestLoan,
  approveFirstLoan,
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

// T6: member requests a loan → head approves it (via the approve sheet) → the
// loan becomes active; the member can see it but has no approve control.
test('T6: member requests loan, head approves, loan active', async ({ page, browser }) => {
  await registerUser(page, 'Head T6', uniqueEmail());
  await createGroup(page, `LoanFam-${Date.now()}`);

  const token = await inviteAndCaptureToken(page);
  const { ctx: memberCtx, page: memberPage } = await joinGroupAsMember(browser, token, 'Member T6');

  try {
    // Member: request a loan (₹6000 over 3 months).
    await requestLoan(memberPage, '6000', '3');
    await expect(memberPage.getByTestId(/^loan-card-/).first()).toBeVisible({ timeout: 15_000 });

    // Head: navigate to the loans tab and approve the requested loan.
    await approveFirstLoan(page);

    // Member: reload and reopen the loans tab → loan card present, no approve
    // control (members cannot approve their own loan).
    await memberPage.reload();
    await openTab(memberPage, 'loans');
    await expect(memberPage.getByTestId(/^loan-card-/).first()).toBeVisible({ timeout: 15_000 });
    await expect(memberPage.getByTestId(/^loan-approve-/)).toHaveCount(0);
  } finally {
    await memberCtx.close();
  }
});

// T6-EMPTY: a brand-new group shows the empty state on the loans tab.
test('T6-EMPTY: new group has empty loans state', async ({ page }) => {
  await registerUser(page, 'Head Loans Empty', uniqueEmail());
  await createGroup(page, `EmptyLoans-${Date.now()}`);

  await openTab(page, 'loans');
  await expect(page.getByTestId('loans-empty')).toBeVisible({ timeout: 10_000 });
});
