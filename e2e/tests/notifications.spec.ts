import { test, expect } from '@playwright/test';
import {
  uniqueEmail,
  registerUser,
  createGroup,
  addMemberByEmail,
  groupIdFromUrl,
  reloadInto,
  openNotifications,
} from '../support/pages';

// V3-5.2 acceptance (master-plan-v3 §8 Phase 5): a REGISTERED member added to a
// group gets an N-1 notification → their header bell shows 1 → opening the row
// marks it read AND deep-links to the group. N-1 fires only for registered users
// (a shadow add writes no notification), so the member must register BEFORE being
// added. Each test is self-provisioning (unique emails) → fullyParallel-safe.

test('notifications bell: registered member added to group → bell shows 1 → open → deep-links + marked read', async ({
  page,
  browser,
}) => {
  // 1. Admin registers and creates a group; capture gid.
  const adminEmail = uniqueEmail();
  await registerUser(page, 'Admin Notif', adminEmail, 'headpass123!');
  await createGroup(page, `Notif-${Date.now()}`);
  const gid = groupIdFromUrl(page);

  // 2. Member registers in a fresh context (isolated storage/token) → dashboard.
  const memberEmail = uniqueEmail();
  const ctxB = await browser.newContext();
  const pageB = await ctxB.newPage();
  try {
    await registerUser(pageB, 'Member Notif', memberEmail, 'member123!');
    // No notification yet → no badge node at all.
    await expect(pageB.getByTestId('header-bell-badge')).toHaveCount(0);

    // 3. Admin adds the (already-registered) member by email → N-1 is inserted.
    await addMemberByEmail(page, memberEmail, 'Member Notif');

    // 4. Force a fresh unread fetch (30s react-query staleTime): reload the member's
    //    dashboard, then assert the badge shows exactly 1.
    await reloadInto(pageB, 'dashboard-root');
    await expect(pageB.getByTestId('header-bell-badge')).toHaveText('1', { timeout: 15_000 });

    // 5. Open the list → exactly one row, unread dot present.
    await openNotifications(pageB);
    const row = pageB.getByTestId(/^notification-row-/);
    await expect(row).toHaveCount(1);
    const tid = await row.first().getAttribute('data-testid');
    const id = tid!.replace('notification-row-', '');
    await expect(pageB.getByTestId(`notification-unread-${id}`)).toBeVisible();

    // 6. Tap the row → deep-links to the group (state change = group screen mounts).
    await pageB.getByTestId(`notification-row-${id}`).click();
    await expect(pageB.getByTestId('group-overview-root')).toBeVisible({ timeout: 15_000 });
    expect(pageB.url()).toContain(`/groups/${gid}`);

    // 7. Marked read: go home, badge is gone (count 0 ⇒ no badge node).
    await pageB.goto('/');
    await expect(pageB.getByTestId('dashboard-root')).toBeVisible({ timeout: 15_000 });
    await expect(pageB.getByTestId('header-bell-badge')).toHaveCount(0, { timeout: 15_000 });
  } finally {
    await ctxB.close();
  }
});

test('notifications: mark-all-read clears the badge and all unread dots', async ({ page, browser }) => {
  // Admin registers + creates a group.
  const adminEmail = uniqueEmail();
  await registerUser(page, 'Admin MarkAll', adminEmail, 'headpass123!');
  await createGroup(page, `MarkAll-${Date.now()}`);

  // Member registers first, then is added → N-1 seeded.
  const memberEmail = uniqueEmail();
  const ctxB = await browser.newContext();
  const pageB = await ctxB.newPage();
  try {
    await registerUser(pageB, 'Member MarkAll', memberEmail, 'member123!');
    await addMemberByEmail(page, memberEmail, 'Member MarkAll');

    await reloadInto(pageB, 'dashboard-root');
    await expect(pageB.getByTestId('header-bell-badge')).toHaveText('1', { timeout: 15_000 });

    await openNotifications(pageB);
    // Bulk clear (distinct from mark-read-on-open) → badge + every unread dot gone.
    await pageB.getByTestId('notifications-mark-all-read').click();
    await expect(pageB.getByTestId(/^notification-unread-/)).toHaveCount(0, { timeout: 15_000 });
    await expect(pageB.getByTestId('header-bell-badge')).toHaveCount(0, { timeout: 15_000 });
  } finally {
    await ctxB.close();
  }
});
