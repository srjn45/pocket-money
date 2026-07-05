import { useCallback, useEffect, useRef, useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import type { Loan } from '../api';
import { formatMinor, parseMoneyToMinorUnits } from '../money';
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
  loan: Loan | null;
}

export function LoanApproveSheet({ visible, onClose, groupId, loan }: LoanApproveSheetProps) {
  const { show: showToast } = useToast();
  const approveLoan = useApproveLoan(groupId);

  const initialPrincipal = useCallback(
    () => (loan ? formatMinor(loan.principal) : ''),
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

  // Re-prefill when the sheet opens (closed→open edge), matching the AllowanceSheet pattern.
  const prevVisible = useRef(visible);
  useEffect(() => {
    if (visible && !prevVisible.current) {
      setPrincipalStr(initialPrincipal());
      setInstallmentsStr(initialInstallments());
      setPrincipalError('');
      setInstallmentsError('');
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

    const principal = parseMoneyToMinorUnits(principalStr);
    if (principal === null) {
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
      await approveLoan.mutateAsync({ loanId: loan.id, principal, installments });
      showToast({ tone: 'success', message: 'Loan approved' });
      handleClose();
    } catch (e) {
      showToast({ tone: 'danger', message: e instanceof Error ? e.message : 'Failed to approve' });
    }
  }

  const isPending = approveLoan.isPending;
  const borrowerName = loan?.note ? undefined : undefined; // note is on the loan, not a user name

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
          />
        </View>
      }
    >
      <TextField
        label="Principal (₹)"
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
          ≈ ₹{formatMinor(estimateEmi)} / month
        </Text>
      )}
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
  noteText: {
    fontSize: theme.fontSize.sm,
    color: theme.color.textSecondary,
    marginTop: theme.spacing.sm,
    fontStyle: 'italic',
  },
});
