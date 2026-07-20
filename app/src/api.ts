import Constants from 'expo-constants';
import { getToken, clearToken } from './storage';
import type { components } from './api-types.gen';

type Schemas = components['schemas'];

// Get API URL from environment or use default
const BASE_URL = Constants.expoConfig?.extra?.apiUrl ||
  process.env.EXPO_PUBLIC_API_URL ||
  'http://localhost:8080/api/v1';

export type ApiError             = Schemas['ErrorResponse'];
export type User                 = Schemas['UserResponse'];
export type Group                = Schemas['GroupResponse'];
export type GroupSummary         = Schemas['GroupSummaryResponse'];
export type GroupDetail          = Schemas['GroupDetailResponse'];
export type Member               = Schemas['MemberResponse'];
export type Chore                = Schemas['ChoreResponse'];
export type LedgerEntry          = Schemas['LedgerResponse'];
export type Balance              = Schemas['BalanceResponse'];
export type InviteResponse       = Schemas['InviteResponse'];
export type LoginResponse        = Schemas['LoginResponse'];
export type Allowance            = Schemas['AllowanceResponse'];
export type Loan                 = Schemas['LoanResponse'];
export type ChangePasswordRequest = Schemas['ChangePasswordRequest'];
export type Money                = Schemas['Money'];
export type CurrencyCode         = Money['currency'];
export type Statement            = Schemas['StatementResponse'];
export type MemberStatement      = Schemas['MemberStatementResponse'];
export type StatementTotals      = Schemas['StatementTotals'];
export type Notification             = Schemas['Notification'];
export type NotificationListResponse = Schemas['NotificationListResponse'];
export type UnreadCountResponse      = Schemas['UnreadCountResponse'];
export type MarkAllReadResponse      = Schemas['MarkAllReadResponse'];

let onUnauthorized: (() => void) | null = null;

export function setOnUnauthorized(callback: () => void) {
  onUnauthorized = callback;
}

async function request<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const token = await getToken();

  const headers: HeadersInit = {
    'Content-Type': 'application/json',
    ...(options.headers || {}),
  };

  if (token) {
    (headers as Record<string, string>)['Authorization'] = `Bearer ${token}`;
  }

  const response = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers,
  });

  if (response.status === 401) {
    await clearToken();
    if (onUnauthorized) {
      onUnauthorized();
    }
    throw new Error('Unauthorized');
  }

  if (!response.ok) {
    const error: ApiError = await response.json().catch(() => ({ error: 'Request failed' }));
    throw new Error(error.error || 'Request failed');
  }

  // Handle 204 No Content
  if (response.status === 204) {
    return undefined as T;
  }

  return response.json();
}

// Auth API
export const authApi = {
  register: (data: { email: string; password: string; name: string; dob?: string; sex?: string }) =>
    request<LoginResponse>('/auth/register', { method: 'POST', body: JSON.stringify(data) }),

  login: (data: { email: string; password: string }) =>
    request<LoginResponse>('/auth/login', { method: 'POST', body: JSON.stringify(data) }),

  me: () => request<User>('/auth/me'),

  changePassword: (data: ChangePasswordRequest) =>
    request<void>('/auth/password', { method: 'PUT', body: JSON.stringify(data) }),
};

