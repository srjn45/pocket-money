import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { loansApi, Money } from '../api';
import { qk } from '../query-keys';

export function useLoans(groupId: string) {
  return useQuery({
    queryKey: qk.loans(groupId),
    queryFn: () => loansApi.list(groupId),
    enabled: !!groupId,
  });
}

export function useRequestLoan(groupId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { principal: Money; installments: number; note?: string | null }) =>
      loansApi.request(groupId, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.loans(groupId) });
    },
  });
}

export function useApproveLoan(groupId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ loanId, ...data }: { loanId: string; principal?: Money | null; installments?: number | null }) =>
      loansApi.approve(loanId, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.loans(groupId) });
      qc.invalidateQueries({ queryKey: qk.ledger(groupId) });
      qc.invalidateQueries({ queryKey: qk.balance(groupId) });
      qc.invalidateQueries({ queryKey: qk.group(groupId) });
    },
  });
}

export function useRejectLoan(groupId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (loanId: string) => loansApi.reject(loanId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.loans(groupId) });
    },
  });
}

export function useCloseLoan(groupId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (loanId: string) => loansApi.close(loanId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.loans(groupId) });
      qc.invalidateQueries({ queryKey: qk.ledger(groupId) });
      qc.invalidateQueries({ queryKey: qk.balance(groupId) });
      qc.invalidateQueries({ queryKey: qk.group(groupId) });
    },
  });
}
