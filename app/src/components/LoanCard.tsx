import { StyleSheet, Text, View } from 'react-native';
import { theme } from '../theme';
import type { Loan, CurrencyCode } from '../api';
import { formatMoney } from '../money';
import { loanRepaid } from '../loan-format';
import { Card } from './Card';
import { AmountText } from './AmountText';
import { StatusBadge } from './StatusBadge';

export type LoanTone = 'neutral' | 'success' | 'warning' | 'danger' | 'info';

export function loanStatusTone(status: Loan['status']): LoanTone {
  switch (status) {
    case 'requested': return 'warning';
    case 'active':    return 'success';
    case 'rejected':  return 'danger';
    case 'closed':    return 'neutral';
  }
}

export function loanStatusLabel(status: Loan['status']): string {
  switch (status) {
    case 'requested': return 'Pending';
    case 'active':    return 'Active';
    case 'rejected':  return 'Rejected';
    case 'closed':    return 'Closed';
  }
}

export interface LoanCardProps {
  loan: Loan;
  /** Borrower name — shown in the header (admin lists). Omit on a self view. */
  memberName?: string;
  currency: CurrencyCode;
  /** When set, the whole card is tappable (→ loan detail). */
  onPress?: () => void;
  /** Admin action buttons (approve/reject/close) rendered below the details. */
  actions?: React.ReactNode;
}

/**
 * One loan, consolidated from the former inline renderers in loans.tsx and
 * members/[userId].tsx. Preserves `loan-card-${id}` and adds a repaid/pending
 * progress line (`loan-progress-${id}`) so e2e can assert both figures without
 * brittle text scans. Schedule detail lives in LoanScheduleList (loan detail).
 */
export function LoanCard({ loan, memberName, currency, onPress, actions }: LoanCardProps) {
  const isActiveOrClosed = loan.status === 'active' || loan.status === 'closed';
  const repaid = loanRepaid(loan);

  return (
    <View testID={`loan-card-${loan.id}`}>
      <Card onPress={onPress} style={styles.card}>
        <View style={styles.header}>
          <View style={styles.headerLeft}>
            <StatusBadge label={loanStatusLabel(loan.status)} tone={loanStatusTone(loan.status)} />
            {memberName ? <Text style={styles.memberName}>{memberName}</Text> : null}
          </View>
          <AmountText minorUnits={loan.principal.value} currency={loan.principal.currency} variant="neutral" size="lg" />
        </View>

        {loan.note ? <Text style={styles.note}>{loan.note}</Text> : null}

        <View style={styles.details}>
          {isActiveOrClosed && (
            <>
              <View style={styles.detailRow}>
                <Text style={styles.detailLabel}>Outstanding</Text>
                <AmountText minorUnits={loan.outstanding.value} currency={loan.outstanding.currency} variant="debit" size="sm" />
              </View>
              <View style={styles.detailRow}>
                <Text style={styles.detailLabel}>EMI</Text>
                <Text style={styles.detailValue}>
                  ≈ {formatMoney(loan.emi_amount.value, loan.emi_amount.currency)} / month
                </Text>
              </View>
              <View style={styles.detailRow}>
                <Text style={styles.detailLabel}>Progress</Text>
                <Text style={styles.detailValue}>
                  {loan.installments_posted}/{loan.installments} paid
                </Text>
              </View>
              <View style={styles.detailRow} testID={`loan-progress-${loan.id}`}>
                <Text style={styles.detailLabel}>
                  Repaid <AmountText minorUnits={repaid} currency={loan.principal.currency} variant="neutral" size="sm" />
                </Text>
                <Text style={styles.detailLabel}>
                  Pending <AmountText minorUnits={loan.outstanding.value} currency={loan.outstanding.currency} variant="neutral" size="sm" />
                </Text>
              </View>
            </>
          )}
          {loan.status === 'requested' && (
            <>
              <View style={styles.detailRow}>
                <Text style={styles.detailLabel}>Requested</Text>
                <Text style={styles.detailValue}>
                  {formatMoney(loan.principal.value, loan.principal.currency)} over {loan.installments} months
                </Text>
              </View>
              <View style={styles.detailRow}>
                <Text style={styles.detailLabel}>Est. EMI</Text>
                <Text style={styles.detailValue}>
                  ≈ {formatMoney(loan.emi_amount.value, loan.emi_amount.currency)} / month
                </Text>
              </View>
            </>
          )}
        </View>

        {actions}
      </Card>
    </View>
  );
}

const styles = StyleSheet.create({
  card: {
    marginBottom: theme.spacing.md,
  },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: theme.spacing.sm,
  },
  headerLeft: {
    flexDirection: 'column',
    gap: theme.spacing.xs,
  },
  memberName: {
    fontSize: theme.fontSize.sm,
    fontWeight: theme.fontWeight.medium,
    color: theme.color.textSecondary,
  },
  note: {
    fontSize: theme.fontSize.sm,
    color: theme.color.textSecondary,
    fontStyle: 'italic',
    marginBottom: theme.spacing.sm,
  },
  details: {
    gap: theme.spacing.xs,
    marginBottom: theme.spacing.sm,
  },
  detailRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  detailLabel: {
    fontSize: theme.fontSize.sm,
    color: theme.color.textSecondary,
  },
  detailValue: {
    fontSize: theme.fontSize.sm,
    color: theme.color.text,
    fontWeight: theme.fontWeight.medium,
  },
});
