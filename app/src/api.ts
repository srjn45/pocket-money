import Constants from 'expo-constants';
import { getToken, clearToken } from './storage';

// Get API URL from environment or use default
const BASE_URL = Constants.expoConfig?.extra?.apiUrl || 
  process.env.EXPO_PUBLIC_API_URL || 
  'http://localhost:8080/api/v1';

export interface ApiError {
  error: string;
}

export interface User {
  id: string;
  email: string;
  name: string;
  dob?: string;
  sex?: string;
  created_at: string;
}

export interface Group {
  id: string;
  name: string;
  head_user_id: string;
  created_at: string;
}

export interface Member {
  user_id: string;
  name: string;
  email: string;
  role: 'head' | 'member';
  joined_at: string;
}

export interface GroupDetail extends Group {
  members: Member[];
  chores_count: number;
}

export interface Chore {
  id: string;
  group_id: string;
  name: string;
  description?: string;
  amount: number;
  created_at: string;
}

export interface LedgerEntry {
  id: string;
  group_id: string;
  user_id: string;
  chore_id: string;
  amount: number;
  status: 'approved' | 'pending_approval' | 'rejected';
  created_by_user_id: string;
  approved_by_user_id?: string;
  rejected_by_user_id?: string;
  created_at: string;
}

export interface Balance {
  user_id: string;
  name: string;
  balance: number;
}

export interface Settlement {
  id: string;
  group_id: string;
  user_id: string;
  amount: number;
  date: string;
  note?: string;
  created_at: string;
}

export interface InviteResponse {
  invite_url: string;
  token: string;
  expires_at: string;
}

export interface LoginResponse {
  token: string;
  user: User;
}

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
  list: (groupId: string, status?: string) => {
    const params = status ? `?status=${status}` : '';
    return request<LedgerEntry[]>(`/groups/${groupId}/ledger${params}`);
  },
  
  create: (groupId: string, data: { user_id?: string; chore_id: string; amount: number }) =>
    request<LedgerEntry>(`/groups/${groupId}/ledger`, { method: 'POST', body: JSON.stringify(data) }),
  
  approve: (id: string) =>
    request<LedgerEntry>(`/ledger/${id}/approve`, { method: 'POST' }),
  
  reject: (id: string) =>
    request<LedgerEntry>(`/ledger/${id}/reject`, { method: 'POST' }),
  
  listPending: (groupId: string) => request<LedgerEntry[]>(`/groups/${groupId}/pending`),
  
  getBalance: (groupId: string) => request<Balance[]>(`/groups/${groupId}/balance`),
};

// Settlements API
export const settlementsApi = {
  list: (groupId: string) => request<Settlement[]>(`/groups/${groupId}/settlements`),
  
  create: (groupId: string, data: { user_id: string; amount: number; date: string; note?: string }) =>
    request<Settlement>(`/groups/${groupId}/settlements`, { method: 'POST', body: JSON.stringify(data) }),
};