// Groups API
export const groupsApi = {
  list: () => request<GroupSummary[]>('/groups'),

  create: (data: { name: string; currency: CurrencyCode }) =>
    request<Group>('/groups', { method: 'POST', body: JSON.stringify(data) }),

  get: (id: string) => request<GroupDetail>(`/groups/${id}`),

  getMembers: (id: string) => request<Member[]>(`/groups/${id}/members`),

  // base_pay is optional (QA batch 1, Item 4): when present the server seeds the
  // new member's monthly allowance atomically with the membership.
  addMember: (id: string, data: { email: string; name: string; base_pay?: Money | null }) =>
    request<Member>(`/groups/${id}/members`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  createInvite: (id: string, expiresInDays?: number) =>
    request<InviteResponse>(`/groups/${id}/invite`, {
      method: 'POST',
      body: JSON.stringify({ expires_in_days: expiresInDays || 7 })
    }),

  join: (token: string) =>
    request<Group>('/groups/join', { method: 'POST', body: JSON.stringify({ token }) }),

  // Serves both "head removes member" and "member leaves" (caller passes own id).
  removeMember: (groupId: string, userId: string) =>
    request<void>(`/groups/${groupId}/members/${userId}`, { method: 'DELETE' }),

  // Soft-delete (archive) a group — admin only, 204 on success (QA batch 1, Item 1).
  delete: (id: string) => request<void>(`/groups/${id}`, { method: 'DELETE' }),
};

// Chores API
export const choresApi = {
  list: (groupId: string) => request<Chore[]>(`/groups/${groupId}/chores`),

  create: (groupId: string, data: { name: string; description?: string; amount: Money }) =>
    request<Chore>(`/groups/${groupId}/chores`, { method: 'POST', body: JSON.stringify(data) }),

  update: (id: string, data: { name?: string; description?: string; amount?: Money | null }) =>
    request<Chore>(`/chores/${id}`, { method: 'PATCH', body: JSON.stringify(data) }),

  delete: (id: string) => request<void>(`/chores/${id}`, { method: 'DELETE' }),
};

// Allowances API (WP-2.1 contract)
export const allowancesApi = {
  // Head → all members' allowance history rows; member → own rows only (API-enforced).
  list: (groupId: string) =>
    request<Allowance[]>(`/groups/${groupId}/allowances`),

  // Head only. amount in minor units (0 = pause); effective_from optional 'YYYY-MM'
  // (server defaults to current month). A new effective_from is a new history row.
  set: (groupId: string, userId: string, data: { amount: Money; effective_from?: string }) =>
    request<Allowance>(`/groups/${groupId}/allowances/${userId}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
};

// Loans API
export const loansApi = {
  list: (groupId: string, options?: { user_id?: string; status?: string }) => {
    const params = new URLSearchParams();
    if (options?.user_id) params.append('user_id', options.user_id);
    if (options?.status) params.append('status', options.status);
    const qs = params.toString();
    return request<Loan[]>(`/groups/${groupId}/loans${qs ? `?${qs}` : ''}`);
  },

  // user_id: admin creates a pre-approved active loan for that member; omit for a
  // member self-request (openapi CreateLoanRequest already allows user_id).
  request: (groupId: string, data: { user_id?: string | null; principal: Money; installments: number; note?: string | null; start_current_month?: boolean | null }) =>
    request<Loan>(`/groups/${groupId}/loans`, { method: 'POST', body: JSON.stringify(data) }),

  approve: (loanId: string, data: { principal?: Money | null; installments?: number | null; start_current_month?: boolean | null }) =>
    request<Loan>(`/loans/${loanId}/approve`, { method: 'POST', body: JSON.stringify(data) }),

  reject: (loanId: string) =>
    request<Loan>(`/loans/${loanId}/reject`, { method: 'POST' }),

  close: (loanId: string) =>
    request<Loan>(`/loans/${loanId}/close`, { method: 'POST' }),
};

// Ledger API
export const ledgerApi = {
  list: (groupId: string, options?: { status?: string; user_id?: string; type?: string; period?: string }) => {
    const params = new URLSearchParams();
    if (options?.status) params.append('status', options.status);
    if (options?.user_id) params.append('user_id', options.user_id);
    if (options?.type) params.append('type', options.type);
    if (options?.period) params.append('period', options.period);
    const queryString = params.toString();
    return request<LedgerEntry[]>(`/groups/${groupId}/ledger${queryString ? `?${queryString}` : ''}`);
  },

  create: (groupId: string, data: {
    entry_type: 'chore' | 'settlement' | 'adjustment';
    user_id?: string;
    chore_id?: string;
    amount?: Money | null;
    direction?: 'credit' | 'debit';
    note?: string;
  }) =>
    request<LedgerEntry>(`/groups/${groupId}/ledger`, { method: 'POST', body: JSON.stringify(data) }),

  approve: (id: string) =>
    request<LedgerEntry>(`/ledger/${id}/approve`, { method: 'POST' }),

  reject: (id: string) =>
    request<LedgerEntry>(`/ledger/${id}/reject`, { method: 'POST' }),

  // Corrections (V3-3.2 endpoints; D3). Manual entries only — the backend returns
  // 403 for allowance/emi and for non-admins. EditLedgerRequest requires amount
  // (value >= 1) even for a chore; direction is for adjustment only.
  edit: (id: string, data: { amount: Money; direction?: 'credit' | 'debit' | null; note?: string | null }) =>
    request<LedgerEntry>(`/ledger/${id}`, { method: 'PUT', body: JSON.stringify(data) }),

  remove: (id: string) =>
    request<void>(`/ledger/${id}`, { method: 'DELETE' }),

  getBalance: (groupId: string) => request<Balance[]>(`/groups/${groupId}/balance`),
};

// Statement API (V3-3.1 endpoint; consumed by the V3-4.2 Statement screen).
// Role-scoped server-side: admin → all non-admin rows + group_total; member →
// own row only, group_total null. Every Money is stamped with the group currency.
export const statementApi = {
  get: (groupId: string, period: string) =>
    request<Statement>(`/groups/${groupId}/statement?period=${period}`),
};

// Notifications API (V3-5.1 endpoints; consumed by the V3-5.2 bell + list screen).
// User-scoped server-side (D6): every call returns/mutates only the caller's own
// notifications. `cursor` is opaque keyset paging (null ⇒ last page).
export const notificationsApi = {
  list: (cursor?: string | null) =>
    request<NotificationListResponse>(
      `/notifications${cursor ? `?cursor=${encodeURIComponent(cursor)}` : ''}`,
    ),

  unreadCount: () => request<UnreadCountResponse>('/notifications/unread_count'),

  // 204 No Content; request() already maps 204 → undefined. Idempotent server-side.
  markRead: (id: string) =>
    request<void>(`/notifications/${id}/read`, { method: 'POST' }),

  markAllRead: () =>
    request<MarkAllReadResponse>('/notifications/read_all', { method: 'POST' }),
};
