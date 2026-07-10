import { test, expect } from '@playwright/test';
import {
  uniqueEmail,
  registerUser,
  createGroup,
  addMemberAndClaim,
  groupIdFromUrl,
  reloadInto,
  autoAcceptDialogs,
  memberLogChore,
  approveFirstPending,
  rejectFirstPending,
} from '../support/pages';
import { apiLogin, apiCreateChore } from '../support/api';

// T4: member logs chores (pending) → head approves one and rejects the other.
// Head-created entries auto-approve, so a *pending* entry (with approve/reject
// controls) can only come from a member submission — that is the flow under test.
test('T4: member logs chores; head approves one and rejects another', async ({ page, browser }) => {
  const headEmail = uniqueEmail();
  const headPass = 'headpass123!';
  await registerUser(page, 'Head T4', headEmail, headPass);
  await createGroup(page, `LedgerFam-${Date.now()}`);
  const groupId = groupIdFromUrl(page);

  // Seed an earnable chore via the API so the member has something to log.
  const { token: headToken } = await apiLogin(headEmail, headPass);
  await apiCreateChore(headToken, groupId, 'Wash dishes', 2000);

  const { ctx: memberCtx, page: memberPage } = await addMemberAndClaim(page, browser, groupId, 'Member T4');
  autoAcceptDialogs(page); // reject shows a window.confirm() on web

  try {
    // Member logs two chores — both land as pending approvals.
    await memberLogChore(memberPage);
    await expect(memberPage.getByTestId('toast-root')).toBeVisible({ timeout: 10_000 });
    await memberLogChore(memberPage);
    await expect(memberPage.getByTestId('toast-root')).toBeVisible({ timeout: 10_000 });

    // Head: reload to see the just-joined member (react-query staleTime), open detail.
    await reloadInto(page, 'group-overview-root');
    const memberCard = page.getByTestId(/^member-card-/).first();
    await expect(memberCard).toBeVisible({ timeout: 10_000 });
    await memberCard.click();
    await expect(page.getByTestId('member-detail-root')).toBeVisible({ timeout: 10_000 });

    // Two pending rows are present → approve one, reject the other.
    await approveFirstPending(page);
    await rejectFirstPending(page);
  } finally {
    await memberCtx.close();
  }
});

// T5: head sets a member's allowance and the value renders on the member summary.
test('T5: head sets member allowance and it renders on the summary', async ({ page, browser }) => {
  const headEmail = uniqueEmail();
  await registerUser(page, 'Head T5', headEmail);
  await createGroup(page, `AllowanceFam-${Date.now()}`);

  const { ctx: memberCtx } = await addMemberAndClaim(page, browser, groupIdFromUrl(page), 'Member T5');

  try {
    await reloadInto(page, 'group-overview-root');
    const memberCard = page.getByTestId(/^member-card-/).first();
    await expect(memberCard).toBeVisible({ timeout: 10_000 });
    await memberCard.click();
    await expect(page.getByTestId('member-detail-root')).toBeVisible({ timeout: 10_000 });

    // Open the allowance sheet (AllowanceSummary "Set"/"Edit" button).
    await page.getByTestId('member-allowance-button').click();

    // Set ₹300/month and save.
    await page.getByPlaceholder('e.g. 500').fill('300');
    await page.getByRole('button', { name: /^save$/i }).click();

    // Success toast, then the amount renders on the member summary in the group's
    // currency (INR default) — the ₹ prefix proves currency-aware formatting, and
    // 300.00 can only be the allowance (balance is ₹0).
    await expect(page.getByTestId('toast-root')).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText(/₹300\.00/).first()).toBeVisible({ timeout: 10_000 });
  } finally {
    await memberCtx.close();
  }
});
