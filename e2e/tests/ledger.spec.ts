import { test, expect, type Browser, type BrowserContext, type Page } from '@playwright/test';
import {
  uniqueEmail,
  registerUser,
  createGroup,
  inviteAndCaptureToken,
  groupIdFromUrl,
  reloadInto,
  memberLogChore,
  approveFirstPending,
  rejectFirstPending,
} from '../support/pages';
import { apiLogin, apiCreateChore } from '../support/api';

const WEB_BASE = process.env.E2E_WEB_BASE ?? 'http://localhost:8081';

async function joinGroupAsMember(
  browser: Browser,
  token: string,
  name: string,
  password = 'member123!',
): Promise<{ ctx: BrowserContext; page: Page; email: string }> {
  const ctx = await browser.newContext();
  const pg = await ctx.newPage();
  await pg.goto(`${WEB_BASE}/invite?token=${token}`);
  await expect(pg.getByTestId('login-submit')).toBeVisible({ timeout: 15_000 });
  await pg.getByTestId('login-link-register').click();
  await expect(pg.getByTestId('register-submit')).toBeVisible({ timeout: 10_000 });
  const email = uniqueEmail();
  await pg.getByTestId('register-name').fill(name);
  await pg.getByTestId('register-email').fill(email);
  await pg.getByTestId('register-password').fill(password);
  await pg.getByTestId('register-confirm').fill(password);
  await pg.getByTestId('register-submit').click();
  await expect(pg.getByTestId('group-overview-root')).toBeVisible({ timeout: 20_000 });
  return { ctx, page: pg, email };
}

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

  const inviteToken = await inviteAndCaptureToken(page);
  const { ctx: memberCtx, page: memberPage } = await joinGroupAsMember(browser, inviteToken, 'Member T4');

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

  const inviteToken = await inviteAndCaptureToken(page);
  const { ctx: memberCtx } = await joinGroupAsMember(browser, inviteToken, 'Member T5');

  try {
    await reloadInto(page, 'group-overview-root');
    const memberCard = page.getByTestId(/^member-card-/).first();
    await expect(memberCard).toBeVisible({ timeout: 10_000 });
    await memberCard.click();
    await expect(page.getByTestId('member-detail-root')).toBeVisible({ timeout: 10_000 });

    // Open the allowance sheet. The AllowanceSummary "Set"/"Edit" button sits in a
    // component outside the bounded testID list, so target it by role/name.
    await page.getByRole('button', { name: /^(set|edit)$/i }).click();

    // Set ₹300/month and save.
    await page.getByPlaceholder('e.g. 500').fill('300');
    await page.getByRole('button', { name: /^save$/i }).click();

    // Success toast, then the amount renders on the member summary (balance is ₹0,
    // so 300.00 can only be the allowance).
    await expect(page.getByTestId('toast-root')).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText(/300\.00/).first()).toBeVisible({ timeout: 10_000 });
  } finally {
    await memberCtx.close();
  }
});
