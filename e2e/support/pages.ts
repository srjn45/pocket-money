// Page-object helpers keyed on testIDs (§4.2).
// All selectors use data-testid (from RN testID prop). No CSS/nth/XPath.
import { type Page, type Browser, type BrowserContext, expect } from '@playwright/test';

// ─── helpers ────────────────────────────────────────────────────────────────

let _runSeq = 0;

/** Unique e-mail per test run, no unique-constraint collisions on re-run. */
export function uniqueEmail(): string {
  return `e2e+${Date.now()}-${++_runSeq}@demo.test`;
}

// ─── Auth helpers ────────────────────────────────────────────────────────────

export async function fillLoginForm(page: Page, email: string, password: string): Promise<void> {
  await page.getByTestId('login-email').fill(email);
  await page.getByTestId('login-password').fill(password);
}

export async function submitLogin(page: Page): Promise<void> {
  await page.getByTestId('login-submit').click();
}

export async function login(page: Page, email: string, password: string): Promise<void> {
  await goToLogin(page);
  await fillLoginForm(page, email, password);
  await submitLogin(page);
  await expect(page.getByTestId('dashboard-root')).toBeVisible({ timeout: 15_000 });
}

export async function fillRegisterForm(
  page: Page,
  name: string,
  email: string,
  password: string,
): Promise<void> {
  await page.getByTestId('register-name').fill(name);
  await page.getByTestId('register-email').fill(email);
  await page.getByTestId('register-password').fill(password);
  await page.getByTestId('register-confirm').fill(password);
}

export async function submitRegister(page: Page): Promise<void> {
  await page.getByTestId('register-submit').click();
}

/** Navigate to the login page and wait for the form to be ready. */
export async function goToLogin(page: Page): Promise<void> {
  await page.goto('/');
  await expect(page.getByTestId('login-submit')).toBeVisible({ timeout: 15_000 });
}

/** Navigate to the register page via the login link. */
export async function goToRegister(page: Page): Promise<void> {
  await goToLogin(page);
  await page.getByTestId('login-link-register').click();
  await expect(page.getByTestId('register-submit')).toBeVisible({ timeout: 10_000 });
}

/** Register a fresh user and assert they land on the dashboard. */
export async function registerUser(
  page: Page,
  name: string,
  email: string,
  password = 'password123',
): Promise<void> {
  await goToRegister(page);
  await fillRegisterForm(page, name, email, password);
  await submitRegister(page);
  await expect(page.getByTestId('dashboard-root')).toBeVisible({ timeout: 15_000 });
}

// ─── Group helpers ───────────────────────────────────────────────────────────

export type Currency = 'EUR' | 'USD' | 'INR';

/**
 * Create a group. `currency` picks a code via the create-group currency picker
 * (defaults to INR, which is also the screen's default selection). currency is
 * REQUIRED on POST /groups now, so a group is always created in a known currency.
 */
export async function createGroup(
  page: Page,
  groupName: string,
  currency: Currency = 'INR',
): Promise<void> {
  await page.getByTestId('dashboard-create-group').click();
  await expect(page.getByTestId('create-group-name')).toBeVisible();
  await page.getByTestId('create-group-name').fill(groupName);
  await page.getByTestId(`create-group-currency-${currency}`).click();
  await page.getByTestId('create-group-submit').click();
  await expect(page.getByTestId('group-overview-root')).toBeVisible({ timeout: 15_000 });
}

/** Admin (on the group overview) adds a member by email via the add-member sheet.
 *  Asserts the state change: sheet closes AND the member row appears (§9.12). */
export async function addMemberByEmail(adminPage: Page, email: string, name: string): Promise<void> {
  await adminPage.getByTestId('group-add-member-button').click();
  await expect(adminPage.getByTestId('add-member-email')).toBeVisible({ timeout: 10_000 });
  await adminPage.getByTestId('add-member-email').fill(email);
  await adminPage.getByTestId('add-member-name').fill(name);
  await adminPage.getByTestId('add-member-submit').click();
  await expect(adminPage.getByTestId('add-member-email')).toHaveCount(0, { timeout: 10_000 }); // sheet closed
  await expect(adminPage.getByText(name).first()).toBeVisible({ timeout: 10_000 });            // row appeared
}

