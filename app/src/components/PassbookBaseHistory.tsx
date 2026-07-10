import { StyleSheet, Text, View } from 'react-native';
import { theme } from '../theme';
import type { Allowance, CurrencyCode } from '../api';
import { humanMonthFull } from '../ledger-format';
import { AmountText } from './AmountText';
import { StatusBadge } from './StatusBadge';

export interface PassbookBaseHistoryProps {
  /** Allowance rows already filtered to the viewed member. */
  allowances: Allowance[];
  currency: CurrencyCode;
}

/**
 * Base-amount (pocket-money) history for a member — every allowance history row,
 * newest first. A value-0 row renders a "Paused" pill. Renders nothing when the
 * member has no allowance rows (caller also hides it on a load error).
 */
export function PassbookBaseHistory({ allowances, currency }: PassbookBaseHistoryProps) {
  if (allowances.length === 0) return null;

  const rows = [...allowances].sort((a, b) =>
    a.effective_from < b.effective_from ? 1 : a.effective_from > b.effective_from ? -1 : 0
  );

  return (
    <View testID="base-history" style={styles.container}>
      <Text style={styles.sectionTitle}>Base amount history</Text>
      {rows.map(row => (
        <View key={row.effective_from} style={styles.row}>
          <Text style={styles.month}>{humanMonthFull(row.effective_from)}</Text>
          {row.amount.value === 0 ? (
            <StatusBadge label="Paused" tone="neutral" />
          ) : (
            <AmountText minorUnits={row.amount.value} currency={currency} variant="neutral" size="sm" />
          )}
        </View>
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    paddingHorizontal: theme.spacing.lg,
    paddingBottom: theme.spacing.md,
  },
  sectionTitle: {
    fontSize: theme.fontSize.sm,
    fontWeight: theme.fontWeight.semibold,
    color: theme.color.textSecondary,
    marginBottom: theme.spacing.sm,
  },
  row: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    paddingVertical: theme.spacing.sm,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: theme.color.border,
  },
  month: {
    fontSize: theme.fontSize.sm,
    color: theme.color.text,
  },
});
