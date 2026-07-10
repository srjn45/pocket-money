import { StyleSheet, Text, View } from 'react-native';
import { theme } from '../theme';
import type { CurrencyCode, MemberStatement } from '../api';
import { Card } from './Card';
import { Avatar } from './Avatar';
import { Button } from './Button';
import { AmountText } from './AmountText';
import { StatusBadge } from './StatusBadge';

interface StatementRowProps {
  /** One member's derived figures for the month (GET /groups/{id}/statement). */
  member: MemberStatement;
  /** The group's currency (D7) — stamped on every figure via AmountText. */
  currency: CurrencyCode;
  /** Render the "Not registered" shadow badge (member status from useGroup, joined by user_id). */
  isShadow?: boolean;
  /** Drill into member detail (admin only). Omitted → the row is not pressable (member's own row). */
  onPress?: () => void;
  /** Open the Record-payment sheet. Present only on the admin view for the current month. */
  onRecordPayment?: () => void;
}

/**
 * One member's monthly statement row: base · chores · −EMI → payable · paid ·
 * remaining, mapped to the API's total_due / cleared / closing_balance (V3-4.2
 * §1.2 — payable/remaining fold in opening_balance + adjustments so the row's
 * own math reconciles). Replaces MemberCard on the group home; keeps the
 * `member-card-${uid}` root testID so the existing drill-in call-sites resolve
 * unchanged (rename to statement-row-* deferred to V3-4.4).
 */
export function StatementRow({ member, currency, isShadow, onPress, onRecordPayment }: StatementRowProps) {
  const uid = member.user_id;

  return (
    <View testID={`member-card-${uid}`} style={styles.wrap}>
      <Card onPress={onPress} padded={false}>
        <View style={styles.header}>
          <Avatar name={member.name} id={uid} size={44} />
          <View style={styles.nameCol}>
            <Text style={styles.name}>{member.name}</Text>
            {isShadow && (
              <View testID={`member-shadow-badge-${uid}`}>
                <StatusBadge label="Not registered" tone="neutral" />
              </View>
            )}
          </View>
        </View>

        {/* Breakdown (de-emphasized): the compact §5 label + carryover/adjustments. */}
        <View style={styles.breakdown}>
          <Figure label="Carried" minorUnits={member.opening_balance.value} currency={currency} signed muted />
          <Figure label="Base" minorUnits={member.base.value} currency={currency} muted />
          <Figure label="Chores" minorUnits={member.chores.value} currency={currency} muted />
          <Figure label="− EMI" minorUnits={member.emi.value} currency={currency} muted />
          <Figure label="± Adjust" minorUnits={member.adjustments.value} currency={currency} signed muted />
        </View>

        {/* Authoritative figures: payable · paid · remaining. */}
        <View style={styles.figures}>
          <View style={styles.figureCell}>
            <Text style={styles.figureLabel}>Payable</Text>
            <View testID={`statement-payable-${uid}`}>
              <AmountText minorUnits={member.total_due.value} currency={currency} variant="neutral" size="md" />
            </View>
          </View>
          <View style={styles.figureCell}>
            <Text style={styles.figureLabel}>Paid</Text>
            <View testID={`statement-cleared-${uid}`}>
              <AmountText minorUnits={member.cleared.value} currency={currency} variant="neutral" size="md" />
            </View>
          </View>
          <View style={styles.figureCell}>
            <Text style={styles.figureLabel}>Remaining</Text>
            <View testID={`statement-remaining-${uid}`}>
              <AmountText
                minorUnits={member.closing_balance.value}
                currency={currency}
                variant={member.closing_balance.value < 0 ? 'debit' : member.closing_balance.value === 0 ? 'neutral' : 'credit'}
                size="md"
              />
            </View>
          </View>
        </View>
      </Card>

      {onRecordPayment && (
        <View style={styles.paymentRow}>
          <Button
            title="Record payment"
            variant="secondary"
            icon="cash-outline"
            size="sm"
            onPress={onRecordPayment}
            fullWidth
            testID={`statement-record-payment-${uid}`}
          />
        </View>
      )}
    </View>
  );
}

function Figure({
  label,
  minorUnits,
  currency,
  signed = false,
  muted = false,
}: {
  label: string;
  minorUnits: number;
  currency: CurrencyCode;
  signed?: boolean;
  muted?: boolean;
}) {
  return (
    <View style={styles.breakdownCell}>
      <Text style={[styles.breakdownLabel, muted && styles.mutedLabel]}>{label}</Text>
      <AmountText
        minorUnits={minorUnits}
        currency={currency}
        variant={signed ? (minorUnits < 0 ? 'debit' : minorUnits === 0 ? 'neutral' : 'credit') : 'neutral'}
        size="sm"
      />
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: {
    marginHorizontal: theme.spacing.lg,
    marginBottom: theme.spacing.sm,
  },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: theme.spacing.md,
    padding: theme.spacing.lg,
    paddingBottom: theme.spacing.md,
  },
  nameCol: {
    flex: 1,
    gap: theme.spacing.xs,
  },
  name: {
    fontSize: theme.fontSize.md,
    fontWeight: theme.fontWeight.semibold,
    color: theme.color.text,
  },
  breakdown: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: theme.spacing.md,
    paddingHorizontal: theme.spacing.lg,
    paddingBottom: theme.spacing.md,
  },
  breakdownCell: {
    gap: 2,
  },
  breakdownLabel: {
    fontSize: theme.fontSize.xs,
    color: theme.color.textSecondary,
  },
  mutedLabel: {
    color: theme.color.textMuted,
  },
  figures: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    paddingHorizontal: theme.spacing.lg,
    paddingBottom: theme.spacing.lg,
    borderTopWidth: StyleSheet.hairlineWidth,
    borderTopColor: theme.color.border,
    paddingTop: theme.spacing.md,
  },
  figureCell: {
    alignItems: 'flex-start',
    gap: 2,
  },
  figureLabel: {
    fontSize: theme.fontSize.xs,
    color: theme.color.textSecondary,
  },
  paymentRow: {
    marginTop: theme.spacing.xs,
  },
});
