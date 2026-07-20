import { test, expect } from '@playwright/test';
import { uniqueEmail, registerUser } from '../support/pages';
import { apiLogin, apiCreateGroup } from '../support/api';

// Regression guard for the group-nav stale-content leak (fix/group-nav-tab-blur-leak).
//
// Root cause (pre-fix): the `groups` route tree lived under the (app) Tabs navigator
// as a hidden tab. On the web build the outer Tabs never set display:none on the
// inactive tab, so leaving a group left its screen mounted-and-visible and its nested
// Stack accumulated a fresh layer on every re-entry — the visible group flickered
// between the one you clicked and a stale prior one, and it took an extra back press
// per accumulated layer to reach the dashboard.
//
// Fix: (app) is now a Stack; Dashboard/Profile live in a nested (tabs) group and the
// group tree is a push-in sibling, so entering/leaving a group is a genuine Stack
// push/pop (which DOES toggle display:none / unmount correctly). This test drives the
// exact reported sequence and asserts the DOM never holds more than one
// group-overview-root, that backing out fully clears it, and that a single back always
// reaches the dashboard.
test('group nav: repeated in/out across two groups never leaks a stale overview', async ({ page }) => {
  const email = uniqueEmail();
  const pass = 'navpass123!';
  await registerUser(page, 'Nav Admin', email, pass);

  // Provision two groups via API, then reload the dashboard so both cards render.
  const { token } = await apiLogin(email, pass);
  const a = await apiCreateGroup(token, `Alpha-${Date.now()}`, 'USD');
  const b = await apiCreateGroup(token, `Beta-${Date.now()}`, 'USD');
  await page.goto('/');
  await expect(page.getByTestId('dashboard-root')).toBeVisible({ timeout: 15_000 });

  const overview = page.getByTestId('group-overview-root');

  const enter = async (id: string) => {
    await page.getByTestId(`group-card-${id}`).click();
    // Exactly one overview in the DOM, and it is the visible one — no stale copy.
    await expect(overview).toHaveCount(1, { timeout: 15_000 });
    await expect(overview).toBeVisible();
  };
  const backToDashboard = async () => {
    await page.getByTestId('header-back-button').click();
    // A single back reaches the dashboard AND fully removes the group screen
    // (pre-fix: the leaked screen stayed and a second back was needed).
    await expect(page.getByTestId('dashboard-root')).toBeVisible({ timeout: 15_000 });
    await expect(overview).toHaveCount(0);
  };

  // Cycle 1: enter Alpha, back out cleanly.
  await enter(a.id);
  await backToDashboard();

  // Cycle 2: enter Beta — the re-entry where the leak used to surface.
  await enter(b.id);
  await backToDashboard();

  // Cycle 3: back to Alpha to prove it does not worsen with repetition.
  await enter(a.id);
  await backToDashboard();
});