/** Full add-by-email → claim onboarding (replaces invite-link joinGroupAsMember).
 *  Admin (adminPage, already inside the group) adds `name`'s email → a fresh
 *  context registers with that email (claims the shadow) → opens the group. */
export async function addMemberAndClaim(
  adminPage: Page,
  browser: Browser,
  groupId: string,
  name: string,
  password = 'member123!',
): Promise<{ ctx: BrowserContext; page: Page; email: string }> {
  const email = uniqueEmail();
  await addMemberByEmail(adminPage, email, name);

  const ctx = await browser.newContext();
  const pg = await ctx.newPage();
  await goToRegister(pg);                              // existing helper
  await fillRegisterForm(pg, name, email, password);   // existing helper
  await submitRegister(pg);                            // existing helper
  await expect(pg.getByTestId('dashboard-root')).toBeVisible({ timeout: 20_000 }); // claim → auto-login → dashboard
  await pg.goto(`/groups/${groupId}`);                 // full-URL nav (§9.12); baseURL from config
  await expect(pg.getByTestId('group-overview-root')).toBeVisible({ timeout: 20_000 });
  return { ctx, page: pg, email };
}

// ─── Toast helper ────────────────────────────────────────────────────────────

export async function waitForToast(page: Page): Promise<void> {
  await expect(page.getByTestId('toast-root')).toBeVisible({ timeout: 10_000 });
}

// ─── Session helpers ─────────────────────────────────────────────────────────

/** Create a fresh browser context (isolated storage — separate "user"). */
export async function freshContext(
  context: BrowserContext,
): Promise<{ ctx: BrowserContext; page: Page }> {
  const ctx = await context.browser()!.newContext();
  const page = await ctx.newPage();
  return { ctx, page };
}

// ─── Navigation / tab helpers ─────────────────────────────────────────────────

/** Extract the group id from a /groups/<id> URL. */
export function groupIdFromUrl(page: Page): string {
  const m = page.url().match(/\/groups\/([0-9a-f-]{36})/i);
  if (!m) throw new Error(`no group id in url: ${page.url()}`);
  return m[1];
}

/**
 * Navigate to a tab.
 *
 * The group tabs (loans/chores) are reached by full-URL navigation, not by
 * clicking the tab link: on web, switching to a sibling tab via the link does
 * not populate the parent [id] dynamic segment in the child screen's
 * useLocalSearchParams, so its group-scoped queries fetch with an empty id
 * (loans list comes back empty, a loan request POSTs to /groups//loans). A full
 * navigation parses [id] from the URL reliably. The app-level Profile tab has no
 * dynamic segment, so a tab-link click is fine there.
 */
export async function openTab(page: Page, suffix: 'loans' | 'chores' | 'profile'): Promise<void> {
  if (suffix === 'profile') {
    await page.locator(`a[role="tab"][href$="/profile"]`).click();
    return;
  }
  const gid = groupIdFromUrl(page);
  await page.goto(`/groups/${gid}/${suffix}`);
}

/**
 * Reload the current page and wait for a testID to reappear. React-query has a
 * 30s staleTime, so a page that already loaded stale data (e.g. a head whose
 * member list was empty when a member later joined in another context) will not
 * refetch on its own — a reload forces a fresh fetch. Auth persists in
 * AsyncStorage (localStorage on web) across the reload.
 */
export async function reloadInto(page: Page, testId: string): Promise<void> {
  await page.reload();
  await expect(page.getByTestId(testId)).toBeVisible({ timeout: 20_000 });
}

/** Auto-accept the browser's native confirm() dialogs (leave/remove use window.confirm). */
export function autoAcceptDialogs(page: Page): void {
  page.on('dialog', (d) => d.accept().catch(() => {}));
}

// ─── Ledger helpers ───────────────────────────────────────────────────────────

/** Member logs a chore (member Overview → "Log a chore" → pick first chore → Save). */
export async function memberLogChore(page: Page): Promise<void> {
  await page.getByTestId('member-log-chore').click();
  const picker = page.getByTestId('entry-chore-picker');
  await expect(picker).toBeVisible({ timeout: 10_000 });
  // Wait for chore options to populate, then pick the first (sets selectedChoreId).
  await expect(picker.locator('option')).not.toHaveCount(0, { timeout: 10_000 });
  await picker.selectOption({ index: 0 });
  await page.getByTestId('entry-submit').click();
}

