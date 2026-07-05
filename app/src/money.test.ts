import { parseMoneyToMinorUnits, formatMinorUnits } from './money';

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

// formatMinorUnits
assert(eq(formatMinorUnits(1250), '₹12.50'), '1250 -> ₹12.50');
assert(eq(formatMinorUnits(100), '₹1.00'), '100 -> ₹1.00');
assert(eq(formatMinorUnits(5), '₹0.05'), '5 -> ₹0.05');
assert(eq(formatMinorUnits(-500), '-₹5.00'), '-500 -> -₹5.00');
assert(eq(formatMinorUnits(0), '₹0.00'), '0 -> ₹0.00');

console.log('All parseMoneyToMinorUnits tests passed.');
