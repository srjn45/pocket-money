// Page-object helpers keyed on testIDs (§4.2).
// All selectors use data-testid (from RN testID prop). No CSS/nth/XPath.
import { type Page, type BrowserContext, expect } from '@playwright/test';

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

export async function createGroup(page: Page, groupName: string): Promise<void> {
  await page.getByTestId('dashboard-create-group').click();
  await expect(page.getByTestId('create-group-name')).toBeVisible();
  await page.getByTestId('create-group-name').fill(groupName);
  await page.getByTestId('create-group-submit').click();
  await expect(page.getByTestId('group-overview-root')).toBeVisible({ timeout: 15_000 });
}

/** Tap Invite, capture the token from the POST /groups/:id/invite response. */
export async function inviteAndCaptureToken(page: Page): Promise<string> {
  const respPromise = page.waitForResponse(
    (r) =>
      r.url().includes('/groups/') &&
      r.url().endsWith('/invite') &&
      r.request().method() === 'POST',
    { timeout: 15_000 },
  );
  await page.getByTestId('group-invite-button').click();
  const resp = await respPromise;
  const body = await resp.json();
  return body.token as string;
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
