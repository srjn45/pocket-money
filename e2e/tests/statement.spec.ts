import { test, expect } from '@playwright/test';
import {
  uniqueEmail,
  registerUser,
  createGroup,
  addMemberByEmail,
  addMemberAndClaim,
  groupIdFromUrl,
  reloadInto,
  recordPayment,
} from '../support/pages';
import { apiLogin, apiCreateChore } from '../support/api';

// T-STMT-ADMIN: the §8 acceptance flow — admin creates a group, adds a member,
// posts a chore (payable > 0), then records the pre-filled payment and the row's
// remaining recomputes to ₹0.00 (statement invalidation → refetch).
test('T-STMT-ADMIN: add chore → payable > 0 → record payment → remaining ₹0.00', async ({ page }) => {
  const headEmail = uniqueEmail();
  const headPass = 'headpass123!';
  await registerUser(page, 'Head STMT', headEmail, headPass);
  await createGroup(page, `StatementFam-${Date.now()}`); // INR
  const groupId = groupIdFromUrl(page);

  // Seed an earnable chore (₹20.00) via the API so the admin can post it.
  const { token: headToken } = await apiLogin(headEmail, headPass);
  await apiCreateChore(headToken, groupId, 'Wash dishes', 2000);

  // Add a member by email (shadow); the statement row carries their figures.
  await addMemberByEmail(page, uniqueEmail(), 'Member STMT');

  // Reload so the chores + statement queries are fresh (chore was created out-of-band).
  await reloadInto(page, 'group-overview-root');

  const card = page.getByTestId(/^member-card-/).first();
  await expect(card).toBeVisible({ timeout: 10_000 });
  const tid = await card.getAttribute('data-testid');
  const uid = tid!.replace('member-card-', '');

  // Post the chore for the member via the statement Add-entry sheet (head → auto-approved).
  await page.getByTestId('statement-add-entry').click();
  const chorePicker = page.getByTestId('entry-chore-picker');
  await expect(chorePicker).toBeVisible({ timeout: 10_000 });
  await expect(chorePicker.locator('option')).not.toHaveCount(0, { timeout: 10_000 });
  await chorePicker.selectOption({ index: 0 });
  await page.getByTestId('entry-submit').click();
  await expect(page.getByTestId('toast-root')).toBeVisible({ timeout: 10_000 });

  // Payable reflects the chore credit (invalidation refetches the open statement).
  await expect(page.getByTestId(`statement-payable-${uid}`)).toHaveText('₹20.00', { timeout: 15_000 });

  // Record the pre-filled payment → remaining recomputes to zero.
  await recordPayment(page, uid);
  await expect(page.getByTestId(`statement-remaining-${uid}`)).toHaveText('₹0.00', { timeout: 15_000 });
});

// T-STMT-MEMBER: the member sees a read-only own-row statement — a receive
// banner, exactly their own row, and no Record-payment control (D6).
test('T-STMT-MEMBER: member sees receive banner, own row only, no payment button', async ({ page, browser }) => {
  const headEmail = uniqueEmail();
  await registerUser(page, 'Head STMT-M', headEmail);
  await createGroup(page, `StatementMemberFam-${Date.now()}`);
  const groupId = groupIdFromUrl(page);

  const { ctx: memberCtx, page: memberPage } = await addMemberAndClaim(page, browser, groupId, 'Member STMT-M');

  try {
    // The member's own statement: receive banner + exactly one row + no payment button.
    await expect(memberPage.getByTestId('statement-receive-banner')).toBeVisible({ timeout: 15_000 });
    await expect(memberPage.getByTestId(/^member-card-/)).toHaveCount(1);
    await expect(memberPage.getByTestId(/^statement-record-payment-/)).toHaveCount(0);
  } finally {
    await memberCtx.close();
  }
});
