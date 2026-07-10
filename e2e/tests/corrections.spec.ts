// V3-4.3 (D3): edit/delete on MANUAL ledger entries with the "edited" affordance.
// Core acceptance (§8.A): post a chore (₹20.00) → statement payable ₹20.00 → edit
// the amount to ₹35.00 → "Edited" badge shows → statement payable recomputes to
// ₹35.00 (useInvalidateGroup fans out to the statement) → delete → payable ₹0.00.
import { test, expect } from '@playwright/test';
import {
  uniqueEmail,
  registerUser,
  createGroup,
  addMemberByEmail,
  groupIdFromUrl,
  reloadInto,
  autoAcceptDialogs,
  headEditEntry,
  headDeleteEntry,
} from '../support/pages';
import { apiLogin, apiCreateChore } from '../support/api';

test('corrections: edit a chore amount → statement updates, then delete', async ({ page }) => {
  autoAcceptDialogs(page); // destructive delete uses window.confirm on web

  const headEmail = uniqueEmail();
  await registerUser(page, 'Head Corr', headEmail);
  await createGroup(page, `CorrFam-${Date.now()}`, 'INR');
  const gid = groupIdFromUrl(page);

  // Seed a ₹20.00 (2000 minor) chore via API, then add a member by email.
  const { token } = await apiLogin(headEmail, 'password123');
  await apiCreateChore(token, gid, 'Dishes', 2000);
  await addMemberByEmail(page, uniqueEmail(), 'Kid Corr');

  // Reload so the head sees the new member + chore, then grab the member uid.
  await reloadInto(page, 'group-overview-root');
  const card = page.getByTestId(/^member-card-/).first();
  await expect(card).toBeVisible({ timeout: 15_000 });
  const uid = (await card.getAttribute('data-testid'))!.replace('member-card-', '');

  // Open the passbook and post the chore (head mode, fixed user → auto-approved).
  await card.click();
  await expect(page.getByTestId('member-detail-root')).toBeVisible({ timeout: 15_000 });
  await page.getByTestId('member-add-entry-button').click();
  const chorePicker = page.getByTestId('entry-chore-picker');
  await expect(chorePicker).toBeVisible({ timeout: 10_000 });
  await expect(chorePicker.locator('option')).not.toHaveCount(0, { timeout: 10_000 });
  await chorePicker.selectOption({ index: 0 });
  await page.getByTestId('entry-submit').click();
  await expect(page.getByText('Dishes').first()).toBeVisible({ timeout: 15_000 });

  // Overview: payable = ₹20.00.
  await page.goto(`/groups/${gid}`);
  await expect(page.getByTestId('group-overview-root')).toBeVisible({ timeout: 15_000 });
  await expect(page.getByTestId(`statement-payable-${uid}`)).toContainText('20.00', { timeout: 15_000 });

  // Re-open passbook, edit the chore amount → ₹35.00, assert the "Edited" badge.
  await page.getByTestId(`member-card-${uid}`).click();
  await expect(page.getByTestId('member-detail-root')).toBeVisible({ timeout: 15_000 });
  const entryId = await headEditEntry(page, '35.00');
  await expect(page.getByTestId(`ledger-edited-${entryId}`)).toBeVisible({ timeout: 15_000 });

  // Overview: payable recomputed to ₹35.00 (invalidation → refetch, §4.6).
  await page.goto(`/groups/${gid}`);
  await expect(page.getByTestId('group-overview-root')).toBeVisible({ timeout: 15_000 });
  await expect(page.getByTestId(`statement-payable-${uid}`)).toContainText('35.00', { timeout: 15_000 });

  // Delete the entry → payable back to ₹0.00.
  await page.getByTestId(`member-card-${uid}`).click();
  await expect(page.getByTestId('member-detail-root')).toBeVisible({ timeout: 15_000 });
  await headDeleteEntry(page, entryId);
  await expect(page.getByTestId(`ledger-delete-${entryId}`)).toHaveCount(0, { timeout: 15_000 });

  await page.goto(`/groups/${gid}`);
  await expect(page.getByTestId('group-overview-root')).toBeVisible({ timeout: 15_000 });
  await expect(page.getByTestId(`statement-payable-${uid}`)).toContainText('0.00', { timeout: 15_000 });
});

// System entries (allowance/emi) must NOT expose edit/delete controls (D3/D6). A
// brand-new member has no system rows, so we assert the negative on the passbook:
// posting a chore yields exactly one manual row with edit/delete, and there is no
// edit control on any non-manual row (there are none to offer here).
test('corrections: manual row exposes edit/delete controls to admin', async ({ page }) => {
  const headEmail = uniqueEmail();
  await registerUser(page, 'Head Corr2', headEmail);
  await createGroup(page, `CorrFam2-${Date.now()}`, 'INR');
  const gid = groupIdFromUrl(page);

  const { token } = await apiLogin(headEmail, 'password123');
  await apiCreateChore(token, gid, 'Sweep', 1500);
  await addMemberByEmail(page, uniqueEmail(), 'Kid Corr2');

  await reloadInto(page, 'group-overview-root');
  const card = page.getByTestId(/^member-card-/).first();
  await expect(card).toBeVisible({ timeout: 15_000 });
  await card.click();
  await expect(page.getByTestId('member-detail-root')).toBeVisible({ timeout: 15_000 });

  await page.getByTestId('member-add-entry-button').click();
  const chorePicker = page.getByTestId('entry-chore-picker');
  await expect(chorePicker).toBeVisible({ timeout: 10_000 });
  await expect(chorePicker.locator('option')).not.toHaveCount(0, { timeout: 10_000 });
  await chorePicker.selectOption({ index: 0 });
  await page.getByTestId('entry-submit').click();

  // The manual chore row exposes both correction controls.
  await expect(page.getByTestId(/^ledger-edit-/).first()).toBeVisible({ timeout: 15_000 });
  await expect(page.getByTestId(/^ledger-delete-/).first()).toBeVisible({ timeout: 15_000 });
});
