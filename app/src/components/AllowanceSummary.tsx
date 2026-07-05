import { StyleSheet, Text, View } from 'react-native';
import type { Allowance } from '../api';
import { describeAllowance } from '../allowance-format';
import { humanMonth } from '../ledger-format';
import { theme } from '../theme';
import { AmountText } from './AmountText';
import { Button } from './Button';

interface AllowanceSummaryProps {
  current: Allowance | null;
  upcoming?: Allowance | null;
  onEdit?: () => void;
}

export function AllowanceSummary({ current, upcoming, onEdit }: AllowanceSummaryProps) {
  return (
    <View style={styles.container}>
      <View style={styles.row}>
        <Text style={styles.label}>Pocket money</Text>
        <View style={styles.valueRow}>
          {current === null ? (
            <Text style={styles.muted}>Not set</Text>
          ) : current.amount === 0 ? (
            <Text style={styles.muted}>Paused</Text>
          ) : (
            <View style={styles.amountRow}>
              <AmountText minorUnits={current.amount} variant="neutral" />
              <Text style={styles.perMonth}> / month</Text>
            </View>
          )}
          {onEdit && (
            <Button
              title={current ? 'Edit' : 'Set'}
              variant="ghost"
              size="sm"
              icon="pencil"
              onPress={onEdit}
              testID="member-allowance-button"
            />
          )}
        </View>
      </View>
      {upcoming && (
        <Text style={styles.upcomingCaption}>
          {`Changing to ${describeAllowance(upcoming)} from ${humanMonth(upcoming.effective_from)}`}
        </Text>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    paddingHorizontal: theme.spacing.lg,
    paddingVertical: theme.spacing.sm,
  },
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
  },
  label: {
    fontSize: theme.fontSize.sm,
    color: theme.color.textSecondary,
  },
  valueRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: theme.spacing.xs,
  },
  amountRow: {
    flexDirection: 'row',
    alignItems: 'baseline',
  },
  perMonth: {
    fontSize: theme.fontSize.xs,
    color: theme.color.textSecondary,
  },
  muted: {
    fontSize: theme.fontSize.sm,
    color: theme.color.textMuted,
  },
  upcomingCaption: {
    fontSize: theme.fontSize.xs,
    color: theme.color.textSecondary,
    marginTop: theme.spacing.xs,
  },
});
