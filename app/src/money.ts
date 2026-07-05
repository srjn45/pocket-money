/** Format integer minor units (paise) as a rupee string WITHOUT sign or symbol: 1250 -> "12.50". */
export function formatMinor(minorUnits: number): string {
  return (Math.abs(minorUnits) / 100).toFixed(2);
}

/**
 * INTERIM SHIM — remove in WP-1.1.
 * The API still returns money as float rupees (openapi `number`) until the minor-units
 * migration (WP-1.1 / migration 008). AmountText's contract is minor units (the end state),
 * so call sites convert with this until the API flips to integers, then delete every call.
 * Half-up rounding avoids the classic 12.345 -> 1234 float bug for the transitional period.
 */
export function rupeesToMinor(rupees: number): number {
  return Math.round(rupees * 100);
}
