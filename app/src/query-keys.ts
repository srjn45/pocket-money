export const qk = {
  groups: () => ['groups'] as const,
  group: (id: string) => ['group', id] as const,
  members: (groupId: string) => ['members', groupId] as const,
  chores: (groupId: string) => ['chores', groupId] as const,
  ledger: (groupId: string, filters?: { status?: string; user_id?: string }) =>
    ['ledger', groupId, filters ?? {}] as const,
  balance: (groupId: string) => ['balance', groupId] as const,
  pending: (groupId: string) => ['pending', groupId] as const,
  settlements: (groupId: string) => ['settlements', groupId] as const,
  // reserved for later WPs (WP-2.x, WP-3.x):
  loans: (groupId: string) => ['loans', groupId] as const,
  allowances: (groupId: string) => ['allowances', groupId] as const,
  memberSummary: (groupId: string, userId: string) => ['member-summary', groupId, userId] as const,
};
