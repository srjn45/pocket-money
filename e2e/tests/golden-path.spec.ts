import { test, expect, type BrowserContext } from '@playwright/test';
import {
  uniqueEmail,
  registerUser,
  createGroup,
  addMemberByEmail,
  groupIdFromUrl,
  reloadInto,
  recordPayment,
  openTab,
  goToRegister,
  fillRegisterForm,
  submitRegister,
} from '../support/pages';
import { apiLogin, apiCreateChore, apiCreateLoan } from '../support/api';

// The documented v3 golden path (master-plan-v3 §11). This file supersedes and
// absorbs onboarding T2 / T2-EUR / T3, statement T-STMT-ADMIN / T-STMT-MEMBER, and
// ledger T5. Each test is fully self-provisioning (no seed dependency), so the file
// runs under fullyParallel. Flake-hardening (V3-4.4 §4): AuthGate claim awaits
// dashboard-root before any group nav; group re-entry is by full URL (expo-router
// [id] discipline); drill-in text assertions are scoped to the active screen root
// (native-Stack hidden-dup guard); success is always an exact final figure or a
// count, never a transient PostDue state.

// Test A — the full INR spine: add-by-email → shadow → base + chore + loan →
// month statement → record payment → claim → member sees own row only.
//
// Payable arithmetic (§1.3): the ₹500 base (current-month allowance, posts now) +
// ₹20 chore = ₹520.00. The admin-created active loan's start_period is *next*
// calendar month (backend CreateLoan → loanNextPeriod), so its first EMI does NOT
// post in the current statement. We therefore lock the exact, stable ₹520.00 (§1.3
// fallback) and assert the loan leg exists separately via its card on the loans tab.
test('golden path (INR): add-by-email → shadow → base + chore + loan → month statement → record payment → claim → member sees own row only', async ({
  page,
  browser,
}) => {
  // 1. Admin registers and auto-logs-in.
  const adminEmail = uniqueEmail();
  const adminPass = 'headpass123!';
  await registerUser(page, 'Admin GP', adminEmail, adminPass);
  expect(page.url()).not.toContain('/login');

  // 2. Create an INR group (default currency); invite path hidden (D8).
  await createGroup(page, `GP-${Date.now()}`);
  await expect(page.getByTestId('group-overview-root')).toBeVisible();
  await expect(page.getByTestId('group-add-member-button')).toBeVisible();
  await expect(page.getByTestId('group-invite-button')).toHaveCount(0);

  // 3. Capture the group id.
  const gid = groupIdFromUrl(page);

  // 4. Seed a ₹20 chore via the API (pure setup — the earnable exists to post).
  const { token } = await apiLogin(adminEmail, adminPass);
  await apiCreateChore(token, gid, 'Dishes', 2000);

  // 5. Add a member by email → shadow member row appears.
  const memberEmail = uniqueEmail();
  await addMemberByEmail(page, memberEmail, 'Kid GP');

  // 6. Resolve the member uid from the first member card.
  const memberCard = page.getByTestId(/^member-card-/).first();
  await expect(memberCard).toBeVisible({ timeout: 10_000 });
  const tid = await memberCard.getAttribute('data-testid');
  const uid = tid!.replace('member-card-', '');

  // 5b. The shadow badge renders on the member's statement row (state change).
  await expect(page.getByTestId(`member-shadow-badge-${uid}`)).toBeVisible({ timeout: 10_000 });

  // 7. Base leg — set a ₹500 base allowance via the UI. Setting a current-month
  //    allowance posts the credit immediately, so ₹500.00 renders on the summary.
  //    Scope to member-detail-root: the same figure also appears on the still-
  //    mounted (display:none) group-overview screen underneath the pushed Stack
  //    screen, so an unscoped .first() would resolve the stale hidden copy (§4.3).
  await page.getByTestId(`member-card-${uid}`).click();
  await expect(page.getByTestId('member-detail-root')).toBeVisible({ timeout: 10_000 });
  await page.getByTestId('member-allowance-button').click();
  await page.getByPlaceholder('e.g. 500').fill('500');
  await page.getByRole('button', { name: /^save$/i }).click();
  await expect(page.getByTestId('toast-root')).toBeVisible({ timeout: 10_000 });
  await expect(
    page.getByTestId('member-detail-root').getByText(/₹500\.00/).first(),
  ).toBeVisible({ timeout: 10_000 });

  // 8. Loan leg — admin creates a pre-approved active loan (₹1200 over 6 → EMI
  //    ₹200/mo). Its first EMI posts next month, so it is out of this statement's
  //    arithmetic; its existence is asserted via the loan card at step 12b.
  const loan = await apiCreateLoan(token, gid, uid, 120000, 6);

  // 9. Chore leg — post the seeded chore from the statement. Full-URL nav back to
  //    the overview (expo-router [id] discipline) forces a fresh chores fetch so
  //    the picker populates.
  await page.goto(`/groups/${gid}`);
  await expect(page.getByTestId('group-overview-root')).toBeVisible({ timeout: 15_000 });
  await page.getByTestId('statement-add-entry').click();
  const chorePicker = page.getByTestId('entry-chore-picker');
  await expect(chorePicker).toBeVisible({ timeout: 10_000 });
  await expect(chorePicker.locator('option')).not.toHaveCount(0, { timeout: 10_000 });
  await chorePicker.selectOption({ index: 0 });
  await page.getByTestId('entry-submit').click();
  await expect(page.getByTestId('toast-root')).toBeVisible({ timeout: 10_000 });

  // 10. Month statement payable = base 500 + chore 20 (EMI is next-month, §1.3).
  await expect(page.getByTestId(`statement-payable-${uid}`)).toHaveText('₹520.00', { timeout: 15_000 });

  // 11. Record the pre-filled payment → the row's remaining recomputes to zero.
  await recordPayment(page, uid);
  await expect(page.getByTestId(`statement-remaining-${uid}`)).toHaveText('₹0.00', { timeout: 15_000 });

  // 12b. The loan leg exists: its card renders on the loans tab (full-URL nav).
  await openTab(page, 'loans');
  await expect(page.getByTestId('loans-root')).toBeVisible({ timeout: 10_000 });
  await expect(page.getByTestId(`loan-card-${loan.id}`)).toBeVisible({ timeout: 15_000 });

  // 13. Claim — the member registers with the shadow email in a fresh context.
  //     Await dashboard-root BEFORE any group nav (AuthGate claim race, §4.1).
  const memberCtx: BrowserContext = await browser.newContext();
  const memberPage = await memberCtx.newPage();
  try {
    await goToRegister(memberPage);
    await fillRegisterForm(memberPage, 'Kid GP', memberEmail, 'member123!');
    await submitRegister(memberPage);
    await expect(memberPage.getByTestId('dashboard-root')).toBeVisible({ timeout: 20_000 });

    // 14. Member opens the group by URL.
    await memberPage.goto(`/groups/${gid}`);
    await expect(memberPage.getByTestId('group-overview-root')).toBeVisible({ timeout: 20_000 });

    // 15. Member-scoped read (D6): the scoping invariant — exactly one row (own),
    //     no record-payment control, a receive banner, leave visible, invite gone.
    //     Not a specific banner amount: step 11 already zeroed the remaining, so the
    //     figure is period-dependent but the scoping is not — that is the real intent.
    await expect(memberPage.getByTestId('statement-receive-banner')).toBeVisible({ timeout: 15_000 });
    await expect(memberPage.getByTestId(/^member-card-/)).toHaveCount(1);
    await expect(memberPage.getByTestId(/^statement-record-payment-/)).toHaveCount(0);
    await expect(memberPage.getByTestId('group-leave-button')).toBeVisible();
    await expect(memberPage.getByTestId('group-invite-button')).toHaveCount(0);
  } finally {
    await memberCtx.close();
  }
});

