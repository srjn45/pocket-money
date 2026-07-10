import type { Loan } from './api';
import { nextPeriod } from './allowance-format';

/**
 * The month `n` steps after `period` ('2026-01', 2 → '2026-03'). n=0 → period.
 * Loops `nextPeriod` (integer month arithmetic, no Date rollover).
 */
export function addMonths(period: string, n: number): string {
  let p = period;
  for (let i = 0; i < n; i++) p = nextPeriod(p);
  return p;
}

/** Amount repaid so far = principal − outstanding (both minor units). */
export function loanRepaid(loan: Loan): number {
  return loan.principal.value - loan.outstanding.value;
}

export type InstallmentStatus = 'paid' | 'due' | 'pending';

export interface ScheduleInstallment {
  /** 1-based installment index. */
  index: number;
  /** 'YYYY-MM' due month, or null when the loan is not yet active (no start_period). */
  duePeriod: string | null;
  /** Installment amount in minor units. Final installment carries the remainder. */
  amount: number;
  status: InstallmentStatus;
}

/**
 * Derive the repayment schedule from a `LoanResponse` (there is no `schedule`
 * field on the API — it is computed from principal/installments/emi_amount/
 * start_period/installments_posted, D5).
 *
 * - installment i (1-based): due = addMonths(start_period, i-1); amount = emi
 *   except the final installment = principal − emi*(installments-1) (remainder).
 * - status: i <= installments_posted → paid; i === installments_posted+1 && loan
 *   active → due; else pending. A closed loan renders all installments paid.
 */
export function buildLoanSchedule(loan: Loan): ScheduleInstallment[] {
  const n = loan.installments;
  const emi = loan.emi_amount.value;
  const principal = loan.principal.value;
  const posted = loan.installments_posted;
  const out: ScheduleInstallment[] = [];

  for (let i = 1; i <= n; i++) {
    const amount = i < n ? emi : principal - emi * (n - 1);
    const duePeriod = loan.start_period ? addMonths(loan.start_period, i - 1) : null;

    let status: InstallmentStatus;
    if (loan.status === 'closed') {
      status = 'paid';
    } else if (i <= posted) {
      status = 'paid';
    } else if (i === posted + 1 && loan.status === 'active') {
      status = 'due';
    } else {
      status = 'pending';
    }

    out.push({ index: i, duePeriod, amount, status });
  }

  return out;
}
