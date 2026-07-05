import Constants from 'expo-constants';
import { getToken, clearToken } from './storage';
import type { components } from './api-types.gen';

type Schemas = components['schemas'];

// Get API URL from environment or use default
const BASE_URL = Constants.expoConfig?.extra?.apiUrl ||
  process.env.EXPO_PUBLIC_API_URL ||
  'http://localhost:8080/api/v1';

export type ApiError      = Schemas['ErrorResponse'];
export type User          = Schemas['UserResponse'];
export type Group         = Schemas['GroupResponse'];
export type GroupDetail   = Schemas['GroupDetailResponse'];
export type Member        = Schemas['MemberResponse'];
export type Chore         = Schemas['ChoreResponse'];
export type LedgerEntry   = Schemas['LedgerResponse'];
export type Balance       = Schemas['BalanceResponse'];
export type InviteResponse = Schemas['InviteResponse'];
export type LoginResponse  = Schemas['LoginResponse'];

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
    request<User>('/auth/register', { method: 'POST', body: JSON.stringify(data) }),

  login: (data: { email: string; password: string }) =>
    request<LoginResponse>('/auth/login', { method: 'POST', body: JSON.stringify(data) }),

  me: () => request<User>('/auth/me'),
};

// Groups API
export const groupsApi = {
  list: () => request<Group[]>('/groups'),

  create: (data: { name: string }) =>
    request<Group>('/groups', { method: 'POST', body: JSON.stringify(data) }),

  get: (id: string) => request<GroupDetail>(`/groups/${id}`),

  getMembers: (id: string) => request<Member[]>(`/groups/${id}/members`),

  createInvite: (id: string, expiresInDays?: number) =>
    request<InviteResponse>(`/groups/${id}/invite`, {
      method: 'POST',
      body: JSON.stringify({ expires_in_days: expiresInDays || 7 })
    }),

  join: (token: string) =>
    request<Group>('/groups/join', { method: 'POST', body: JSON.stringify({ token }) }),
};

// Chores API
export const choresApi = {
  list: (groupId: string) => request<Chore[]>(`/groups/${groupId}/chores`),

  create: (groupId: string, data: { name: string; description?: string; amount: number }) =>
    request<Chore>(`/groups/${groupId}/chores`, { method: 'POST', body: JSON.stringify(data) }),

  update: (id: string, data: { name?: string; description?: string; amount?: number }) =>
    request<Chore>(`/chores/${id}`, { method: 'PATCH', body: JSON.stringify(data) }),

  delete: (id: string) => request<void>(`/chores/${id}`, { method: 'DELETE' }),
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
    amount?: number;
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
