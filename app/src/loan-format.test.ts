import { buildLoanSchedule, loanRepaid, addMonths } from './loan-format';
import type { Loan } from './api';

function assert(cond: boolean, msg: string): void {
  if (!cond) throw new Error(`FAIL: ${msg}`);
}

function eq(a: unknown, b: unknown): boolean {
  return a === b;
}

// addMonths — integer month arithmetic, no Date rollover
assert(eq(addMonths('2026-01', 0), '2026-01'), 'addMonths n=0 identity');
assert(eq(addMonths('2026-01', 2), '2026-03'), '2026-01 +2 -> 2026-03');
assert(eq(addMonths('2026-11', 3), '2027-02'), '2026-11 +3 -> 2027-02 (year rollover)');

/** Build a LoanResponse-shaped fixture with sensible defaults. */
function makeLoan(over: Partial<Loan> = {}): Loan {
  return {
    id: 'loan-1',
    group_id: 'g1',
    user_id: 'u1',
    principal: { currency: 'INR', value: 9000 },
    installments: 3,
    emi_amount: { currency: 'INR', value: 3000 }, // ceil(9000/3)
    start_period: '2026-01',
    status: 'active',
    note: null,
    requested_at: '2026-01-01T00:00:00Z',
    decided_by: null,
    decided_at: null,
    installments_posted: 1,
    outstanding: { currency: 'INR', value: 6000 },
    ...over,
  };
}

// loanRepaid = principal − outstanding
assert(eq(loanRepaid(makeLoan()), 3000), 'repaid = 9000 - 6000 = 3000');

// n=3, posted=1, active → statuses [paid, due, pending]
{
  const sched = buildLoanSchedule(makeLoan());
  assert(eq(sched.length, 3), 'schedule has 3 installments');
  assert(eq(sched[0].status, 'paid'), 'installment 1 paid');
  assert(eq(sched[1].status, 'due'), 'installment 2 due');
  assert(eq(sched[2].status, 'pending'), 'installment 3 pending');
  assert(eq(sched[0].duePeriod, '2026-01'), 'installment 1 due 2026-01');
  assert(eq(sched[1].duePeriod, '2026-02'), 'installment 2 due 2026-02');
  assert(eq(sched[2].duePeriod, '2026-03'), 'installment 3 due 2026-03');
}

// Final installment carries the remainder = principal − emi*(n-1).
// principal 10000, n=3, emi=ceil(10000/3)=3334 → final = 10000 - 3334*2 = 3332
{
  const sched = buildLoanSchedule(
    makeLoan({
      principal: { currency: 'INR', value: 10000 },
      emi_amount: { currency: 'INR', value: 3334 },
      installments_posted: 0,
      outstanding: { currency: 'INR', value: 10000 },
    })
  );
  assert(eq(sched[0].amount, 3334), 'installment 1 = emi 3334');
  assert(eq(sched[1].amount, 3334), 'installment 2 = emi 3334');
  assert(eq(sched[2].amount, 3332), 'final installment = 10000 - 3334*2 = 3332');
  const total = sched.reduce((s, i) => s + i.amount, 0);
  assert(eq(total, 10000), 'installments sum to principal');
}

// Not-yet-active loan (no start_period) → duePeriod null, no "due" row
{
  const sched = buildLoanSchedule(
    makeLoan({ status: 'requested', start_period: null, installments_posted: 0 })
  );
  assert(eq(sched[0].duePeriod, null), 'requested loan installment has null duePeriod');
  assert(eq(sched.every(i => i.status === 'pending'), true), 'requested loan all pending');
}

// Closed loan → all installments render paid
{
  const sched = buildLoanSchedule(
    makeLoan({ status: 'closed', installments_posted: 2, outstanding: { currency: 'INR', value: 0 } })
  );
  assert(eq(sched.every(i => i.status === 'paid'), true), 'closed loan all paid');
}

console.log('All loan-format tests passed.');
