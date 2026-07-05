import type { LedgerEntry, Chore, Member } from './api';

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

/** Friendly title for a ledger row per §2.2 of WP-1.3 spec. */
export function entryTitle(entry: LedgerEntry, chores: Chore[]): string {
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
      // WP-3.2 will add n/12 installment index; for now render without it
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
      return sum + (e.direction === 'credit' ? e.amount : -e.amount);
    }, 0);
    return { period, data, monthTotal };
  });
}
