import { StyleSheet, Text, View } from 'react-native';
import { theme } from '../theme';
import type { CurrencyCode } from '../api';
import { AmountText } from './AmountText';

const MONTH_NAMES = [
  'January', 'February', 'March', 'April', 'May', 'June',
  'July', 'August', 'September', 'October', 'November', 'December',
] as const;

interface MonthHeaderProps {
  /** 'YYYY-MM' (server period format, master plan §10.7). */
  period: string;
  /** Optional right-aligned month total, in minor units. */
  totalMinorUnits?: number;
  /** Currency for the total. Required when totalMinorUnits is shown (group currency). */
  currency?: CurrencyCode;
  /** Direction for the total's color/sign. Default 'neutral'. */
  totalVariant?: 'credit' | 'debit' | 'neutral';
}

function formatPeriod(period: string): string {
  const [yearStr, monthStr] = period.split('-');
  const monthIndex = parseInt(monthStr, 10) - 1;
  const name = MONTH_NAMES[monthIndex] ?? monthStr;
  return `${name} ${yearStr}`;
}

export function MonthHeader({ period, totalMinorUnits, currency, totalVariant = 'neutral' }: MonthHeaderProps) {
  return (
    <View style={styles.row}>
      <Text style={styles.label}>{formatPeriod(period)}</Text>
      {totalMinorUnits !== undefined && currency !== undefined ? (
        <AmountText minorUnits={totalMinorUnits} currency={currency} variant={totalVariant} size="sm" />
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: theme.spacing.lg,
    paddingVertical: theme.spacing.sm,
    backgroundColor: theme.color.surfaceMuted,
  },
  label: {
    fontSize: theme.fontSize.sm,
    fontWeight: theme.fontWeight.semibold,
    color: theme.color.textSecondary,
  },
});
