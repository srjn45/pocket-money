import { useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import { Redirect, Stack, useLocalSearchParams, useRouter } from 'expo-router';
import type { Loan } from '../../../../../src/api';
import { useAuth } from '../../../../../src/auth-context';
import { useGroup } from '../../../../../src/hooks/useGroup';
import { useChores } from '../../../../../src/hooks/useChores';
import { useLedger, useBalance, useApproveLedger, useRejectLedger } from '../../../../../src/hooks/useLedger';
import { useAllowances } from '../../../../../src/hooks/useAllowances';
import { useLoans } from '../../../../../src/hooks/useLoans';
import { confirmAsync } from '../../../../../src/confirm';
import { currentPeriod, currentAllowanceFor, upcomingAllowanceFor } from '../../../../../src/allowance-format';
import { formatMinor } from '../../../../../src/money';
import { theme } from '../../../../../src/theme';
import {
  AmountText,
  Button,
  Card,
  LedgerList,
  AddEntrySheet,
  AllowanceSummary,
  AllowanceSheet,
  StatusBadge,
  LoadingSpinner,
  ErrorMessage,
  useToast,
} from '../../../../../src/components';

function loanStatusTone(status: Loan['status']): 'neutral' | 'success' | 'warning' | 'danger' | 'info' {
  switch (status) {
    case 'requested': return 'warning';
    case 'active':    return 'success';
    case 'rejected':  return 'danger';
    case 'closed':    return 'neutral';
  }
}

export default function MemberDetailScreen() {
  const { id, userId, name } = useLocalSearchParams<{ id: string; userId: string; name?: string }>();
  const { user } = useAuth();
  const router = useRouter();
  const { show: showToast } = useToast();

  // All hooks above any early return (§9.3)
  const [sheetVisible, setSheetVisible] = useState(false);
  const [allowanceSheetVisible, setAllowanceSheetVisible] = useState(false);
  const [processingId, setProcessingId] = useState<string | null>(null);

  const groupQuery = useGroup(id ?? '');
  const group = groupQuery.data;
  const members = group?.members ?? [];
  const isHead = members.find(m => m.user_id === user?.id)?.role === 'head';

  const ledgerQuery = useLedger(id ?? '', { user_id: userId });
  const balanceQuery = useBalance(id ?? '');
  const choresQuery = useChores(id ?? '');
  const allowancesQuery = useAllowances(id ?? '');
  const loansQuery = useLoans(id ?? '');

  const approveMutation = useApproveLedger(id ?? '');
  const rejectMutation = useRejectLedger(id ?? '');

  const period = currentPeriod();
  const memberAllowances = (allowancesQuery.data ?? []).filter(a => a.user_id === userId);
  const currentAllow = currentAllowanceFor(memberAllowances, period);
  const upcomingAllow = upcomingAllowanceFor(memberAllowances, period);

  const memberBalance = balanceQuery.data?.find(b => b.user_id === userId);
  const entries = ledgerQuery.data ?? [];

  // Loans filtered to this member (read-only; management is on Loans tab)
  const memberLoans = (loansQuery.data ?? []).filter(l => l.user_id === userId);
  const activeLoans = memberLoans.filter(l => l.status === 'active');
  const requestedLoans = memberLoans.filter(l => l.status === 'requested');
  const hasLoans = activeLoans.length + requestedLoans.length > 0;

  async function handleApprove(entryId: string) {
    setProcessingId(entryId);
    try {
      await approveMutation.mutateAsync(entryId);
      showToast({ message: 'Approved', tone: 'success' });
    } catch (e) {
      showToast({ message: e instanceof Error ? e.message : 'Failed to approve', tone: 'danger' });
    } finally {
      setProcessingId(null);
    }
  }

  async function handleReject(entryId: string) {
    const confirmed = await confirmAsync({
      title: 'Reject entry',
      message: "This entry won't count toward the balance.",
      confirmLabel: 'Reject',
      destructive: true,
    });
    if (!confirmed) return;

    setProcessingId(entryId);
    try {
      await rejectMutation.mutateAsync(entryId);
      showToast({ message: 'Rejected', tone: 'info' });
    } catch (e) {
      showToast({ message: e instanceof Error ? e.message : 'Failed to reject', tone: 'danger' });
    } finally {
      setProcessingId(null);
    }
  }

  // Early returns after all hooks
  if (groupQuery.isLoading || balanceQuery.isLoading) {
    return <View style={styles.centered}><LoadingSpinner /></View>;
  }

  if (!isHead) {
    return <Redirect href={`/(app)/groups/${id}/` as never} />;
  }

  if (groupQuery.error) {
    return <ErrorMessage message={groupQuery.error instanceof Error ? groupQuery.error.message : 'Failed to load'} />;
  }

  const bal = memberBalance?.balance ?? 0;

  function renderLoanRow(loan: Loan) {
    const isActive = loan.status === 'active';
    return (
      <Card key={loan.id} style={styles.loanRow}>
        <View style={styles.loanHeader}>
          <StatusBadge
            label={isActive ? 'Active' : 'Pending'}
            tone={loanStatusTone(loan.status)}
          />
          <AmountText minorUnits={loan.principal} variant="neutral" size="sm" />
        </View>
        {loan.note ? <Text style={styles.loanNote}>{loan.note}</Text> : null}
        <View style={styles.loanDetails}>
          {isActive && (
            <>
              <View style={styles.detailRow}>
                <Text style={styles.detailLabel}>Outstanding</Text>
                <AmountText minorUnits={loan.outstanding} variant="debit" size="sm" />
              </View>
              <View style={styles.detailRow}>
                <Text style={styles.detailLabel}>EMI</Text>
                <Text style={styles.detailValue}>≈ ₹{formatMinor(loan.emi_amount)} / month</Text>
              </View>
              <View style={styles.detailRow}>
                <Text style={styles.detailLabel}>Progress</Text>
                <Text style={styles.detailValue}>{loan.installments_posted}/{loan.installments} paid</Text>
              </View>
            </>
          )}
          {!isActive && (
            <>
              <View style={styles.detailRow}>
                <Text style={styles.detailLabel}>Amount</Text>
                <Text style={styles.detailValue}>₹{formatMinor(loan.principal)} over {loan.installments} months</Text>
              </View>
              <View style={styles.detailRow}>
                <Text style={styles.detailLabel}>Est. EMI</Text>
                <Text style={styles.detailValue}>≈ ₹{formatMinor(loan.emi_amount)} / month</Text>
              </View>
            </>
          )}
        </View>
      </Card>
    );
  }

  return (
    <>
      <Stack.Screen options={{ title: name ? `${name}'s ledger` : 'Ledger' }} />
      <View style={styles.screen}>
        {/* Balance header */}
        <View style={styles.summaryCard}>
          <Text style={styles.summaryLabel}>{name ? `${name}'s balance` : 'Balance'}</Text>
          <AmountText
            minorUnits={bal}
            variant={bal < 0 ? 'debit' : 'credit'}
            size="xl"
          />
          <Text style={styles.summaryHint}>
            {bal < 0 ? 'owes you' : 'owed'}
          </Text>
          {!allowancesQuery.isError && (
            <AllowanceSummary
              current={currentAllow}
              upcoming={upcomingAllow}
              onEdit={() => setAllowanceSheetVisible(true)}
            />
          )}
        </View>

        <View style={styles.addButtonRow}>
          <Button
            title="Add entry"
            variant="primary"
            icon="add"
            onPress={() => setSheetVisible(true)}
            fullWidth
          />
        </View>

        {/* Loans section — read-only, hidden when none or on error */}
        {hasLoans && !loansQuery.isError && (
          <View style={styles.loansSection}>
            <View style={styles.loansSectionHeader}>
              <Text style={styles.sectionTitle}>Loans</Text>
              <Button
                title="Manage in Loans tab"
                variant="ghost"
                size="sm"
                onPress={() => router.push(`/(app)/groups/${id}/loans` as never)}
              />
            </View>
            {[...activeLoans, ...requestedLoans].map(renderLoanRow)}
          </View>
        )}

        <LedgerList
          entries={entries}
          chores={choresQuery.data ?? []}
          members={members}
          isHead={true}
          groupId={id ?? ''}
          onApprove={handleApprove}
          onReject={handleReject}
          processingId={processingId}
          refreshing={ledgerQuery.isFetching}
          onRefresh={() => {
            ledgerQuery.refetch();
            balanceQuery.refetch();
            allowancesQuery.refetch();
            loansQuery.refetch();
          }}
          emptyTitle="No entries yet"
          emptySubtitle="Add a chore, settlement, or adjustment"
        />

        <AddEntrySheet
          visible={sheetVisible}
          onClose={() => setSheetVisible(false)}
          groupId={id ?? ''}
          chores={choresQuery.data ?? []}
          mode="head"
          fixedUserId={userId}
        />

        <AllowanceSheet
          visible={allowanceSheetVisible}
          onClose={() => setAllowanceSheetVisible(false)}
          groupId={id ?? ''}
          userId={userId ?? ''}
          memberName={name}
          current={currentAllow}
        />
      </View>
    </>
  );
}

const styles = StyleSheet.create({
  screen: {
    flex: 1,
    backgroundColor: theme.color.background,
  },
  centered: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
  },
  summaryCard: {
    backgroundColor: theme.color.surface,
    padding: theme.spacing.xl,
    alignItems: 'center',
    borderBottomWidth: 1,
    borderBottomColor: theme.color.border,
  },
  summaryLabel: {
    fontSize: theme.fontSize.sm,
    color: theme.color.textSecondary,
    marginBottom: theme.spacing.sm,
  },
  summaryHint: {
    fontSize: theme.fontSize.xs,
    color: theme.color.textSecondary,
    marginTop: theme.spacing.xs,
  },
  addButtonRow: {
    padding: theme.spacing.lg,
  },
  loansSection: {
    paddingHorizontal: theme.spacing.lg,
    paddingBottom: theme.spacing.md,
  },
  loansSectionHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: theme.spacing.sm,
  },
  sectionTitle: {
    fontSize: theme.fontSize.sm,
    fontWeight: theme.fontWeight.semibold,
    color: theme.color.textSecondary,
  },
  loanRow: {
    marginBottom: theme.spacing.sm,
  },
  loanHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: theme.spacing.xs,
  },
  loanNote: {
    fontSize: theme.fontSize.xs,
    color: theme.color.textSecondary,
    fontStyle: 'italic',
    marginBottom: theme.spacing.xs,
  },
  loanDetails: {
    gap: theme.spacing.xs,
  },
  detailRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  detailLabel: {
    fontSize: theme.fontSize.xs,
    color: theme.color.textSecondary,
  },
  detailValue: {
    fontSize: theme.fontSize.xs,
    color: theme.color.text,
    fontWeight: theme.fontWeight.medium,
  },
});
