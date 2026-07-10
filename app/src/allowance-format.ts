import type { Allowance, CurrencyCode } from './api';
import { formatMoney } from './money';

/** Current server-local month as 'YYYY-MM'. The server re-defaults/validates on PUT. */
export function currentPeriod(now: Date = new Date()): string {
  const y = now.getFullYear();
  const m = String(now.getMonth() + 1).padStart(2, '0');
  return `${y}-${m}`;
}

/** The month after `period` ('2026-12' → '2027-01'). Integer arithmetic, no Date rollover. */
export function nextPeriod(period: string): string {
  const [y, m] = period.split('-').map(Number);
  const total = y * 12 + (m - 1) + 1;
  const ny = Math.floor(total / 12);
  const nm = String((total % 12) + 1).padStart(2, '0');
  return `${ny}-${nm}`;
}

/** The allowance in force at `period` = greatest effective_from <= period. Null if all-future. */
export function currentAllowanceFor(rows: Allowance[], period: string): Allowance | null {
  let best: Allowance | null = null;
  for (const r of rows) {
    if (r.effective_from <= period && (best === null || r.effective_from > best.effective_from)) {
      best = r;
    }
  }
  return best;
}

/** Earliest scheduled future change (smallest effective_from > period), or null. */
export function upcomingAllowanceFor(rows: Allowance[], period: string): Allowance | null {
  let best: Allowance | null = null;
  for (const r of rows) {
    if (r.effective_from > period && (best === null || r.effective_from < best.effective_from)) {
      best = r;
    }
  }
  return best;
}

/** Human label: null → 'Not set'; amount 0 → 'Paused'; else 'X.XX / month' in the group currency. */
export function describeAllowance(a: Allowance | null, currency: CurrencyCode): string {
  if (a === null) return 'Not set';
  if (a.amount.value === 0) return 'Paused';
  return `${formatMoney(a.amount.value, currency)} / month`;
}
