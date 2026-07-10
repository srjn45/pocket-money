import { parseMoneyToMinorUnits, formatMoney, formatMinorGrouped } from './money';

function assert(cond: boolean, msg: string): void {
  if (!cond) throw new Error(`FAIL: ${msg}`);
}

function eq(a: unknown, b: unknown): boolean {
  return a === b;
}

// parseMoneyToMinorUnits cases from spec §5
assert(eq(parseMoneyToMinorUnits('12.50'), 1250), '12.50 -> 1250');
assert(eq(parseMoneyToMinorUnits('12'), 1200), '12 -> 1200');
assert(eq(parseMoneyToMinorUnits('12.5'), 1250), '12.5 -> 1250');
assert(eq(parseMoneyToMinorUnits('0.05'), 5), '0.05 -> 5');
assert(eq(parseMoneyToMinorUnits('.5'), 50), '.5 -> 50');
assert(eq(parseMoneyToMinorUnits('0.99'), 99), '0.99 -> 99');
assert(eq(parseMoneyToMinorUnits('1'), 100), '1 -> 100');
assert(eq(parseMoneyToMinorUnits('100'), 10000), '100 -> 10000');

// null cases
assert(eq(parseMoneyToMinorUnits(''), null), 'empty -> null');
assert(eq(parseMoneyToMinorUnits('.'), null), '. -> null');
assert(eq(parseMoneyToMinorUnits('0'), null), '0 -> null (must be > 0)');
assert(eq(parseMoneyToMinorUnits('0.00'), null), '0.00 -> null');
assert(eq(parseMoneyToMinorUnits('abc'), null), 'abc -> null');
assert(eq(parseMoneyToMinorUnits('12.999'), null), '12.999 -> null (>2 decimals)');
assert(eq(parseMoneyToMinorUnits('-5'), null), '-5 -> null (negative)');
assert(eq(parseMoneyToMinorUnits('12.3.4'), null), '12.3.4 -> null');

// formatMinorGrouped — en grouping, deterministic across locales
assert(eq(formatMinorGrouped(152000), '1,520.00'), '152000 -> 1,520.00');
assert(eq(formatMinorGrouped(1250), '12.50'), '1250 -> 12.50');
assert(eq(formatMinorGrouped(5), '0.05'), '5 -> 0.05');
assert(eq(formatMinorGrouped(100000000), '1,000,000.00'), '100000000 -> 1,000,000.00');
assert(eq(formatMinorGrouped(0), '0.00'), '0 -> 0.00');

// formatMoney — symbol by currency, signed by numeric sign
assert(eq(formatMoney(152000, 'INR'), '₹1,520.00'), '152000 INR -> ₹1,520.00');
assert(eq(formatMoney(152000, 'EUR'), '€1,520.00'), '152000 EUR -> €1,520.00');
assert(eq(formatMoney(152000, 'USD'), '$1,520.00'), '152000 USD -> $1,520.00');
assert(eq(formatMoney(1250, 'INR'), '₹12.50'), '1250 INR -> ₹12.50');
assert(eq(formatMoney(-500, 'USD'), '-$5.00'), '-500 USD -> -$5.00');
assert(eq(formatMoney(0, 'INR'), '₹0.00'), '0 INR -> ₹0.00');

console.log('All money tests passed.');
