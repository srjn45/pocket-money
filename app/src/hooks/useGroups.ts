import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { groupsApi, GroupSummary, CurrencyCode } from '../api';
import { qk } from '../query-keys';

export function useGroups() {
  return useQuery<GroupSummary[]>({
    queryKey: qk.groups(),
    queryFn: () => groupsApi.list(),
  });
}

export function useCreateGroup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { name: string; currency: CurrencyCode }) => groupsApi.create(data),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.groups() }),
  });
}

export function useJoinGroup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (token: string) => groupsApi.join(token),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.groups() }),
  });
}

export function useAddMember(groupId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { email: string; name: string }) =>
      groupsApi.addMember(groupId, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.group(groupId) });     // refresh member list
      qc.invalidateQueries({ queryKey: qk.balance(groupId) });   // new member's balance row
      qc.invalidateQueries({ queryKey: qk.statement(groupId) }); // V3-4.2: statement rows drive the group home now
    },
  });
}
