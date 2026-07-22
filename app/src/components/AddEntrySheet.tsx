import { StyleSheet, Text, View } from 'react-native';
import { Picker } from '@react-native-picker/picker';
import { useEffect, useRef, useState } from 'react';
import { theme } from '../theme';
import type { Chore, Member, CurrencyCode, LedgerEntry } from '../api';
import { parseMoneyToMinorUnits, formatMinor, formatMoney } from '../money';
import { useCreateLedgerEntry, useEditLedgerEntry } from '../hooks/useLedger';
import { useCreateLoan } from '../hooks/useLoans';
import { Sheet } from './Sheet';
import { Button } from './Button';
import { TextField } from './TextField';
import { DateField } from './DateField';
import { useToast } from './Toast';
import { AmountText } from './AmountText';

export interface AddEntrySheetProps {
  visible: boolean;
  onClose: () => void;
  groupId: string;
  /** The group's currency — stamped onto outgoing settlement/adjustment/loan amounts. */
  currency: CurrencyCode;
  chores: Chore[];
  mode: 'head' | 'member';
  members?: Member[];
  fixedUserId?: string;
  selfUserId?: string;
  /** When set, the sheet is in edit mode (corrections, D3): type immutable, PUT /ledger/{id}. */
  editEntry?: LedgerEntry;
  /** Called with the entry id after a successful edit — passbook keeps a session "Edited" set. */
  onEdited?: (entryId: string) => void;
}

// 'loan' creates the loan ENTITY (POST /groups/{id}/loans), not a ledger row.
type EntryKind = 'chore' | 'settlement' | 'adjustment' | 'loan';
type Direction = 'credit' | 'debit';

