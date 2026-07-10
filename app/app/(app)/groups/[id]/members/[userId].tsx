import { useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import { Redirect, Stack, useLocalSearchParams, useRouter } from 'expo-router';
import type { LedgerEntry } from '../../../../../src/api';
import { useAuth } from '../../../../../src/auth-context';
import { useGroup } from '../../../../../src/hooks/useGroup';
import { useChores } from '../../../../../src/hooks/useChores';
import {
  useLedger,
  useBalance,
  useApproveLedger,
  useRejectLedger,
  useDeleteLedgerEntry,
} from '../../../../../src/hooks/useLedger';
import { useAllowances } from '../../../../../src/hooks/useAllowances';
import { useLoans } from '../../../../../src/hooks/useLoans';
import { confirmAsync } from '../../../../../src/confirm';
import { currentPeriod, currentAllowanceFor, upcomingAllowanceFor } from '../../../../../src/allowance-format';
import { currencySymbol } from '../../../../../src/money';
import { useRemoveMember } from '../../../../../src/hooks/useHygiene';
import { theme } from '../../../../../src/theme';
import {
  AmountText,
  Button,
  LedgerList,
  LoanCard,
  PassbookBaseHistory,
  AddEntrySheet,
  AllowanceSummary,
  AllowanceSheet,
  StatusBadge,
  LoadingSpinner,
  ErrorMessage,
  ScreenContainer,
  useToast,
} from '../../../../../src/components';

export default function MemberDetailScreen() {
  const { id, userId, name } = useLocalSearchParams<{ id: string; userId: string; name?: string }>();
  const { user } = useAuth();
  const router = useRouter();
  const { show: showToast } = useToast();

  // All hooks above any early return (§9.3)
  const [sheetVisible, setSheetVisible] = useState(false);
  const [allowanceSheetVisible, setAllowanceSheetVisible] = useState(false);
  const [processingId, setProcessingId] = useState<string | null>(null);
  // Corrections (D3): entry being edited + session-local "Edited" badge set (§4.3).
  const [editEntry, setEditEntry] = useState<LedgerEntry | null>(null);
  const [editedIds, setEditedIds] = useState<Set<string>>(() => new Set());

  const groupQuery = useGroup(id ?? '');
  const group = groupQuery.data;
  const members = group?.members ?? [];
  const isHead = members.find(m => m.user_id === user?.id)?.role === 'admin';
  const currency = group?.currency ?? 'INR';

  const ledgerQuery = useLedger(id ?? '', { user_id: userId });
  const balanceQuery = useBalance(id ?? '');
  const choresQuery = useChores(id ?? '');
  const allowancesQuery = useAllowances(id ?? '');
  const loansQuery = useLoans(id ?? '');

  const approveMutation = useApproveLedger(id ?? '');
  const rejectMutation = useRejectLedger(id ?? '');
  const removeMutation = useRemoveMember(id ?? '');
  const deleteEntryMutation = useDeleteLedgerEntry(id ?? '');

  const period = currentPeriod();
  const memberAllowances = (allowancesQuery.data ?? []).filter(a => a.user_id === userId);
  const currentAllow = currentAllowanceFor(memberAllowances, period);
  const upcomingAllow = upcomingAllowanceFor(memberAllowances, period);

  const memberBalance = balanceQuery.data?.find(b => b.user_id === userId);
  const entries = ledgerQuery.data ?? [];
  const viewedMember = members.find(m => m.user_id === userId);
  const canRemove = isHead && viewedMember?.role !== 'admin';

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

  async function handleDeleteEntry(entry: LedgerEntry) {
    const confirmed = await confirmAsync({
      title: 'Delete entry',
      message: "This entry will be removed and won't count toward the balance.",
      confirmLabel: 'Delete',
      destructive: true,
    });
    if (!confirmed) return;
    try {
      await deleteEntryMutation.mutateAsync(entry.id);
      showToast({ message: 'Entry deleted', tone: 'success' });
    } catch (e) {
      showToast({ message: e instanceof Error ? e.message : 'Failed to delete', tone: 'danger' });
    }
  }

  function handleEdited(entryId: string) {
    setEditedIds(prev => {
      const next = new Set(prev);
      next.add(entryId);
      return next;
    });
  }

  async function handleRemoveMember() {
    const memberName = name || 'this member';
    const confirmed = await confirmAsync({
      title: 'Remove member',
      message: `Remove ${memberName} from the group? Their ledger history is kept. This only works if their balance is ${currencySymbol(currency)}0 and they have no active or pending loans.`,
      confirmLabel: 'Remove',
      destructive: true,
    });
    if (!confirmed) return;
    try {
      await removeMutation.mutateAsync(userId ?? '');
      showToast({ tone: 'success', message: `${memberName} removed` });
      router.back();
    } catch (e) {
      showToast({ tone: 'danger', message: e instanceof Error ? e.message : 'Failed to remove member' });
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

  const balValue = memberBalance?.balance.value ?? 0;
  const balCurrency = memberBalance?.balance.currency ?? currency;

  return (
    <>
      <Stack.Screen options={{ title: name ? `${name}'s ledger` : 'Ledger' }} />
      <ScreenContainer style={styles.screen} testID="member-detail-root">
        {/* Balance header */}
        <View style={styles.summaryCard}>
          <Text style={styles.summaryLabel}>{name ? `${name}'s balance` : 'Balance'}</Text>
          {viewedMember?.status === 'shadow' && (
            <View testID="member-detail-shadow-badge" style={styles.shadowBadge}>
              <StatusBadge label="Not registered yet" tone="neutral" />
            </View>
          )}
          <AmountText
            minorUnits={balValue}
            currency={balCurrency}
            variant={balValue < 0 ? 'debit' : 'credit'}
            size="xl"
          />
          <Text style={styles.summaryHint}>
            {balValue < 0 ? 'owes you' : 'owed'}
          </Text>
          {!allowancesQuery.isError && (
            <AllowanceSummary
              current={currentAllow}
              upcoming={upcomingAllow}
              currency={currency}
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
            testID="member-add-entry-button"
          />
        </View>

        {/* Base-amount (pocket money) history — hidden when none or on error */}
        {!allowancesQuery.isError && (
          <PassbookBaseHistory allowances={memberAllowances} currency={currency} />
        )}

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
            {[...activeLoans, ...requestedLoans].map(loan => (
              <LoanCard
                key={loan.id}
                loan={loan}
                currency={currency}
                onPress={() => router.push(`/(app)/groups/${id}/loans/${loan.id}` as never)}
              />
            ))}
          </View>
        )}

        <LedgerList
          entries={entries}
          chores={choresQuery.data ?? []}
          members={members}
          loans={memberLoans}
          isHead={true}
          groupId={id ?? ''}
          currency={currency}
          onApprove={handleApprove}
          onReject={handleReject}
          processingId={processingId}
          canEdit={isHead}
          onEditEntry={setEditEntry}
          onDeleteEntry={handleDeleteEntry}
          editedIds={editedIds}
          refreshing={ledgerQuery.isFetching}
          onRefresh={() => {
            ledgerQuery.refetch();
            balanceQuery.refetch();
            allowancesQuery.refetch();
            loansQuery.refetch();
          }}
          emptyTitle="No entries yet"
          emptySubtitle="Add a chore, payment, or adjustment"
        />

        <AddEntrySheet
          visible={sheetVisible}
          onClose={() => setSheetVisible(false)}
          groupId={id ?? ''}
          currency={currency}
          chores={choresQuery.data ?? []}
          mode="head"
          fixedUserId={userId}
        />

        {/* Corrections edit sheet (D3) — type immutable, PUT /ledger/{id} */}
        <AddEntrySheet
          visible={editEntry !== null}
          onClose={() => setEditEntry(null)}
          groupId={id ?? ''}
          currency={currency}
          chores={choresQuery.data ?? []}
          mode="head"
          fixedUserId={userId}
          editEntry={editEntry ?? undefined}
          onEdited={handleEdited}
        />

        <AllowanceSheet
          visible={allowanceSheetVisible}
          onClose={() => setAllowanceSheetVisible(false)}
          groupId={id ?? ''}
          currency={currency}
          userId={userId ?? ''}
          memberName={name}
          current={currentAllow}
        />

        {canRemove && (
          <View style={styles.removeButtonRow}>
            <Button
              title="Remove member"
              variant="danger"
              loading={removeMutation.isPending}
              onPress={handleRemoveMember}
              fullWidth
              testID="member-remove-button"
            />
          </View>
        )}
      </ScreenContainer>
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
  shadowBadge: {
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
  removeButtonRow: {
    padding: theme.spacing.lg,
  },
});
