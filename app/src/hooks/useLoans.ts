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

// Create a loan from the add-entry sheet. Admin passes user_id → a pre-approved
// active loan that may post an immediate/next-period EMI, so it also invalidates
// ledger/balance/group/statement (not just loans). Member omits user_id (self).
export function useCreateLoan(groupId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { user_id?: string | null; principal: Money; installments: number; note?: string | null; start_current_month?: boolean | null }) =>
      loansApi.request(groupId, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.loans(groupId) });
      qc.invalidateQueries({ queryKey: qk.ledger(groupId) });
      qc.invalidateQueries({ queryKey: qk.balance(groupId) });
      qc.invalidateQueries({ queryKey: qk.group(groupId) });
      qc.invalidateQueries({ queryKey: qk.statement(groupId) });
      qc.invalidateQueries({ queryKey: qk.groups() });
    },
  });
}

export function useApproveLoan(groupId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ loanId, ...data }: { loanId: string; principal?: Money | null; installments?: number | null; start_current_month?: boolean | null }) =>
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
