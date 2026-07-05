/** Format integer minor units (paise) as a rupee string WITHOUT sign or symbol: 1250 -> "12.50". */
export function formatMinor(minorUnits: number): string {
  return (Math.abs(minorUnits) / 100).toFixed(2);
}

/**
 * Parse a user-entered rupee string to integer minor units (paise).
 * "12.50" -> 1250, "12" -> 1200, "0.05" -> 5, ".5" -> 50, "12.5" -> 1250.
 * Returns null for invalid input (empty, non-numeric, negative, >2 decimals).
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

/** Format minor units as a rupee string with sign and symbol: 1250 -> "₹12.50", -500 -> "-₹5.00". */
export function formatMinorUnits(minor: number): string {
  const sign = minor < 0 ? '-' : '';
  const abs = Math.abs(minor);
  return `${sign}₹${Math.floor(abs / 100)}.${String(abs % 100).padStart(2, '0')}`;
}
