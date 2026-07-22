import { useCallback, useEffect, useRef, useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import { Picker } from '@react-native-picker/picker';
import type { Allowance, CurrencyCode } from '../api';
import { currentPeriod, nextPeriod } from '../allowance-format';
import { humanMonth } from '../ledger-format';
import { formatMinor, parseMoneyToMinorUnits, currencySymbol } from '../money';
import { useSetAllowance } from '../hooks/useAllowances';
import { confirmAsync } from '../confirm';
import { theme } from '../theme';
import { Sheet } from './Sheet';
import { Button } from './Button';
import { TextField } from './TextField';
import { useToast } from './Toast';

interface AllowanceSheetProps {
  visible: boolean;
  onClose: () => void;
  groupId: string;
  /** Group currency — stamped onto the outgoing allowance amount. */
  currency: CurrencyCode;
  userId: string;
  memberName?: string;
  current: Allowance | null;
}

export function AllowanceSheet({
  visible,
  onClose,
  groupId,
  currency,
  userId,
  memberName,
  current,
}: AllowanceSheetProps) {
  const { show: showToast } = useToast();
  const setAllowance = useSetAllowance(groupId);

  const thisPeriod = currentPeriod();
  const nextP = nextPeriod(thisPeriod);

  const initialAmount = useCallback(
    () => (current && current.amount.value > 0 ? formatMinor(current.amount.value) : ''),
    [current],
  );

  const [amountStr, setAmountStr] = useState(initialAmount);
  const [amountError, setAmountError] = useState('');
  const [effectiveMonth, setEffectiveMonth] = useState<string>(thisPeriod);

  // Re-prefill from the latest `current` each time the sheet opens (the allowance
  // may load/refetch after this component first mounts — e.g. a cold cache on web
  // page-refresh, or a change we just PUT). Guarded to the closed→open edge so a
  // background refetch never wipes what the head is mid-typing.
  const prevVisible = useRef(visible);
  useEffect(() => {
    if (visible && !prevVisible.current) {
      setAmountStr(initialAmount());
      setAmountError('');
      setEffectiveMonth(thisPeriod);
    }
    prevVisible.current = visible;
  }, [visible, initialAmount, thisPeriod]);

  function resetForm() {
    setAmountStr(initialAmount());
    setAmountError('');
    setEffectiveMonth(thisPeriod);
  }

  function handleClose() {
    resetForm();
    onClose();
  }

  async function handleSave() {
    setAmountError('');
    const value = parseMoneyToMinorUnits(amountStr);
    if (value === null) {
      setAmountError('Enter a valid amount (e.g. 500).');
      return;
    }
    try {
      await setAllowance.mutateAsync({ userId, amount: { currency, value }, effective_from: effectiveMonth });
      showToast({ tone: 'success', message: 'Pocket money updated' });
      handleClose();
    } catch (e) {
      showToast({ tone: 'danger', message: e instanceof Error ? e.message : 'Failed to update' });
    }
  }

  async function handlePause() {
    const confirmed = await confirmAsync({
      title: 'Pause pocket money',
      message: 'No pocket money will be posted from the selected month until you set a new amount.',
      confirmLabel: 'Pause',
      destructive: true,
    });
    if (!confirmed) return;
    try {
      await setAllowance.mutateAsync({ userId, amount: { currency, value: 0 }, effective_from: effectiveMonth });
      showToast({ tone: 'success', message: 'Pocket money updated' });
      handleClose();
    } catch (e) {
      showToast({ tone: 'danger', message: e instanceof Error ? e.message : 'Failed to pause' });
    }
  }

  const isPending = setAllowance.isPending;
  const canPause = current !== null && current.amount.value > 0;

  return (
    <Sheet
      visible={visible}
      onClose={handleClose}
      title={memberName ? `Pocket money — ${memberName}` : 'Pocket money'}
      footer={
        <View style={styles.footerRow}>
          <Button title="Cancel" variant="ghost" onPress={handleClose} fullWidth />
          <Button
            title="Save"
            variant="primary"
            onPress={handleSave}
            loading={isPending}
            disabled={isPending}
            fullWidth
          />
        </View>
      }
    >
      <TextField
        label={`Monthly amount (${currencySymbol(currency)})`}
        keyboardType="decimal-pad"
        value={amountStr}
        onChangeText={(v) => { setAmountStr(v); setAmountError(''); }}
        error={amountError}
        placeholder="e.g. 500"
      />

      <Text style={styles.pickerLabel}>Effective month</Text>
      <View style={styles.pickerWrap}>
        <Picker<string>
          selectedValue={effectiveMonth}
          onValueChange={setEffectiveMonth}
          style={styles.picker}
          dropdownIconColor={theme.color.textSecondary}
        >
          <Picker.Item label="This month" value={thisPeriod} />
          <Picker.Item label={`Next month (${humanMonth(nextP)})`} value={nextP} />
        </Picker>
      </View>

      {canPause && (
        <View style={styles.pauseRow}>
          <Button
            title="Pause allowance"
            variant="danger"
            onPress={handlePause}
            disabled={isPending}
            fullWidth
          />
        </View>
      )}
    </Sheet>
  );
}

const styles = StyleSheet.create({
  pickerLabel: {
    fontSize: theme.fontSize.xs,
    fontWeight: theme.fontWeight.semibold as '600',
    color: theme.color.textSecondary,
    marginBottom: theme.spacing.xs,
    marginTop: theme.spacing.md,
  },
  // Force readable selected-value text on Android (see AddEntrySheet.picker).
  picker: {
    color: theme.color.text,
  },
  pickerWrap: {
    borderWidth: 1,
    borderColor: theme.color.border,
    borderRadius: theme.radius.sm,
    marginBottom: theme.spacing.xs,
  },
  pauseRow: {
    marginTop: theme.spacing.lg,
  },
  footerRow: {
    flexDirection: 'row',
    gap: theme.spacing.sm,
  },
});
