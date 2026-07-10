import { useEffect, useRef, useState } from 'react';
import {
  FlatList,
  RefreshControl,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import type { Loan } from '../../../../../src/api';
import { useAuth } from '../../../../../src/auth-context';
import { useGroup } from '../../../../../src/hooks/useGroup';
import { useLoans, useRequestLoan, useRejectLoan, useCloseLoan } from '../../../../../src/hooks/useLoans';
import {
  Button,
  Sheet,
  TextField,
  EmptyState,
  LoadingSpinner,
  LoanApproveSheet,
  LoanCard,
  ScreenContainer,
  GroupSectionTabs,
  useToast,
} from '../../../../../src/components';
import { parseMoneyToMinorUnits, formatMoney, currencySymbol } from '../../../../../src/money';
import { confirmAsync } from '../../../../../src/confirm';
import { theme } from '../../../../../src/theme';

export default function LoansScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const router = useRouter();
  const { user } = useAuth();
  const toast = useToast();

  // All hooks above any early return
  const loansQuery = useLoans(id ?? '');
  const groupQuery = useGroup(id ?? '');
  const requestLoan = useRequestLoan(id ?? '');
  const rejectLoan = useRejectLoan(id ?? '');
  const closeLoan = useCloseLoan(id ?? '');

  const [requestSheetVisible, setRequestSheetVisible] = useState(false);
  const [approveSheetLoan, setApproveSheetLoan] = useState<Loan | null>(null);

  // Request loan form state
  const [principalStr, setPrincipalStr] = useState('');
  const [installmentsStr, setInstallmentsStr] = useState('');
  const [noteStr, setNoteStr] = useState('');
  const [principalError, setPrincipalError] = useState('');
  const [installmentsError, setInstallmentsError] = useState('');

  // Re-prefill request form on sheet open (closed→open edge)
  const prevRequestVisible = useRef(requestSheetVisible);
  useEffect(() => {
    if (requestSheetVisible && !prevRequestVisible.current) {
      setPrincipalStr('');
      setInstallmentsStr('');
      setNoteStr('');
      setPrincipalError('');
      setInstallmentsError('');
    }
    prevRequestVisible.current = requestSheetVisible;
  }, [requestSheetVisible]);

  const loans = loansQuery.data ?? [];
  const group = groupQuery.data;
  const isHead = group?.members.find(m => m.user_id === user?.id)?.role === 'admin';
  const currency = group?.currency ?? 'INR';

  const isLoading = loansQuery.isLoading || groupQuery.isLoading;
  const isRefetching = loansQuery.isRefetching || groupQuery.isRefetching;
  const loadError = loansQuery.error instanceof Error ? loansQuery.error.message
    : groupQuery.error instanceof Error ? groupQuery.error.message : '';

  const onRefresh = () => {
    loansQuery.refetch();
    groupQuery.refetch();
  };

  // Derived EMI estimate for request sheet
  const parsedPrincipal = parseMoneyToMinorUnits(principalStr);
  const parsedInstallments = (() => {
    const n = parseInt(installmentsStr.trim(), 10);
    return Number.isInteger(n) && n > 0 ? n : null;
  })();
  const estimateEmi =
    parsedPrincipal !== null && parsedInstallments !== null
      ? Math.ceil(parsedPrincipal / parsedInstallments)
      : null;

  async function handleRequestSubmit() {
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

    try {
      await requestLoan.mutateAsync({
        principal: { currency, value: principalValue },
        installments,
        note: noteStr.trim() || null,
      });
      toast.show({ tone: 'success', message: 'Loan requested' });
      setRequestSheetVisible(false);
    } catch (e) {
      toast.show({ tone: 'danger', message: e instanceof Error ? e.message : 'Failed to request loan' });
    }
  }

  async function handleReject(loan: Loan) {
    const ok = await confirmAsync({
      title: 'Reject Loan',
      message: `Reject this loan request for ${formatMoney(loan.principal.value, loan.principal.currency)}?`,
      confirmLabel: 'Reject',
      destructive: true,
    });
    if (!ok) return;
    try {
      await rejectLoan.mutateAsync(loan.id);
      toast.show({ tone: 'success', message: 'Loan rejected' });
    } catch (e) {
      toast.show({ tone: 'danger', message: e instanceof Error ? e.message : 'Failed to reject loan' });
    }
  }

  async function handleClose(loan: Loan) {
    const ok = await confirmAsync({
      title: 'Close Loan Early',
      message: `Pay off the outstanding ${formatMoney(loan.outstanding.value, loan.outstanding.currency)} now and close this loan?`,
      confirmLabel: 'Close Early',
      destructive: false,
    });
    if (!ok) return;
    try {
      await closeLoan.mutateAsync(loan.id);
      toast.show({ tone: 'success', message: 'Loan closed' });
    } catch (e) {
      toast.show({ tone: 'danger', message: e instanceof Error ? e.message : 'Failed to close loan' });
    }
  }

  function renderLoan({ item: loan }: { item: Loan }) {
    const memberName = isHead ? group?.members.find(m => m.user_id === loan.user_id)?.name : undefined;

    // Admin action buttons live in the card's `actions` slot; tapping the card
    // (anywhere else) opens the read-only loan detail with the repayment schedule.
    const actions = isHead && loan.status === 'requested' ? (
      <View style={styles.actions}>
        <Button
          title="Approve"
          variant="primary"
          size="sm"
          onPress={() => setApproveSheetLoan(loan)}
          style={styles.actionBtn}
          testID={`loan-approve-${loan.id}`}
        />
        <Button
          title="Reject"
          variant="danger"
          size="sm"
          onPress={() => handleReject(loan)}
          loading={rejectLoan.isPending}
          style={styles.actionBtn}
        />
      </View>
    ) : isHead && loan.status === 'active' && loan.outstanding.value > 0 ? (
      <View style={styles.actions}>
        <Button
          title="Close Early"
          variant="secondary"
          size="sm"
          onPress={() => handleClose(loan)}
          loading={closeLoan.isPending}
        />
      </View>
    ) : undefined;

    return (
      <LoanCard
        loan={loan}
        memberName={memberName}
        currency={currency}
        onPress={() => router.push(`/(app)/groups/${id}/loans/${loan.id}` as never)}
        actions={actions}
      />
    );
  }

  if (isLoading) {
    return <LoadingSpinner />;
  }

  const requestSheetFooter = (
    <View style={styles.footerRow}>
      <Button
        title="Cancel"
        variant="ghost"
        onPress={() => setRequestSheetVisible(false)}
        fullWidth
      />
      <Button
        title="Request"
        variant="primary"
        onPress={handleRequestSubmit}
        loading={requestLoan.isPending}
        disabled={requestLoan.isPending}
        fullWidth
        testID="loan-request-submit"
      />
    </View>
  );

  const emptyTitle = 'No loans yet';
  const emptySubtitle = isHead
    ? 'When a member requests a loan, it appears here for you to approve'
    : 'Tap "Request Loan" to borrow money repaid via pocket money';

  return (
    <ScreenContainer style={styles.container} testID="loans-root">
      <GroupSectionTabs groupId={id ?? ''} active="loans" />
      {loadError ? (
        <Text style={styles.loadError}>{loadError}</Text>
      ) : null}

      {!isHead && (
        <View style={styles.addButtonWrapper}>
          <Button
            title="Request Loan"
            icon="add"
            variant="primary"
            fullWidth
            onPress={() => setRequestSheetVisible(true)}
            testID="loans-request-button"
          />
        </View>
      )}

      {loans.length === 0 ? (
        <ScrollView
          refreshControl={<RefreshControl refreshing={isRefetching} onRefresh={onRefresh} />}
          contentContainerStyle={styles.emptyContainer}
        >
          <View testID="loans-empty">
            <EmptyState
              icon="card-outline"
              title={emptyTitle}
              subtitle={emptySubtitle}
            />
          </View>
        </ScrollView>
      ) : (
        <FlatList
          data={loans}
          renderItem={renderLoan}
          keyExtractor={(item) => item.id}
          refreshControl={<RefreshControl refreshing={isRefetching} onRefresh={onRefresh} />}
          contentContainerStyle={styles.list}
        />
      )}

      {/* Request Loan Sheet (member) */}
      <Sheet
        visible={requestSheetVisible}
        onClose={() => setRequestSheetVisible(false)}
        title="Request a Loan"
        footer={requestSheetFooter}
      >
        <TextField
          label={`Amount (${currencySymbol(currency)})`}
          keyboardType="decimal-pad"
          value={principalStr}
          onChangeText={(v) => { setPrincipalStr(v); setPrincipalError(''); }}
          error={principalError}
          placeholder="e.g. 5000"
          testID="loan-amount"
        />
        <TextField
          label="Repay over (months)"
          keyboardType="number-pad"
          value={installmentsStr}
          onChangeText={(v) => { setInstallmentsStr(v); setInstallmentsError(''); }}
          error={installmentsError}
          placeholder="e.g. 6"
          testID="loan-installments"
        />
        {estimateEmi !== null && (
          <Text style={styles.emiHint}>
            ≈ {formatMoney(estimateEmi, currency)} / month deducted from your pocket money
          </Text>
        )}
        <TextField
          label="Note (optional)"
          value={noteStr}
          onChangeText={setNoteStr}
          placeholder="What's this loan for?"
        />
      </Sheet>

      {/* Approve Loan Sheet (head) */}
      <LoanApproveSheet
        visible={approveSheetLoan !== null}
        onClose={() => setApproveSheetLoan(null)}
        groupId={id ?? ''}
        currency={currency}
        loan={approveSheetLoan}
      />
    </ScreenContainer>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: theme.color.background,
  },
  loadError: {
    color: theme.color.danger,
    padding: theme.spacing.lg,
    textAlign: 'center',
    fontSize: theme.fontSize.sm,
  },
  addButtonWrapper: {
    margin: theme.spacing.lg,
  },
  list: {
    padding: theme.spacing.lg,
  },
  emptyContainer: {
    flexGrow: 1,
  },
  actions: {
    flexDirection: 'row',
    gap: theme.spacing.sm,
    marginTop: theme.spacing.xs,
  },
  actionBtn: {
    flex: 1,
  },
  emiHint: {
    fontSize: theme.fontSize.sm,
    color: theme.color.textSecondary,
    marginTop: theme.spacing.sm,
    marginBottom: theme.spacing.xs,
  },
  footerRow: {
    flexDirection: 'row',
    gap: theme.spacing.sm,
  },
});
