import { StyleSheet, Text, View } from 'react-native';
import { theme } from '../theme';
import type { Loan, CurrencyCode } from '../api';
import { buildLoanSchedule, type InstallmentStatus } from '../loan-format';
import { humanMonth } from '../ledger-format';
import { AmountText } from './AmountText';
import { StatusBadge } from './StatusBadge';

const STATUS_LABEL: Record<InstallmentStatus, string> = {
  paid: 'Paid',
  due: 'Due',
  pending: 'Pending',
};

const STATUS_TONE: Record<InstallmentStatus, 'neutral' | 'success' | 'warning'> = {
  paid: 'success',
  due: 'warning',
  pending: 'neutral',
};

export interface LoanScheduleListProps {
  loan: Loan;
  currency: CurrencyCode;
}

/**
 * Repayment schedule derived from the loan primitives (buildLoanSchedule, D5 —
 * there is no `schedule` field on the API). One row per installment.
 */
export function LoanScheduleList({ loan, currency }: LoanScheduleListProps) {
  const schedule = buildLoanSchedule(loan);

  return (
    <View testID="loan-schedule" style={styles.container}>
      <Text style={styles.header}>Repayment schedule</Text>
      {loan.start_period === null && (
        <Text style={styles.hint}>Starts once the loan is approved.</Text>
      )}
      {schedule.map(inst => (
        <View
          key={inst.index}
          testID={`loan-installment-${loan.id}-${inst.index}`}
          style={styles.row}
        >
          <Text style={styles.index}>#{inst.index}</Text>
          <Text style={styles.period}>
            {inst.duePeriod ? humanMonth(inst.duePeriod) : '—'}
          </Text>
          <AmountText minorUnits={inst.amount} currency={currency} variant="neutral" size="sm" />
          <View style={styles.badgeWrap}>
            <StatusBadge label={STATUS_LABEL[inst.status]} tone={STATUS_TONE[inst.status]} />
          </View>
        </View>
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    padding: theme.spacing.lg,
    gap: theme.spacing.sm,
  },
  header: {
    fontSize: theme.fontSize.sm,
    fontWeight: theme.fontWeight.semibold,
    color: theme.color.textSecondary,
    marginBottom: theme.spacing.xs,
  },
  hint: {
    fontSize: theme.fontSize.xs,
    color: theme.color.textSecondary,
    fontStyle: 'italic',
    marginBottom: theme.spacing.xs,
  },
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: theme.spacing.sm,
    paddingVertical: theme.spacing.sm,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: theme.color.border,
  },
  index: {
    fontSize: theme.fontSize.sm,
    fontWeight: theme.fontWeight.semibold,
    color: theme.color.text,
    width: 36,
  },
  period: {
    fontSize: theme.fontSize.sm,
    color: theme.color.textSecondary,
    flex: 1,
  },
  badgeWrap: {
    minWidth: 72,
    alignItems: 'flex-end',
  },
});
