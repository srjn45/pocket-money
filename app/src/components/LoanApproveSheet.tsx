import { useCallback, useEffect, useRef, useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import type { Loan, CurrencyCode } from '../api';
import { formatMinor, parseMoneyToMinorUnits, formatMoney, currencySymbol } from '../money';
import { useApproveLoan } from '../hooks/useLoans';
import { theme } from '../theme';
import { Sheet } from './Sheet';
import { Button } from './Button';
import { TextField } from './TextField';
import { useToast } from './Toast';

interface LoanApproveSheetProps {
  visible: boolean;
  onClose: () => void;
  groupId: string;
  /** Group currency — stamped onto the outgoing principal. */
  currency: CurrencyCode;
  loan: Loan | null;
}

export function LoanApproveSheet({ visible, onClose, groupId, currency, loan }: LoanApproveSheetProps) {
  const { show: showToast } = useToast();
  const approveLoan = useApproveLoan(groupId);

  const initialPrincipal = useCallback(
    () => (loan ? formatMinor(loan.principal.value) : ''),
    [loan],
  );
  const initialInstallments = useCallback(
    () => (loan ? String(loan.installments) : ''),
    [loan],
  );

  const [principalStr, setPrincipalStr] = useState(initialPrincipal);
  const [installmentsStr, setInstallmentsStr] = useState(initialInstallments);
  const [principalError, setPrincipalError] = useState('');
  const [installmentsError, setInstallmentsError] = useState('');
  // First-installment month (QA batch 1, Item 5). false = next month (default).
  const [startCurrentMonth, setStartCurrentMonth] = useState(false);

  // Re-prefill when the sheet opens (closed→open edge), matching the AllowanceSheet pattern.
  const prevVisible = useRef(visible);
  useEffect(() => {
    if (visible && !prevVisible.current) {
      setPrincipalStr(initialPrincipal());
      setInstallmentsStr(initialInstallments());
      setPrincipalError('');
      setInstallmentsError('');
      setStartCurrentMonth(false);
    }
    prevVisible.current = visible;
  }, [visible, initialPrincipal, initialInstallments]);

  function handleClose() {
    onClose();
  }

  const parsedPrincipal = parseMoneyToMinorUnits(principalStr);
  const parsedInstallments = (() => {
    const n = parseInt(installmentsStr.trim(), 10);
    return Number.isInteger(n) && n > 0 ? n : null;
  })();

  const estimateEmi =
    parsedPrincipal !== null && parsedInstallments !== null
      ? Math.ceil(parsedPrincipal / parsedInstallments)
      : null;

  async function handleApprove() {
    setPrincipalError('');
    setInstallmentsError('');

    const principalValue = parseMoneyToMinorUnits(principalStr);
    if (principalValue === null) {
      setPrincipalError('Enter a valid amount (e.g. 5000).');
      return;
    }
    const installments = parseInt(installmentsStr.trim(), 10);
    if (!Number.isInteger(installments) || installments <= 0) {
      setInstallmentsError('Enter a whole number of months (e.g. 6).');
      return;
    }

    if (!loan) return;
    try {
      await approveLoan.mutateAsync({ loanId: loan.id, principal: { currency, value: principalValue }, installments, start_current_month: startCurrentMonth });
      showToast({ tone: 'success', message: 'Loan approved' });
      handleClose();
    } catch (e) {
      showToast({ tone: 'danger', message: e instanceof Error ? e.message : 'Failed to approve' });
    }
  }

  const isPending = approveLoan.isPending;

  return (
    <Sheet
      visible={visible}
      onClose={handleClose}
      title="Approve Loan"
      footer={
        <View style={styles.footerRow}>
          <Button title="Cancel" variant="ghost" onPress={handleClose} fullWidth />
          <Button
            title="Approve"
            variant="primary"
            onPress={handleApprove}
            loading={isPending}
            disabled={isPending}
            fullWidth
            testID="loan-approve-submit"
          />
        </View>
      }
    >
      <TextField
        label={`Principal (${currencySymbol(currency)})`}
        keyboardType="decimal-pad"
        value={principalStr}
        onChangeText={(v) => { setPrincipalStr(v); setPrincipalError(''); }}
        error={principalError}
        placeholder="e.g. 5000"
      />
      <TextField
        label="Installments (months)"
        keyboardType="number-pad"
        value={installmentsStr}
        onChangeText={(v) => { setInstallmentsStr(v); setInstallmentsError(''); }}
        error={installmentsError}
        placeholder="e.g. 6"
      />
      {estimateEmi !== null && (
        <Text style={styles.emiHint}>
          ≈ {formatMoney(estimateEmi, currency)} / month
        </Text>
      )}
      <Text style={styles.startLabel}>First installment</Text>
      <View style={styles.startRow}>
        <Button
          title="Next month"
          variant={startCurrentMonth ? 'secondary' : 'primary'}
          size="sm"
          onPress={() => setStartCurrentMonth(false)}
          style={styles.startBtn}
          testID="loan-approve-start-next-month"
        />
        <Button
          title="This month"
          variant={startCurrentMonth ? 'primary' : 'secondary'}
          size="sm"
          onPress={() => setStartCurrentMonth(true)}
          style={styles.startBtn}
          testID="loan-approve-start-this-month"
        />
      </View>
      {loan?.note ? (
        <Text style={styles.noteText}>Note: {loan.note}</Text>
      ) : null}
    </Sheet>
  );
}

const styles = StyleSheet.create({
  footerRow: {
    flexDirection: 'row',
    gap: theme.spacing.sm,
  },
  emiHint: {
    fontSize: theme.fontSize.sm,
    color: theme.color.textSecondary,
    marginTop: theme.spacing.sm,
  },
  startLabel: {
    fontSize: theme.fontSize.sm,
    fontWeight: theme.fontWeight.semibold,
    color: theme.color.textSecondary,
    marginTop: theme.spacing.md,
    marginBottom: theme.spacing.xs,
  },
  startRow: {
    flexDirection: 'row',
    gap: theme.spacing.sm,
  },
  startBtn: {
    flex: 1,
  },
  noteText: {
    fontSize: theme.fontSize.sm,
    color: theme.color.textSecondary,
    marginTop: theme.spacing.sm,
    fontStyle: 'italic',
  },
});