/**
 * Head adds a settlement/adjustment entry from a member's detail screen
 * (member-detail add-entry is head mode with a fixed user → auto-approved).
 */
export async function headAddEntry(
  page: Page,
  kind: 'settlement' | 'adjustment',
  amount: string,
  direction: 'credit' | 'debit' = 'credit',
): Promise<void> {
  await page.getByTestId('member-add-entry-button').click();
  await page.getByTestId('entry-type-picker').selectOption(kind);
  await page.getByTestId('entry-amount').fill(amount);
  if (kind === 'adjustment') {
    await page.getByTestId('entry-direction-picker').selectOption(direction);
  }
  await page.getByTestId('entry-submit').click();
}

/**
 * Approve the first pending ledger row. Captures that row's exact testID before
 * clicking so the "disappeared" assertion is not confused by other pending rows
 * that still carry an approve control.
 */
export async function approveFirstPending(page: Page): Promise<void> {
  const btn = page.getByTestId(/^ledger-approve-/).first();
  await expect(btn).toBeVisible({ timeout: 15_000 });
  const tid = await btn.getAttribute('data-testid');
  await btn.click();
  await expect(page.getByTestId(tid!)).toHaveCount(0, { timeout: 10_000 });
}

/** Reject the first pending ledger row (same exact-testID capture as approve). */
export async function rejectFirstPending(page: Page): Promise<void> {
  const btn = page.getByTestId(/^ledger-reject-/).first();
  await expect(btn).toBeVisible({ timeout: 15_000 });
  const tid = await btn.getAttribute('data-testid');
  await btn.click();
  await expect(page.getByTestId(tid!)).toHaveCount(0, { timeout: 10_000 });
}

// ─── Statement helpers (V3-4.2) ───────────────────────────────────────────────

/**
 * Admin records a payment for a member on the statement screen: opens the
 * per-row Record-payment sheet (specific uid, or the first row), asserts the
 * amount is pre-filled (visible), submits, and awaits the success toast.
 */
export async function recordPayment(page: Page, uid?: string): Promise<void> {
  const btn = uid
    ? page.getByTestId(`statement-record-payment-${uid}`)
    : page.getByTestId(/^statement-record-payment-/).first();
  await expect(btn).toBeVisible({ timeout: 10_000 });
  await btn.click();
  await expect(page.getByTestId('record-payment-amount')).toBeVisible({ timeout: 10_000 });
  await page.getByTestId('record-payment-submit').click();
  await expect(page.getByTestId('toast-root')).toBeVisible({ timeout: 10_000 });
}

/** Navigate the statement month switcher one step (prev/next). */
export async function openStatementMonth(page: Page, dir: 'prev' | 'next'): Promise<void> {
  await page.getByTestId(`statement-${dir}-month`).click();
}

// ─── Loan helpers ─────────────────────────────────────────────────────────────

export async function requestLoan(
  page: Page,
  amount: string,
  installments: string,
): Promise<void> {
  await openTab(page, 'loans');
  await expect(page.getByTestId('loans-root')).toBeVisible({ timeout: 10_000 });
  await page.getByTestId('loans-request-button').click();
  await page.getByTestId('loan-amount').fill(amount);
  await page.getByTestId('loan-installments').fill(installments);
  await page.getByTestId('loan-request-submit').click();
}

/** Head approves the first requested loan (opens the approve sheet, confirms). */
export async function approveFirstLoan(page: Page): Promise<void> {
  await openTab(page, 'loans');
  await expect(page.getByTestId('loans-root')).toBeVisible({ timeout: 10_000 });
  const approve = page.getByTestId(/^loan-approve-/).first();
  await expect(approve).toBeVisible({ timeout: 15_000 });
  await approve.click();
  await page.getByTestId('loan-approve-submit').click();
  await expect(page.getByTestId('loan-approve-submit')).not.toBeVisible({ timeout: 10_000 });
}
