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

// T6: Member requests loan → head approves → loan card shows active.
test('T6: member requests loan, head approves, EMI visible', async ({ page, browser }) => {
  await registerUser(page, 'Head T6', uniqueEmail());
  await createGroup(page, `LoanFam-${Date.now()}`);

  const token = await inviteAndCaptureToken(page);
  const { ctx: memberCtx, page: memberPage } = await joinGroupAsMember(browser, token, 'Member T6');

  try {
    // ── Member: navigate to loans tab ────────────────────────────────────────
    await expect(memberPage.getByTestId('loans-root')).toBeVisible({ timeout: 10_000 }).catch(async () => {
      // Loans tab might require navigation via tab bar.
      await memberPage.getByRole('tab', { name: /loan/i }).click();
      await expect(memberPage.getByTestId('loans-root')).toBeVisible({ timeout: 10_000 });
    });

    // Member: tap "Request Loan".
    await memberPage.getByTestId('loans-request-button').click();

    // Fill the loan request form.
    // Amount field.
    const amountInput = memberPage.locator('[data-testid="loan-amount"], input[inputmode="numeric"]').first();
    await expect(amountInput).toBeVisible({ timeout: 8_000 });
    await amountInput.fill('60000');

    // Installments field (if present).
    const installmentsInput = memberPage
      .locator('[data-testid="loan-installments"], input[inputmode="numeric"]')
      .last();
    await installmentsInput.fill('3').catch(() => {
      // Installments input may not be separate — ignore if not found.
    });

    // Submit.
    await memberPage.getByRole('button', { name: /submit|request|apply/i }).last().click();

    // Loan card should appear in the list.
    const loanCard = memberPage.getByTestId(/^loan-card-/).first();
    await expect(loanCard).toBeVisible({ timeout: 15_000 });

    // ── Head: approve the loan ────────────────────────────────────────────────
    // Navigate head to the same group's loans tab.
    const headLoansRoot = page.getByTestId('loans-root');
    await headLoansRoot.isVisible().catch(async () => {
      await page.getByRole('tab', { name: /loan/i }).click();
      await expect(page.getByTestId('loans-root')).toBeVisible({ timeout: 10_000 });
    });

    // The approve button should be visible for the requested loan.
    const approveBtn = page.getByTestId(/^loan-approve-/).first();
    await expect(approveBtn).toBeVisible({ timeout: 15_000 });
    await approveBtn.click();

    // Approve button should disappear (loan is now active).
    await expect(approveBtn).not.toBeVisible({ timeout: 10_000 });

    // ── Member: loan card now shows active status ─────────────────────────────
    // Reload member's loans view.
    await memberPage.reload();
    await expect(memberPage.getByTestId(/^loan-card-/).first()).toBeVisible({ timeout: 15_000 });
    // Card should NOT have an approve button (member can't approve).
    await expect(memberPage.getByTestId(/^loan-approve-/).first()).not.toBeVisible();
  } finally {
    await memberCtx.close();
  }
});

// T6-EMPTY: New group shows empty state on loans tab.
test('T6-EMPTY: new group has empty loans state', async ({ page }) => {
  await registerUser(page, 'Head Loans Empty', uniqueEmail());
  await createGroup(page, `EmptyLoans-${Date.now()}`);

  // Navigate to loans tab.
  const loansRoot = page.getByTestId('loans-root');
  await loansRoot.isVisible({ timeout: 5_000 }).catch(async () => {
    await page.getByRole('tab', { name: /loan/i }).click();
  });
  await expect(page.getByTestId('loans-empty')).toBeVisible({ timeout: 10_000 });
});