// Test B — EUR currency isolation (D7): the picked currency flows through the
// dashboard and a live posted chore, never the hardcoded ₹.
test('golden path (EUR currency): create EUR group → € dashboard balance → chore posts in €, never ₹', async ({
  page,
}) => {
  // 1. Register + create an EUR group (drives create-group-currency-EUR).
  const adminEmail = uniqueEmail();
  const adminPass = 'headpass123!';
  await registerUser(page, 'Euro Head', adminEmail, adminPass);
  const groupName = `EUR-${Date.now()}`;
  await createGroup(page, groupName, 'EUR');
  await expect(page.getByTestId('group-overview-root')).toBeVisible();
  const gid = groupIdFromUrl(page);

  // 2. Dashboard shows the group balance in € (picked currency flows through).
  await page.goto('/');
  await expect(page.getByTestId('dashboard-root')).toBeVisible({ timeout: 15_000 });
  await expect(page.getByText(groupName)).toBeVisible({ timeout: 10_000 });
  await expect(page.getByText(/€0\.00/).first()).toBeVisible({ timeout: 10_000 });

  // 3. Seed a €12.50 chore + add a member by email.
  const { token } = await apiLogin(adminEmail, adminPass);
  await apiCreateChore(token, gid, 'Gelato', 1250, 'EUR');
  // Navigate back into the group overview (we left it for the dashboard € check).
  await page.goto(`/groups/${gid}`);
  await expect(page.getByTestId('group-overview-root')).toBeVisible({ timeout: 15_000 });
  await addMemberByEmail(page, uniqueEmail(), 'Euro Kid');

  // 4. Post the chore from the statement → payable renders as € (never ₹).
  await reloadInto(page, 'group-overview-root');
  const card = page.getByTestId(/^member-card-/).first();
  await expect(card).toBeVisible({ timeout: 10_000 });
  const tid = await card.getAttribute('data-testid');
  const uid = tid!.replace('member-card-', '');

  await page.getByTestId('statement-add-entry').click();
  const chorePicker = page.getByTestId('entry-chore-picker');
  await expect(chorePicker).toBeVisible({ timeout: 10_000 });
  await expect(chorePicker.locator('option')).not.toHaveCount(0, { timeout: 10_000 });
  await chorePicker.selectOption({ index: 0 });
  await page.getByTestId('entry-submit').click();
  await expect(page.getByTestId('toast-root')).toBeVisible({ timeout: 10_000 });

  await expect(page.getByTestId(`statement-payable-${uid}`)).toHaveText('€12.50', { timeout: 15_000 });
});
