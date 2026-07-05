import { useMutation, useQueryClient } from '@tanstack/react-query';
import { authApi, groupsApi, ChangePasswordRequest } from '../api';
import { qk } from '../query-keys';

export function useChangePassword() {
  return useMutation({
    mutationFn: (data: ChangePasswordRequest) => authApi.changePassword(data),
    // No invalidation: nothing cached changes; JWT unchanged (D1).
  });
}

export function useRemoveMember(groupId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (userId: string) => groupsApi.removeMember(groupId, userId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.group(groupId) });
      qc.invalidateQueries({ queryKey: qk.members(groupId) });
      qc.invalidateQueries({ queryKey: qk.balance(groupId) });
      qc.invalidateQueries({ queryKey: qk.loans(groupId) });
      qc.invalidateQueries({ queryKey: qk.groups() });
    },
  });
}

export function useLeaveGroup(groupId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (userId: string) => groupsApi.removeMember(groupId, userId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.groups() });
      // Remove cached group detail — caller no longer has access; prevents 403 refetch.
      qc.removeQueries({ queryKey: qk.group(groupId) });
    },
  });
}
