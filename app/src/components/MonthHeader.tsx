import { Pressable, StyleSheet, Text, View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';
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
  /** Go to the previous month. When set, a ‹ control is shown (statement switcher). */
  onPrev?: () => void;
  /** Go to the next month. When set, a › control is shown (statement switcher). */
  onNext?: () => void;
  /** Disable the › control (UI never navigates into a future month). */
  nextDisabled?: boolean;
}

function formatPeriod(period: string): string {
  const [yearStr, monthStr] = period.split('-');
  const monthIndex = parseInt(monthStr, 10) - 1;
  const name = MONTH_NAMES[monthIndex] ?? monthStr;
  return `${name} ${yearStr}`;
}

export function MonthHeader({
  period,
  totalMinorUnits,
  currency,
  totalVariant = 'neutral',
  onPrev,
  onNext,
  nextDisabled = false,
}: MonthHeaderProps) {
  const showSwitcher = onPrev !== undefined || onNext !== undefined;
  return (
    <View style={styles.row}>
      {showSwitcher ? (
        <View style={styles.labelGroup}>
          <Pressable
            onPress={onPrev}
            disabled={onPrev === undefined}
            hitSlop={8}
            style={styles.arrow}
            testID="statement-prev-month"
          >
            <Ionicons name="chevron-back" size={20} color={theme.color.textSecondary} />
          </Pressable>
          <Text style={styles.label}>{formatPeriod(period)}</Text>
          <Pressable
            onPress={onNext}
            disabled={onNext === undefined || nextDisabled}
            hitSlop={8}
            style={styles.arrow}
            testID="statement-next-month"
          >
            <Ionicons
              name="chevron-forward"
              size={20}
              color={onNext === undefined || nextDisabled ? theme.color.textMuted : theme.color.textSecondary}
            />
          </Pressable>
        </View>
      ) : (
        <Text style={styles.label}>{formatPeriod(period)}</Text>
      )}
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
  labelGroup: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: theme.spacing.sm,
  },
  arrow: {
    padding: theme.spacing.xs,
  },
  label: {
    fontSize: theme.fontSize.sm,
    fontWeight: theme.fontWeight.semibold,
    color: theme.color.textSecondary,
  },
});
