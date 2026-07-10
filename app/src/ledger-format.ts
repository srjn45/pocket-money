import type { LedgerEntry, Chore, Member, Loan } from './api';

const MONTH_NAMES = [
  'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
  'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec',
] as const;

const MONTH_NAMES_FULL = [
  'January', 'February', 'March', 'April', 'May', 'June',
  'July', 'August', 'September', 'October', 'November', 'December',
] as const;

/** 'YYYY-MM' → 'Jul 2026' (short). Also used by entryTitle for allowance rows. */
export function humanMonth(period: string): string {
  const [yearStr, monthStr] = period.split('-');
  const monthIndex = parseInt(monthStr, 10) - 1;
  const name = MONTH_NAMES[monthIndex] ?? monthStr;
  return `${name} ${yearStr}`;
}

/** 'YYYY-MM' → 'July 2026' (full). Matches MonthHeader's formatPeriod. */
export function humanMonthFull(period: string): string {
  const [yearStr, monthStr] = period.split('-');
  const monthIndex = parseInt(monthStr, 10) - 1;
  const name = MONTH_NAMES_FULL[monthIndex] ?? monthStr;
  return `${name} ${yearStr}`;
}

/** ISO date string → 'YYYY-MM' using local timezone year+month. */
export function toYearMonth(iso: string): string {
  const d = new Date(iso);
  const year = d.getFullYear();
  const month = String(d.getMonth() + 1).padStart(2, '0');
  return `${year}-${month}`;
}

/** Returns the month key for grouping: period wins over created_at month. */
export function monthKeyOf(entry: LedgerEntry): string {
  return entry.period ?? toYearMonth(entry.created_at);
}

/** Months between two YYYY-MM strings (to - from, signed). */
function monthDiff(from: string, to: string): number {
  const [fy, fm] = from.split('-').map(Number);
  const [ty, tm] = to.split('-').map(Number);
  return (ty - fy) * 12 + (tm - fm);
}

/**
 * Friendly title for a ledger row per §2.2 of WP-1.3 spec.
 * Pass `loans` to enable "EMI k/n — <note>" labels; omit for safe fallback.
 */
export function entryTitle(entry: LedgerEntry, chores: Chore[], loans?: Loan[]): string {
  switch (entry.entry_type) {
    case 'chore': {
      const chore = chores.find(c => c.id === entry.chore_id);
      return chore?.name ?? 'Chore';
    }
    case 'allowance': {
      const period = entry.period ?? toYearMonth(entry.created_at);
      return `Pocket money — ${humanMonth(period)}`;
    }
    case 'emi': {
      const loan = loans?.find(l => l.id === entry.loan_id);
      if (loan && loan.start_period && entry.period) {
        const k = monthDiff(loan.start_period, entry.period) + 1;
        const n = loan.installments;
        const notePart = loan.note ? ` — ${loan.note}` : '';
        return `EMI ${k}/${n}${notePart}`;
      }
      // Safe fallback: no loan data available
      return entry.note ? `EMI — ${entry.note}` : 'EMI';
    }
    case 'settlement':
      return 'Settlement';
    case 'adjustment':
      return 'Adjustment';
    default:
      return 'Entry';
  }
}

/** Short date like '3 Jul' from an ISO date string. */
function shortDate(iso: string): string {
  const d = new Date(iso);
  const day = d.getDate();
  const month = MONTH_NAMES[d.getMonth()] ?? '';
  return `${day} ${month}`;
}

/** Returns the decided-by caption string (or null if the entry has no decided_by). */
export function decidedByCaption(entry: LedgerEntry, members: Member[]): string | null {
  if (!entry.decided_by) return null;
  const name = members.find(m => m.user_id === entry.decided_by)?.name ?? 'head';
  const action = entry.status === 'rejected' ? 'rejected' : 'approved';
  const datePart = entry.decided_at ? `, ${shortDate(entry.decided_at)}` : '';
  return `${action} by ${name}${datePart}`;
}

export interface MonthGroup {
  period: string;
  /** SectionList requires a `data` key for its items. */
  data: LedgerEntry[];
  /** Σ approved credits − Σ approved debits (pending/rejected excluded). */
  monthTotal: number;
}

/** Sort entries newest-first, bucket by monthKeyOf, compute approved-only total per bucket. */
export function groupEntriesByMonth(entries: LedgerEntry[]): MonthGroup[] {
  const sorted = [...entries].sort(
    (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
  );

  const map = new Map<string, LedgerEntry[]>();
  for (const entry of sorted) {
    const key = monthKeyOf(entry);
    const bucket = map.get(key);
    if (bucket) {
      bucket.push(entry);
    } else {
      map.set(key, [entry]);
    }
  }

  return Array.from(map.entries()).map(([period, data]) => {
    const monthTotal = data.reduce((sum, e) => {
      if (e.status !== 'approved') return sum;
      return sum + (e.direction === 'credit' ? e.amount.value : -e.amount.value);
    }, 0);
    return { period, data, monthTotal };
  });
}
