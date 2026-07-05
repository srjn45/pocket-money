import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { allowancesApi } from '../api';
import { qk } from '../query-keys';

export function useAllowances(groupId: string) {
  return useQuery({
    queryKey: qk.allowances(groupId),
    queryFn: () => allowancesApi.list(groupId),
    enabled: !!groupId,
  });
}

export function useSetAllowance(groupId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ userId, ...data }: { userId: string; amount: number; effective_from?: string }) =>
      allowancesApi.set(groupId, userId, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.allowances(groupId) });
      qc.invalidateQueries({ queryKey: qk.ledger(groupId) });
      qc.invalidateQueries({ queryKey: qk.balance(groupId) });
      qc.invalidateQueries({ queryKey: qk.group(groupId) });
    },
  });
}
