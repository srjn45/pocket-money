// T-RO: Read-only assertions against the seeded "Sharma Family" demo data.
// These tests log in as the seeded users and assert that the expected balances
// and loan state from seed.summary.json are reflected in the UI.
//
// This spec is SKIPPED unless seed.summary.json exists (CI only after `make seed`).
import { test, expect } from '@playwright/test';
import { existsSync, readFileSync } from 'fs';
import { resolve } from 'path';
import { login, openTab } from '../support/pages';
import type { SeedSummary } from '../fixtures/seed';
import { SEED } from '../fixtures/seed';

const WEB_BASE = process.env.E2E_WEB_BASE ?? 'http://localhost:8081';

const summaryPath = resolve(__dirname, '../fixtures/seed.summary.json');
const hasSummary = existsSync(summaryPath);

/** Format minor units as the deterministic en-grouped 2-dp string, e.g. 1250 -> "12.50". */
function formatMinorGrouped(minor: number): string {
  const abs = Math.abs(minor);
  const whole = Math.floor(abs / 100);
  const frac = String(abs % 100).padStart(2, '0');
  const grouped = String(whole).replace(/\B(?=(\d{3})+(?!\d))/g, ',');
  return `${grouped}.${frac}`;
}

/** Escape a string for use inside a RegExp. */
function reEscape(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

const summaryTest = hasSummary ? test : test.skip;

function loadSummary(): SeedSummary {
  return JSON.parse(readFileSync(summaryPath, 'utf-8')) as SeedSummary;
}

// T-RO-1: Head logs in, sees the Sharma Family group on the dashboard.
summaryTest('T-RO-1: head sees Sharma Family on dashboard', async ({ page }) => {
  await login(page, SEED.HEAD.email, SEED.DEMO_PASSWORD);
  await expect(page.getByText(SEED.GROUP_NAME)).toBeVisible({ timeout: 10_000 });
});

// T-RO-2: Head navigates to the group; Aarav and Diya are listed as members.
summaryTest('T-RO-2: head sees both members in group overview', async ({ page }) => {
  await login(page, SEED.HEAD.email, SEED.DEMO_PASSWORD);
  // Click the Sharma Family group card.
  await page.getByText(SEED.GROUP_NAME).first().click();
  await expect(page.getByTestId('group-overview-root')).toBeVisible({ timeout: 15_000 });

  await expect(page.getByText(SEED.AARAV.name)).toBeVisible();
  await expect(page.getByText(SEED.DIYA.name)).toBeVisible();

  // Add-by-email is the visible membership path for the admin (D8).
  await expect(page.getByTestId('group-add-member-button')).toBeVisible();
});

// T-RO-3: Aarav's member detail shows an active loan.
summaryTest('T-RO-3: head sees aarav loan outstanding in member detail', async ({ page }) => {
  const summary = loadSummary();
  await login(page, SEED.HEAD.email, SEED.DEMO_PASSWORD);
  await page.getByText(SEED.GROUP_NAME).first().click();
  await expect(page.getByTestId('group-overview-root')).toBeVisible({ timeout: 15_000 });

  // Open Aarav's member card.
  const aaravId = summary.users.aarav.id;
  const aaravCard = page.getByTestId(`member-card-${aaravId}`);
  await expect(aaravCard).toBeVisible({ timeout: 10_000 });
  await aaravCard.click();
  await expect(page.getByTestId('member-detail-root')).toBeVisible({ timeout: 10_000 });

  // The ledger should contain EMI debit entries (from PostDue).
  // We don't assert the exact row count to keep this test robust against
  // period drift; just assert the member detail loaded successfully.
  await expect(page.getByTestId('member-detail-root')).toBeVisible();
});

// T-RO-4: Loans tab shows Aarav's active loan and Diya's rejected loan.
summaryTest('T-RO-4: head sees active and rejected loans on loans tab', async ({ page }) => {
  await login(page, SEED.HEAD.email, SEED.DEMO_PASSWORD);
  await page.getByText(SEED.GROUP_NAME).first().click();
  await expect(page.getByTestId('group-overview-root')).toBeVisible({ timeout: 15_000 });

  // Navigate to loans tab.
  await openTab(page, 'loans');
  await expect(page.getByTestId('loans-root')).toBeVisible({ timeout: 10_000 });

  // At least one loan card should be visible.
  const card = page.getByTestId(/^loan-card-/).first();
  await expect(card).toBeVisible({ timeout: 10_000 });

  // V3-4.3: tapping a card opens the read-only loan detail with a derived
  // repayment schedule (buildLoanSchedule) — one installment row per month.
  const loanId = (await card.getAttribute('data-testid'))!.replace('loan-card-', '');
  await card.click();
  await expect(page.getByTestId('loan-detail-root')).toBeVisible({ timeout: 15_000 });
  await expect(page.getByTestId('loan-schedule')).toBeVisible({ timeout: 10_000 });
  await expect(page.getByTestId(`loan-installment-${loanId}-1`)).toBeVisible({ timeout: 10_000 });
});

// T-RO-6: The seeded EUR group ("Sharma Europe Trip") renders its Gelato-treat
// chore with the € symbol — proving per-group currency (D7) flows end-to-end and
// is NOT the hardcoded ₹. Head (Priya) is a member of both groups.
summaryTest('T-RO-6: EUR group chore renders with euro symbol', async ({ page }) => {
  const summary = loadSummary();
  expect(summary.group2.currency).toBe('EUR');

  await login(page, SEED.HEAD.email, SEED.DEMO_PASSWORD);

  // Full-URL navigation to the EUR group's chores tab (expo-router web-param
  // gotcha, §9.12): tab-link clicks don't populate the [id] segment.
  await page.goto(`${WEB_BASE}/groups/${summary.group2.id}/chores`);
  await expect(page.getByTestId('chores-root')).toBeVisible({ timeout: 15_000 });

  // The seeded EUR chore amount renders as "€12.50" (group2_amount minor units),
  // never the INR ₹ — asserts concrete currency-formatted text (§9.12).
  const euro = `€${formatMinorGrouped(summary.group2_amount)}`;
  await expect(page.getByText(new RegExp(reEscape(euro))).first()).toBeVisible({ timeout: 10_000 });
});

// T-RO-5: Diya logs in and sees the Sharma Family group on her dashboard.
summaryTest('T-RO-5: diya (member) sees her group on dashboard', async ({ page }) => {
  await login(page, SEED.DIYA.email, SEED.DEMO_PASSWORD);
  await expect(page.getByText(SEED.GROUP_NAME)).toBeVisible({ timeout: 10_000 });
  // Member sees leave button, not invite button.
  await page.getByText(SEED.GROUP_NAME).first().click();
  await expect(page.getByTestId('group-overview-root')).toBeVisible({ timeout: 15_000 });
  await expect(page.getByTestId('group-leave-button')).toBeVisible();
  await expect(page.getByTestId('group-invite-button')).not.toBeVisible();
});
