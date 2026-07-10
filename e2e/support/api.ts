// Thin REST helpers for test setup (register/login without driving the UI).
// Used only when direct API state manipulation is faster than going through
// the browser for preconditions — mutating specs drive the full UI for their
// core journey assertions.

const API_BASE = process.env.E2E_API_BASE ?? 'http://localhost:8080/api/v1';

async function request<T>(path: string, options: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: { 'Content-Type': 'application/json', ...options.headers },
  });
  if (!res.ok) {
    const body = await res.text().catch(() => '');
    throw new Error(`${options.method ?? 'GET'} ${path} → ${res.status}: ${body}`);
  }
  return res.json();
}

export type AuthResponse = { token: string; user: { id: string; email: string; name: string } };

export type Currency = 'EUR' | 'USD' | 'INR';
/** Money object mirrors backend Money{currency,value} — value is minor units, never a float. */
export type Money = { currency: Currency; value: number };

export async function apiRegister(name: string, email: string, password: string): Promise<AuthResponse> {
  return request('/auth/register', {
    method: 'POST',
    body: JSON.stringify({ name, email, password }),
  });
}

export async function apiLogin(email: string, password: string): Promise<AuthResponse> {
  return request('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });
}

export async function apiCreateGroup(
  token: string,
  name: string,
  currency: Currency = 'INR',
): Promise<{ id: string; name: string; currency: Currency }> {
  return request('/groups', {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
    body: JSON.stringify({ name, currency }),
  });
}

export async function apiAddEntry(
  token: string,
  groupId: string,
  body: Record<string, unknown>,
): Promise<unknown> {
  return request(`/groups/${groupId}/ledger`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
    body: JSON.stringify(body),
  });
}

export async function apiApproveEntry(token: string, groupId: string, entryId: string): Promise<unknown> {
  return request(`/groups/${groupId}/ledger/${entryId}/approve`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
  });
}

// Toggle a group's member chore-submission flag (D2, default off since V3-3.2).
// Member chore logging is inert until an admin enables this.
export async function apiSetChoreSubmission(
  token: string,
  groupId: string,
  enabled: boolean,
): Promise<unknown> {
  return request(`/groups/${groupId}`, {
    method: 'PATCH',
    headers: { Authorization: `Bearer ${token}` },
    body: JSON.stringify({ member_chore_submission_enabled: enabled }),
  });
}

export async function apiCreateChore(
  token: string,
  groupId: string,
  name: string,
  amountMinor: number,
  currency: Currency = 'INR',
): Promise<{ id: string; name: string }> {
  return request(`/groups/${groupId}/chores`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
    // amount is a Money object now (currency must match the group's, V3-1.1).
    body: JSON.stringify({ name, amount: { currency, value: amountMinor } }),
  });
}
