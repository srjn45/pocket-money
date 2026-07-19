export const qk = {
  groups: () => ['groups'] as const,
  group: (id: string) => ['group', id] as const,
  members: (groupId: string) => ['members', groupId] as const,
  chores: (groupId: string) => ['chores', groupId] as const,
  ledger: (groupId: string, filters?: { status?: string; user_id?: string; type?: string; period?: string }) =>
    ['ledger', groupId, filters ?? {}] as const,
  balance: (groupId: string) => ['balance', groupId] as const,
  // Monthly statement (V3-4.2). Prefix form (no period) invalidates every cached
  // month for the group; the period form keys a specific month's query.
  statement: (groupId: string, period?: string) =>
    period ? (['statement', groupId, period] as const) : (['statement', groupId] as const),
  // Notifications (V3-5.2). The list is an infinite (paged) query; the count is the
  // badge source. Mark-read / mark-all-read invalidate BOTH so the badge and the
  // read/unread rows recompute together.
  notifications: () => ['notifications', 'list'] as const,
  unreadCount:   () => ['notifications', 'unread-count'] as const,
  // reserved for later WPs (WP-2.x, WP-3.x):
  loans: (groupId: string) => ['loans', groupId] as const,
  allowances: (groupId: string) => ['allowances', groupId] as const,
  memberSummary: (groupId: string, userId: string) => ['member-summary', groupId, userId] as const,
};
