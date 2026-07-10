import { StyleSheet, Text, View } from 'react-native';
import { Picker } from '@react-native-picker/picker';
import { useState } from 'react';
import { theme } from '../theme';
import type { Chore, Member, CurrencyCode } from '../api';
import { parseMoneyToMinorUnits } from '../money';
import { useCreateLedgerEntry } from '../hooks/useLedger';
import { Sheet } from './Sheet';
import { Button } from './Button';
import { TextField } from './TextField';
import { useToast } from './Toast';
import { AmountText } from './AmountText';

export interface AddEntrySheetProps {
  visible: boolean;
  onClose: () => void;
  groupId: string;
  /** The group's currency — stamped onto outgoing settlement/adjustment amounts. */
  currency: CurrencyCode;
  chores: Chore[];
  mode: 'head' | 'member';
  members?: Member[];
  fixedUserId?: string;
  selfUserId?: string;
}

type EntryKind = 'chore' | 'settlement' | 'adjustment';
type Direction = 'credit' | 'debit';

export function AddEntrySheet({
  visible,
  onClose,
  groupId,
  currency,
  chores,
  mode,
  members = [],
  fixedUserId,
  selfUserId,
}: AddEntrySheetProps) {
  const { show: showToast } = useToast();
  const createEntry = useCreateLedgerEntry(groupId);

  const nonSystemChores = chores.filter(c => !c.is_system);

  const [kind, setKind] = useState<EntryKind>('chore');
  const [selectedChoreId, setSelectedChoreId] = useState(nonSystemChores[0]?.id ?? '');
  const [selectedMemberId, setSelectedMemberId] = useState(
    fixedUserId ?? members[0]?.user_id ?? ''
  );
  const [amountStr, setAmountStr] = useState('');
  const [direction, setDirection] = useState<Direction>('credit');
  const [note, setNote] = useState('');

  const selectedChore = nonSystemChores.find(c => c.id === selectedChoreId);

  function resetForm() {
    setKind('chore');
    setSelectedChoreId(nonSystemChores[0]?.id ?? '');
    setSelectedMemberId(fixedUserId ?? members[0]?.user_id ?? '');
    setAmountStr('');
    setDirection('credit');
    setNote('');
  }

  function handleClose() {
    resetForm();
    onClose();
  }

  function validate(): string | null {
    if (mode === 'head' && !fixedUserId && !selectedMemberId) {
      return 'Please select a member.';
    }
    if (kind === 'chore' && !selectedChoreId) {
      return 'Please select a chore.';
    }
    if ((kind === 'settlement' || kind === 'adjustment')) {
      const parsed = parseMoneyToMinorUnits(amountStr);
      if (parsed === null) return 'Enter a valid amount (e.g. 12.50).';
    }
    return null;
  }

  async function handleSubmit() {
    const err = validate();
    if (err) {
      showToast({ message: err, tone: 'danger' });
      return;
    }

    const userId = mode === 'head'
      ? (fixedUserId ?? selectedMemberId)
      : selfUserId;

    try {
      if (kind === 'chore') {
        await createEntry.mutateAsync({
          entry_type: 'chore',
          user_id: mode === 'head' ? userId : undefined,
          chore_id: selectedChoreId,
        });
      } else if (kind === 'settlement') {
        const value = parseMoneyToMinorUnits(amountStr)!;
        await createEntry.mutateAsync({
          entry_type: 'settlement',
          user_id: userId,
          amount: { currency, value },
          note: note || undefined,
        });
      } else {
        const value = parseMoneyToMinorUnits(amountStr)!;
        await createEntry.mutateAsync({
          entry_type: 'adjustment',
          user_id: userId,
          amount: { currency, value },
          direction,
          note: note || undefined,
        });
      }
      showToast({ message: 'Entry added', tone: 'success' });
      handleClose();
    } catch (e) {
      showToast({
        message: e instanceof Error ? e.message : 'Failed to add entry',
        tone: 'danger',
      });
    }
  }

  const isSubmitting = createEntry.isPending;

  return (
    <Sheet
      visible={visible}
      onClose={handleClose}
      title="Add entry"
      footer={
        <View style={styles.footerRow}>
          <Button
            title="Cancel"
            variant="ghost"
            onPress={handleClose}
            fullWidth
          />
          <Button
            title="Save"
            variant="primary"
            onPress={handleSubmit}
            loading={isSubmitting}
            disabled={isSubmitting}
            fullWidth
            testID="entry-submit"
          />
        </View>
      }
    >
      {mode === 'head' && (
        <>
          <Text style={styles.label}>Entry type</Text>
          <View style={styles.pickerWrap}>
            <Picker<EntryKind>
              selectedValue={kind}
              onValueChange={setKind}
              testID="entry-type-picker"
            >
              <Picker.Item label="Chore" value="chore" />
              <Picker.Item label="Settlement" value="settlement" />
              <Picker.Item label="Adjustment" value="adjustment" />
            </Picker>
          </View>
        </>
      )}

      {mode === 'head' && !fixedUserId && (
        <>
          <Text style={styles.label}>Member</Text>
          <View style={styles.pickerWrap}>
            <Picker<string>
              selectedValue={selectedMemberId}
              onValueChange={setSelectedMemberId}
            >
              {members.filter(m => m.role !== 'admin').map(m => (
                <Picker.Item key={m.user_id} label={m.name} value={m.user_id} />
              ))}
            </Picker>
          </View>
        </>
      )}

      {kind === 'chore' && (
        <>
          <Text style={styles.label}>Chore</Text>
          {nonSystemChores.length === 0 ? (
            <Text style={styles.hint}>No chores available. Create chores first.</Text>
          ) : (
            <View style={styles.pickerWrap}>
              <Picker<string>
                selectedValue={selectedChoreId}
                onValueChange={setSelectedChoreId}
                testID="entry-chore-picker"
              >
                {nonSystemChores.map(c => (
                  <Picker.Item
                    key={c.id}
                    label={`${c.name}`}
                    value={c.id}
                  />
                ))}
              </Picker>
            </View>
          )}
          {selectedChore && (
            <View style={styles.choreAmountRow}>
              <Text style={styles.choreAmountLabel}>Amount:</Text>
              <AmountText minorUnits={selectedChore.amount.value} currency={selectedChore.amount.currency} variant="neutral" />
            </View>
          )}
          {mode === 'member' && (
            <Text style={styles.hint}>This will be sent to the admin for approval.</Text>
          )}
        </>
      )}

      {(kind === 'settlement' || kind === 'adjustment') && (
        <>
          <TextField
            label="Amount"
            value={amountStr}
            onChangeText={setAmountStr}
            keyboardType="decimal-pad"
            placeholder="e.g. 12.50"
            testID="entry-amount"
          />
        </>
      )}

      {kind === 'adjustment' && (
        <>
          <Text style={styles.label}>Direction</Text>
          <View style={styles.pickerWrap}>
            <Picker<Direction>
              selectedValue={direction}
              onValueChange={setDirection}
              testID="entry-direction-picker"
            >
              <Picker.Item label="Add to balance (credit)" value="credit" />
              <Picker.Item label="Subtract from balance (debit)" value="debit" />
            </Picker>
          </View>
        </>
      )}

      {(kind === 'settlement' || kind === 'adjustment') && (
        <TextField
          label={kind === 'adjustment' ? 'Note (why)' : 'Note (optional)'}
          value={note}
          onChangeText={setNote}
          placeholder="Optional note"
        />
      )}
    </Sheet>
  );
}

const styles = StyleSheet.create({
  label: {
    fontSize: 13,
    fontWeight: '600',
    color: theme.color.textSecondary,
    marginBottom: 6,
    marginTop: 12,
  },
  pickerWrap: {
    borderWidth: 1,
    borderColor: theme.color.border,
    borderRadius: 8,
    marginBottom: 4,
  },
  hint: {
    fontSize: 13,
    color: theme.color.warning,
    marginVertical: 8,
  },
  choreAmountRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
    marginBottom: 8,
    marginTop: 4,
  },
  choreAmountLabel: {
    fontSize: 13,
    color: theme.color.textSecondary,
  },
  footerRow: {
    flexDirection: 'row',
    gap: 8,
  },
});
