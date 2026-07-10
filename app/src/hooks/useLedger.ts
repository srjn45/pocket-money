import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { ledgerApi } from '../api';
import { qk } from '../query-keys';

type LedgerFilters = { status?: string; user_id?: string; type?: string; period?: string };

export function useLedger(groupId: string, filters?: LedgerFilters) {
  return useQuery({
    queryKey: qk.ledger(groupId, filters),
    queryFn: () => ledgerApi.list(groupId, filters),
    enabled: !!groupId,
  });
}

export function useBalance(groupId: string) {
  return useQuery({
    queryKey: qk.balance(groupId),
    queryFn: () => ledgerApi.getBalance(groupId),
    enabled: !!groupId,
  });
}

function useInvalidateGroup(groupId: string) {
  const qc = useQueryClient();
  return () => {
    qc.invalidateQueries({ queryKey: qk.ledger(groupId) });
    qc.invalidateQueries({ queryKey: qk.balance(groupId) });
    qc.invalidateQueries({ queryKey: qk.group(groupId) });
    // V3-4.2: refresh the statement (so `remaining` recomputes to 0 after a
    // payment — prefix form invalidates all cached months) and the dashboard
    // headline (summary_balance from GET /groups) so both track every mutation.
    qc.invalidateQueries({ queryKey: qk.statement(groupId) });
    qc.invalidateQueries({ queryKey: qk.groups() });
  };
}

export function useCreateLedgerEntry(groupId: string) {
  const invalidate = useInvalidateGroup(groupId);
  return useMutation({
    mutationFn: (data: Parameters<typeof ledgerApi.create>[1]) =>
      ledgerApi.create(groupId, data),
    onSuccess: invalidate,
  });
}

export function useApproveLedger(groupId: string) {
  const invalidate = useInvalidateGroup(groupId);
  return useMutation({
    mutationFn: (id: string) => ledgerApi.approve(id),
    onSuccess: invalidate,
  });
}

export function useRejectLedger(groupId: string) {
  const invalidate = useInvalidateGroup(groupId);
  return useMutation({
    mutationFn: (id: string) => ledgerApi.reject(id),
    onSuccess: invalidate,
  });
}
