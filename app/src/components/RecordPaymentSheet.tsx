import { useEffect, useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import { theme } from '../theme';
import type { CurrencyCode, MemberStatement } from '../api';
import { formatMinor, parseMoneyToMinorUnits } from '../money';
import { useCreateLedgerEntry } from '../hooks/useLedger';
import { Sheet } from './Sheet';
import { Button } from './Button';
import { TextField } from './TextField';
import { AmountText } from './AmountText';
import { useToast } from './Toast';

interface RecordPaymentSheetProps {
  visible: boolean;
  onClose: () => void;
  groupId: string;
  /** The group's currency (D7) — stamped on the outgoing settlement amount. */
  currency: CurrencyCode;
  /** The member being paid; `closing_balance` pre-fills the amount (the remaining). */
  member: MemberStatement | null;
}

/**
 * Single-purpose sheet: record a payment to one member. Amount pre-fills with the
 * member's remaining (`closing_balance`); Save posts a `settlement` debit of that
 * amount so `cleared += remaining` and the recomputed `closing_balance → 0`
 * (V3-4.2 acceptance). Reuses useCreateLedgerEntry — same mutation/currency-stamp/
 * invalidation path as AddEntrySheet, which stays untouched (no regression to T4).
 */
export function RecordPaymentSheet({ visible, onClose, groupId, currency, member }: RecordPaymentSheetProps) {
  const { show: showToast } = useToast();
  const createEntry = useCreateLedgerEntry(groupId);
  const [amountStr, setAmountStr] = useState('');
  const [note, setNote] = useState('');

  // Re-seed the amount from the remaining each time the sheet opens for a member.
  useEffect(() => {
    if (visible && member) {
      setAmountStr(formatMinor(member.closing_balance.value));
      setNote('');
    }
  }, [visible, member]);

  function handleClose() {
    setAmountStr('');
    setNote('');
    onClose();
  }

  async function handleSubmit() {
    if (!member) return;
    const value = parseMoneyToMinorUnits(amountStr);
    if (value === null) {
      showToast({ message: 'Enter a valid amount (e.g. 12.50).', tone: 'danger' });
      return;
    }
    try {
      await createEntry.mutateAsync({
        entry_type: 'settlement',
        user_id: member.user_id,
        amount: { currency, value },
        note: note || undefined,
      });
      showToast({ message: 'Payment recorded', tone: 'success' });
      handleClose();
    } catch (e) {
      showToast({
        message: e instanceof Error ? e.message : 'Failed to record payment',
        tone: 'danger',
      });
    }
  }

  const isSubmitting = createEntry.isPending;

  return (
    <Sheet
      visible={visible}
      onClose={handleClose}
      title="Record payment"
      footer={
        <View style={styles.footerRow}>
          <Button title="Cancel" variant="ghost" onPress={handleClose} fullWidth />
          <Button
            title="Save"
            variant="primary"
            onPress={handleSubmit}
            loading={isSubmitting}
            disabled={isSubmitting}
            fullWidth
            testID="record-payment-submit"
          />
        </View>
      }
    >
      {member && (
        <>
          <Text style={styles.payingLabel}>Paying {member.name}</Text>
          <View style={styles.remainingRow}>
            <Text style={styles.remainingLabel}>Remaining</Text>
            <AmountText
              minorUnits={member.closing_balance.value}
              currency={currency}
              variant={member.closing_balance.value < 0 ? 'debit' : 'credit'}
              size="md"
            />
          </View>
          <TextField
            label="Amount"
            value={amountStr}
            onChangeText={setAmountStr}
            keyboardType="decimal-pad"
            placeholder="e.g. 12.50"
            testID="record-payment-amount"
          />
          <TextField
            label="Note (optional)"
            value={note}
            onChangeText={setNote}
            placeholder="Optional note"
          />
        </>
      )}
    </Sheet>
  );
}

const styles = StyleSheet.create({
  payingLabel: {
    fontSize: theme.fontSize.md,
    fontWeight: theme.fontWeight.semibold,
    color: theme.color.text,
    marginBottom: theme.spacing.md,
  },
  remainingRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: theme.spacing.md,
  },
  remainingLabel: {
    fontSize: theme.fontSize.sm,
    color: theme.color.textSecondary,
  },
  footerRow: {
    flexDirection: 'row',
    gap: theme.spacing.sm,
  },
});
