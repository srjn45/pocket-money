// Shared contract for the platform-split DateField (DateField.tsx = native
// calendar picker, DateField.web.tsx = typed field). The value is always a plain
// calendar date string 'YYYY-MM-DD' (or '' for empty), which is exactly what the
// backend's occurred_at parser and the HTML <input type="date"> both expect — so
// the value contract is identical across platforms.
export interface DateFieldProps {
  label?: string;
  /** 'YYYY-MM-DD', or '' when unset (caller supplies the default, e.g. today). */
  value: string;
  onChange: (next: string) => void;
  placeholder?: string;
  /** Inline error shown below the control. */
  error?: string;
  /** Latest selectable date, inclusive (native picker clamps to it). */
  maximumDate?: Date;
  testID?: string;
}

// Parse 'YYYY-MM-DD' → a LOCAL-midnight Date (calendar date, no time zone shift).
// Returns null for malformed or rolled-over input (e.g. 2026-02-31).
export function ymdToDate(v: string): Date | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(v);
  if (!m) return null;
  const y = Number(m[1]);
  const mo = Number(m[2]);
  const da = Number(m[3]);
  const dt = new Date(y, mo - 1, da);
  // Reject rolled-over dates: JS Date silently normalises 2026-02-31 → 2026-03-03.
  if (dt.getFullYear() !== y || dt.getMonth() !== mo - 1 || dt.getDate() !== da) {
    return null;
  }
  return dt;
}

// Format a Date's LOCAL calendar day as 'YYYY-MM-DD'.
export function dateToYmd(d: Date): string {
  const y = d.getFullYear();
  const mo = String(d.getMonth() + 1).padStart(2, '0');
  const da = String(d.getDate()).padStart(2, '0');
  return `${y}-${mo}-${da}`;
}
