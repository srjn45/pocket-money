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

  addMember: (id: string, data: { email: string; name: string }) =>
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

  request: (groupId: string, data: { principal: Money; installments: number; note?: string | null }) =>
    request<Loan>(`/groups/${groupId}/loans`, { method: 'POST', body: JSON.stringify(data) }),

  approve: (loanId: string, data: { principal?: Money | null; installments?: number | null }) =>
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

  getBalance: (groupId: string) => request<Balance[]>(`/groups/${groupId}/balance`),
};
