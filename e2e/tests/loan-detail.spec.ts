// V3-4.3 (§2, D5): loan card shows repaid/pending; loan detail shows the derived
// repayment schedule. Full-UI flow (no seed dependency): member requests a loan,
// head approves it (→ active), then the head opens the loan detail and asserts the
// progress line + one schedule row per installment.
import { test, expect } from '@playwright/test';
import {
  uniqueEmail,
  registerUser,
  createGroup,
  addMemberAndClaim,
  groupIdFromUrl,
  openTab,
  requestLoan,
  approveFirstLoan,
  openLoanDetail,
} from '../support/pages';

test('loan detail: card progress + repayment schedule', async ({ page, browser }) => {
  await registerUser(page, 'Head LoanDetail', uniqueEmail());
  await createGroup(page, `LoanDetailFam-${Date.now()}`);
  const gid = groupIdFromUrl(page);

  const { ctx: memberCtx, page: memberPage } = await addMemberAndClaim(page, browser, gid, 'Member LoanDetail');

  try {
    // Member requests ₹6000 over 3 months; head approves → active loan.
    await requestLoan(memberPage, '6000', '3');
    await expect(memberPage.getByTestId(/^loan-card-/).first()).toBeVisible({ timeout: 15_000 });
    await approveFirstLoan(page);

    // Head: on the loans tab, the active loan card shows the repaid/pending line.
    await openTab(page, 'loans');
    await expect(page.getByTestId('loans-root')).toBeVisible({ timeout: 10_000 });
    const cardTid = await page.getByTestId(/^loan-card-/).first().getAttribute('data-testid');
    const loanId = cardTid!.replace('loan-card-', '');
    await expect(page.getByTestId(`loan-progress-${loanId}`)).toBeVisible({ timeout: 15_000 });

    // Tap into the detail: the schedule renders one row per installment (3).
    await openLoanDetail(page, loanId);
    await expect(page.getByTestId('loan-schedule')).toBeVisible({ timeout: 15_000 });
    await expect(page.getByTestId(`loan-installment-${loanId}-1`)).toBeVisible({ timeout: 15_000 });
    await expect(page.getByTestId(new RegExp(`^loan-installment-${loanId}-`))).toHaveCount(3);
  } finally {
    await memberCtx.close();
  }
});
