import type { CurrencyCode } from './api';

export type { CurrencyCode };

/** Currency symbol table. The ONLY place a currency symbol is hardcoded (§3.8). */
const SYMBOL: Record<CurrencyCode, string> = { EUR: '€', USD: '$', INR: '₹' };

/** The currency's symbol, e.g. 'INR' -> '₹'. For input labels / inline copy. */
export function currencySymbol(currency: CurrencyCode): string {
  return SYMBOL[currency];
}

/** Format integer minor units (paise/cents) as a plain string WITHOUT sign or symbol: 1250 -> "12.50". */
export function formatMinor(minorUnits: number): string {
  return (Math.abs(minorUnits) / 100).toFixed(2);
}

/**
 * Unsigned, no symbol, en-grouped: 152000 -> "1,520.00", 1250 -> "12.50".
 * Hand-rolled grouping (NOT Intl.NumberFormat) so output is deterministic across CI locales (§2).
 */
export function formatMinorGrouped(minor: number): string {
  const abs = Math.abs(minor);
  const whole = Math.floor(abs / 100);
  const frac = String(abs % 100).padStart(2, '0');
  const grouped = String(whole).replace(/\B(?=(\d{3})+(?!\d))/g, ',');
  return `${grouped}.${frac}`;
}

/**
 * Symbol + en-grouped, signed by numeric sign:
 * (152000,'INR') -> "₹1,520.00"; (-500,'USD') -> "-$5.00".
 */
export function formatMoney(minor: number, currency: CurrencyCode): string {
  const sign = minor < 0 ? '-' : '';
  return `${sign}${SYMBOL[currency]}${formatMinorGrouped(minor)}`;
}

/**
 * Parse a user-entered amount string to integer minor units (paise/cents).
 * "12.50" -> 1250, "12" -> 1200, "0.05" -> 5, ".5" -> 50, "12.5" -> 1250.
 * Returns null for invalid input (empty, non-numeric, negative, >2 decimals).
 * NEVER uses parseFloat — integer-based to avoid FP error (§9.6). Unchanged.
 */
export function parseMoneyToMinorUnits(input: string): number | null {
  const s = input.trim();
  // optional leading digits, optional . with up to 2 decimal digits
  const m = /^(\d*)(?:\.(\d{0,2}))?$/.exec(s);
  if (!m || s === '' || s === '.') return null;
  const rupees = m[1] === '' ? 0 : parseInt(m[1], 10);
  const frac = (m[2] ?? '').padEnd(2, '0');       // "" -> "00", "5" -> "50"
  const paise = frac === '' ? 0 : parseInt(frac, 10);
  const minor = rupees * 100 + paise;
  return minor > 0 ? minor : null;                 // amounts must be > 0 (mirrors backend gt=0)
}
