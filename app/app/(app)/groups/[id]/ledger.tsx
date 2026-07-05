import { useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import { Redirect, Stack, useLocalSearchParams } from 'expo-router';
import { useAuth } from '../../../../src/auth-context';
import { useGroup } from '../../../../src/hooks/useGroup';
import { useChores } from '../../../../src/hooks/useChores';
import { useLedger, useBalance, useApproveLedger, useRejectLedger } from '../../../../src/hooks/useLedger';
import { confirmAsync } from '../../../../src/confirm';
import {
  AmountText,
  Button,
  LedgerList,
  AddEntrySheet,
  LoadingSpinner,
  ErrorMessage,
  useToast,
} from '../../../../src/components';

export default function MemberLedgerScreen() {
  const { id, userId, name } = useLocalSearchParams<{ id: string; userId: string; name: string }>();
  const { user } = useAuth();
  const { show: showToast } = useToast();

  const [sheetVisible, setSheetVisible] = useState(false);
  const [processingId, setProcessingId] = useState<string | null>(null);

  const groupQuery = useGroup(id ?? '');
  const group = groupQuery.data;
  const members = group?.members ?? [];
  const isHead = members.find(m => m.user_id === user?.id)?.role === 'head';

  const ledgerQuery = useLedger(id ?? '', { user_id: userId });
  const balanceQuery = useBalance(id ?? '');
  const choresQuery = useChores(id ?? '');
  const approveMutation = useApproveLedger(id ?? '');
  const rejectMutation = useRejectLedger(id ?? '');

  const memberBalance = balanceQuery.data?.find(b => b.user_id === userId);
  const entries = ledgerQuery.data ?? [];

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
      </View>
    </>
  );
}

const styles = StyleSheet.create({
  screen: {
    flex: 1,
    backgroundColor: '#F5F5F5',
  },
  centered: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
  },
  summaryCard: {
    backgroundColor: '#FFFFFF',
    padding: 24,
    alignItems: 'center',
    borderBottomWidth: 1,
    borderBottomColor: '#E5E7EB',
  },
  summaryLabel: {
    fontSize: 15,
    color: '#6B7280',
    marginBottom: 8,
  },
  summaryHint: {
    fontSize: 13,
    color: '#6B7280',
    marginTop: 4,
  },
  addButtonRow: {
    padding: 16,
  },
});