// Optional back-date for an entry. Empty → undefined (server stamps now()).
// A valid 'YYYY-MM-DD' becomes a noon-UTC RFC3339 timestamp (noon avoids day/
// month drift when the server buckets created_at into a month). Returns an
// `error` string for malformed / rolled-over (e.g. 2026-02-31) / future dates.
function parseOccurredAt(input: string): { value?: string; error?: string } {
  const t = input.trim();
  if (!t) return {};
  if (!/^\d{4}-\d{2}-\d{2}$/.test(t)) return { error: 'Date must be YYYY-MM-DD.' };
  const d = new Date(`${t}T12:00:00Z`);
  if (Number.isNaN(d.getTime()) || d.toISOString().slice(0, 10) !== t) {
    return { error: 'Enter a valid date (YYYY-MM-DD).' };
  }
  if (d.getTime() > Date.now() + 60_000) return { error: 'Date cannot be in the future.' };
  return { value: d.toISOString() };
}

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
  editEntry,
  onEdited,
}: AddEntrySheetProps) {
  const { show: showToast } = useToast();
  const createEntry = useCreateLedgerEntry(groupId);
  const editLedger = useEditLedgerEntry(groupId);
  const createLoan = useCreateLoan(groupId);

  const isEdit = !!editEntry;
  const nonSystemChores = chores.filter(c => !c.is_system);

  const [kind, setKind] = useState<EntryKind>('chore');
  const [selectedChoreId, setSelectedChoreId] = useState(nonSystemChores[0]?.id ?? '');
  const [selectedMemberId, setSelectedMemberId] = useState(
    fixedUserId ?? members[0]?.user_id ?? ''
  );
  const [amountStr, setAmountStr] = useState('');
  const [direction, setDirection] = useState<Direction>('credit');
  const [note, setNote] = useState('');
  // Optional effective date (add-mode only; edit keeps created_at immutable).
  const [dateStr, setDateStr] = useState('');
  const [installmentsStr, setInstallmentsStr] = useState('');
  // Loan first-installment month (QA batch 1, Item 5). false = next month (default).
  const [startCurrentMonth, setStartCurrentMonth] = useState(false);

  function resetForm() {
    setKind('chore');
    setSelectedChoreId(nonSystemChores[0]?.id ?? '');
    setSelectedMemberId(fixedUserId ?? members[0]?.user_id ?? '');
    setAmountStr('');
    setDirection('credit');
    setNote('');
    setDateStr('');
    setInstallmentsStr('');
    setStartCurrentMonth(false);
  }

  // Prefill on open: edit-mode seeds from editEntry (type immutable); add-mode resets.
  const prevVisible = useRef(visible);
  useEffect(() => {
    if (visible && !prevVisible.current) {
      if (editEntry) {
        setKind(editEntry.entry_type as EntryKind);
        setAmountStr(formatMinor(editEntry.amount.value));
        setDirection(editEntry.direction);
        setNote(editEntry.note ?? '');
      } else {
        resetForm();
      }
    }
    prevVisible.current = visible;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [visible, editEntry]);

  function handleClose() {
    resetForm();
    onClose();
  }

  const selectedChore = nonSystemChores.find(c => c.id === selectedChoreId);

  // Derived EMI estimate for the "New loan" branch.
  const parsedLoanInstallments = (() => {
    const n = parseInt(installmentsStr.trim(), 10);
    return Number.isInteger(n) && n > 0 ? n : null;
  })();
  const parsedLoanPrincipal = parseMoneyToMinorUnits(amountStr);
  const loanEmiEstimate =
    kind === 'loan' && parsedLoanPrincipal !== null && parsedLoanInstallments !== null
      ? Math.ceil(parsedLoanPrincipal / parsedLoanInstallments)
      : null;

  function validate(): string | null {
    if (isEdit) {
      // EditLedgerRequest requires amount (value >= 1) even for a chore row.
      if (parseMoneyToMinorUnits(amountStr) === null) return 'Enter a valid amount (e.g. 12.50).';
      return null;
    }
    if (mode === 'head' && !fixedUserId && !selectedMemberId) {
      return 'Please select a member.';
    }
    if (kind === 'chore' && !selectedChoreId) {
      return 'Please select a chore.';
    }
    if (kind === 'settlement' || kind === 'adjustment' || kind === 'loan') {
      if (parseMoneyToMinorUnits(amountStr) === null) return 'Enter a valid amount (e.g. 12.50).';
    }
    if (kind === 'loan' && parsedLoanInstallments === null) {
      return 'Enter a whole number of months (e.g. 6).';
    }
    // Optional back-date applies to ledger rows (not the loan entity).
    if (kind !== 'loan') {
      const { error } = parseOccurredAt(dateStr);
      if (error) return error;
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
      if (isEdit) {
        const value = parseMoneyToMinorUnits(amountStr)!;
        await editLedger.mutateAsync({
          id: editEntry!.id,
          amount: { currency, value },
          // Direction is editable for adjustment only; server keeps chore/settlement's.
          direction: editEntry!.entry_type === 'adjustment' ? direction : undefined,
          note: note.trim() || null,
        });
        onEdited?.(editEntry!.id);
        showToast({ message: 'Entry updated', tone: 'success' });
        handleClose();
        return;
      }

      // Optional back-date (validated above); undefined → server stamps now().
      const occurred_at = parseOccurredAt(dateStr).value;

      if (kind === 'chore') {
        await createEntry.mutateAsync({
          entry_type: 'chore',
          user_id: mode === 'head' ? userId : undefined,
          chore_id: selectedChoreId,
          occurred_at,
        });
        showToast({ message: 'Entry added', tone: 'success' });
      } else if (kind === 'settlement') {
        const value = parseMoneyToMinorUnits(amountStr)!;
        await createEntry.mutateAsync({
          entry_type: 'settlement',
          user_id: userId,
          amount: { currency, value },
          note: note || undefined,
          occurred_at,
        });
        showToast({ message: 'Entry added', tone: 'success' });
      } else if (kind === 'adjustment') {
        const value = parseMoneyToMinorUnits(amountStr)!;
        await createEntry.mutateAsync({
          entry_type: 'adjustment',
          user_id: userId,
          amount: { currency, value },
          direction,
          note: note || undefined,
          occurred_at,
        });
        showToast({ message: 'Entry added', tone: 'success' });
      } else {
        // New loan — the loan entity, not a ledger row. Head with a fixed/selected
        // user → pre-approved active loan (CreateLoanRequest.user_id).
        const value = parseMoneyToMinorUnits(amountStr)!;
        await createLoan.mutateAsync({
          user_id: userId,
          principal: { currency, value },
          installments: parsedLoanInstallments!,
          note: note.trim() || null,
          start_current_month: startCurrentMonth,
        });
        showToast({ message: 'Loan added', tone: 'success' });
      }
      handleClose();
    } catch (e) {
      showToast({
        message: e instanceof Error ? e.message : 'Failed to add entry',
        tone: 'danger',
      });
    }
  }

  const isSubmitting = createEntry.isPending || editLedger.isPending || createLoan.isPending;
  // Amount field shows for money kinds, and in edit mode for every manual type
  // (EditLedgerRequest requires amount even for a chore correction).
  const showAmount = isEdit || kind === 'settlement' || kind === 'adjustment' || kind === 'loan';
  // Direction only when adjustment (chore/settlement keep a server-fixed direction).
  const showDirection = kind === 'adjustment';
  // Note shows in edit mode (any manual type) and in add mode for non-chore kinds.
  const showNote = isEdit || kind === 'settlement' || kind === 'adjustment' || kind === 'loan';

  return (
    <Sheet
      visible={visible}
      onClose={handleClose}
      title={isEdit ? 'Edit entry' : 'Add entry'}
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
      {mode === 'head' && !isEdit && (
        <>
          <Text style={styles.label}>Entry type</Text>
          <View style={styles.pickerWrap}>
            <Picker<EntryKind>
              selectedValue={kind}
              onValueChange={setKind}
              style={styles.picker}
              dropdownIconColor={theme.color.textSecondary}
              testID="entry-type-picker"
            >
              <Picker.Item label="Chore" value="chore" />
              <Picker.Item label="Payment" value="settlement" />
              <Picker.Item label="Adjustment" value="adjustment" />
              <Picker.Item label="New loan" value="loan" />
            </Picker>
          </View>
        </>
      )}

      {mode === 'head' && !fixedUserId && !isEdit && (
        <>
          <Text style={styles.label}>Member</Text>
          <View style={styles.pickerWrap}>
            <Picker<string>
              selectedValue={selectedMemberId}
              onValueChange={setSelectedMemberId}
              style={styles.picker}
              dropdownIconColor={theme.color.textSecondary}
            >
              {members.filter(m => m.role !== 'admin').map(m => (
                <Picker.Item key={m.user_id} label={m.name} value={m.user_id} />
              ))}
            </Picker>
          </View>
        </>
      )}

      {!isEdit && kind === 'chore' && (
        <>
          <Text style={styles.label}>Chore</Text>
          {nonSystemChores.length === 0 ? (
            <Text style={styles.hint}>No chores available. Create chores first.</Text>
          ) : (
            <View style={styles.pickerWrap}>
              <Picker<string>
                selectedValue={selectedChoreId}
                onValueChange={setSelectedChoreId}
                style={styles.picker}
                dropdownIconColor={theme.color.textSecondary}
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

      {showAmount && (
        <TextField
          label={kind === 'loan' ? 'Amount (loan)' : 'Amount'}
          value={amountStr}
          onChangeText={setAmountStr}
          keyboardType="decimal-pad"
          placeholder="e.g. 12.50"
          testID="entry-amount"
        />
      )}

      {!isEdit && kind === 'loan' && (
        <>
          <TextField
            label="Repay over (months)"
            value={installmentsStr}
            onChangeText={setInstallmentsStr}
            keyboardType="number-pad"
            placeholder="e.g. 6"
            testID="entry-loan-installments"
          />
          {loanEmiEstimate !== null && (
            <Text style={styles.hint}>
              ≈ {formatMoney(loanEmiEstimate, currency)} / month
            </Text>
          )}
          <Text style={styles.label}>First installment</Text>
          <View style={styles.startRow}>
            <Button
              title="Next month"
              variant={startCurrentMonth ? 'secondary' : 'primary'}
              size="sm"
              onPress={() => setStartCurrentMonth(false)}
              style={styles.startBtn}
              testID="loan-start-next-month"
            />
            <Button
              title="This month"
              variant={startCurrentMonth ? 'primary' : 'secondary'}
              size="sm"
              onPress={() => setStartCurrentMonth(true)}
              style={styles.startBtn}
              testID="loan-start-this-month"
            />
          </View>
        </>
      )}

      {showDirection && (
        <>
          <Text style={styles.label}>Direction</Text>
          <View style={styles.pickerWrap}>
            <Picker<Direction>
              selectedValue={direction}
              onValueChange={setDirection}
              style={styles.picker}
              dropdownIconColor={theme.color.textSecondary}
              testID="entry-direction-picker"
            >
              <Picker.Item label="Add to balance (credit)" value="credit" />
              <Picker.Item label="Subtract from balance (debit)" value="debit" />
            </Picker>
          </View>
        </>
      )}

      {!isEdit && kind !== 'loan' && (
        <DateField
          label="Date (optional)"
          value={dateStr}
          onChange={setDateStr}
          placeholder="Defaults to today"
          maximumDate={new Date()}
          testID="entry-date"
        />
      )}

      {showNote && (
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
  // Android renders the collapsed selected-value text in the Picker's own text
  // color, which defaults to an (often invisible) system color in a release
  // build — force it to the theme text color so the selection is readable.
  picker: {
    color: theme.color.text,
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
  startRow: {
    flexDirection: 'row',
    gap: 8,
    marginBottom: 4,
  },
  startBtn: {
    flex: 1,
  },
});
