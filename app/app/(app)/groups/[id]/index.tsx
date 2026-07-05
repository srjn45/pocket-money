import { useState } from 'react';
import { FlatList, Platform, ScrollView, Share, StyleSheet, Text, View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import * as Clipboard from 'expo-clipboard';
import { useAuth } from '../../../../src/auth-context';
import { useGroup } from '../../../../src/hooks/useGroup';
import { useChores } from '../../../../src/hooks/useChores';
import { useLedger, useBalance } from '../../../../src/hooks/useLedger';
import { useAllowances } from '../../../../src/hooks/useAllowances';
import { useLoans } from '../../../../src/hooks/useLoans';
import { currentPeriod, currentAllowanceFor, upcomingAllowanceFor } from '../../../../src/allowance-format';
import { groupsApi } from '../../../../src/api';
import type { Balance } from '../../../../src/api';
import { confirmAsync } from '../../../../src/confirm';
import { useLeaveGroup } from '../../../../src/hooks/useHygiene';
import { theme } from '../../../../src/theme';
import {
  Button,
  Sheet,
  TextField,
  AmountText,
  MemberCard,
  LedgerList,
  AddEntrySheet,
  AllowanceSummary,
  EmptyState,
  ErrorMessage,
  LoadingSpinner,
  useToast,
} from '../../../../src/components';

export default function GroupOverviewScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { user } = useAuth();
  const router = useRouter();
  const { show: showToast } = useToast();

  const [sheetVisible, setSheetVisible] = useState(false);
  const [inviteLoading, setInviteLoading] = useState(false);
  const [fallbackUrl, setFallbackUrl] = useState<string | null>(null);

  const groupQuery = useGroup(id ?? '');
  const balanceQuery = useBalance(id ?? '');
  const choresQuery = useChores(id ?? '');
  const allowancesQuery = useAllowances(id ?? '');
  const loansQuery = useLoans(id ?? '');

  const group = groupQuery.data;
  const members = group?.members ?? [];
  const isHead = members.find(m => m.user_id === user?.id)?.role === 'head';

  const pendingLedgerQuery = useLedger(id ?? '', { status: 'pending_approval' });
  const myLedgerQuery = useLedger(id ?? '', isHead ? undefined : {});

  const leaveMutation = useLeaveGroup(id ?? '');

  const memberPeriod = currentPeriod();
  const memberCurrentAllow = currentAllowanceFor(allowancesQuery.data ?? [], memberPeriod);
  const memberUpcomingAllow = upcomingAllowanceFor(allowancesQuery.data ?? [], memberPeriod);

  const isLoading = groupQuery.isLoading || balanceQuery.isLoading;
  const error = groupQuery.error || balanceQuery.error;

  const nonHeadMembers = members.filter(m => m.role !== 'head');
  const myBalance = balanceQuery.data?.find(b => b.user_id === user?.id);

  function countPendingFor(userId: string): number {
    return (pendingLedgerQuery.data ?? []).filter(e => e.user_id === userId).length;
  }

  async function handleLeaveGroup() {
    const confirmed = await confirmAsync({
      title: 'Leave group',
      message: `Leave ${group?.name ?? 'this group'}? You can rejoin with a new invite. This only works if your balance is ₹0 and you have no active or pending loans.`,
      confirmLabel: 'Leave',
      destructive: true,
    });
    if (!confirmed) return;
    try {
      await leaveMutation.mutateAsync(user?.id ?? '');
      showToast({ tone: 'success', message: 'You left the group' });
      router.replace('/(app)' as never);
    } catch (e) {
      showToast({ tone: 'danger', message: e instanceof Error ? e.message : 'Failed to leave group' });
    }
  }

  async function handleInvite() {
    if (!id) return;
    setInviteLoading(true);
    try {
      const invite = await groupsApi.createInvite(id);
      if (Platform.OS === 'web') {
        try {
          // On non-secure http origins (LAN) navigator.clipboard is unavailable;
          // expo-clipboard falls back to execCommand and RESOLVES false (it does
          // not reject), so we must honor the boolean, not just catch a throw.
          const copied = await Clipboard.setStringAsync(invite.invite_url);
          if (copied) {
            showToast({ message: 'Invite link copied to clipboard', tone: 'success' });
          } else {
            setFallbackUrl(invite.invite_url);
            showToast({ message: 'Copy failed — select and copy the link below', tone: 'danger' });
          }
        } catch {
          setFallbackUrl(invite.invite_url);
          showToast({ message: 'Copy failed — select and copy the link below', tone: 'danger' });
        }
      } else {
        await Share.share({
          message: `Join my group "${group?.name}" on Pocket Money!\n\n${invite.invite_url}`,
          url: invite.invite_url,
          title: 'Join My Group',
        });
        // dismissedAction is not an error — no Toast on cancel.
      }
    } catch (e) {
      showToast({ message: e instanceof Error ? e.message : 'Failed to create invite', tone: 'danger' });
    } finally {
      setInviteLoading(false);
    }
  }

  if (isLoading) {
    return <View style={styles.centered}><LoadingSpinner /></View>;
  }

  if (error) {
    return <ErrorMessage message={error instanceof Error ? error.message : 'Failed to load group'} />;
  }

  // ─── HEAD VIEW ─────────────────────────────────────────────────────────────
  if (isHead) {
    const balances = balanceQuery.data ?? [];
    const memberBalanceMap = new Map<string, Balance>(balances.map(b => [b.user_id, b]));

    return (
      <View style={styles.screen}>
        <View style={styles.header}>
          <Text style={styles.memberCount}>
            {nonHeadMembers.length} {nonHeadMembers.length === 1 ? 'member' : 'members'}
          </Text>
          <Button
            title="Invite"
            variant="ghost"
            icon="person-add"
            loading={inviteLoading}
            onPress={handleInvite}
            size="sm"
          />
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

        <Text style={styles.sectionTitle}>Members</Text>

        <FlatList
          data={nonHeadMembers}
          keyExtractor={m => m.user_id}
          renderItem={({ item: member }) => {
            const bal = memberBalanceMap.get(member.user_id) ?? {
              user_id: member.user_id,
              name: member.name,
              balance: 0,
            };
            return (
              <MemberCard
                balance={bal}
                member={member}
                pendingCount={countPendingFor(member.user_id)}
                onPress={() =>
                  router.push({
                    pathname: `/(app)/groups/${id}/members/${member.user_id}`,
                    params: { name: member.name },
                  })
                }
              />
            );
          }}
          ListEmptyComponent={
            <EmptyState
              icon="people-outline"
              title="No members yet"
              subtitle="Tap Invite to add your family"
            />
          }
          refreshing={groupQuery.isFetching || balanceQuery.isFetching}
          onRefresh={() => {
            groupQuery.refetch();
            balanceQuery.refetch();
            pendingLedgerQuery.refetch();
          }}
          contentContainerStyle={styles.listContent}
        />

        <AddEntrySheet
          visible={sheetVisible}
          onClose={() => setSheetVisible(false)}
          groupId={id ?? ''}
          chores={choresQuery.data ?? []}
          mode="head"
          members={nonHeadMembers}
        />

        <Sheet
          visible={!!fallbackUrl}
          onClose={() => setFallbackUrl(null)}
          title="Invite link"
        >
          <TextField
            value={fallbackUrl ?? ''}
            editable={false}
            selectTextOnFocus
          />
        </Sheet>
      </View>
    );
  }

  // ─── MEMBER VIEW ────────────────────────────────────────────────────────────
  const myEntries = myLedgerQuery.data ?? [];

  return (
    <View style={styles.screen}>
      <ScrollView contentContainerStyle={styles.memberScroll}>
        {/* Balance summary header */}
        <View style={styles.summaryCard}>
          <Text style={styles.summaryLabel}>Your balance</Text>
          <AmountText
            minorUnits={myBalance?.balance ?? 0}
            variant={
              (myBalance?.balance ?? 0) < 0 ? 'debit' : 'credit'
            }
            size="xl"
          />
          <Text style={styles.summaryHint}>
            {(myBalance?.balance ?? 0) < 0 ? 'you owe' : 'owed to you'}
          </Text>
          {!allowancesQuery.isError && (
            <AllowanceSummary
              current={memberCurrentAllow}
              upcoming={memberUpcomingAllow}
            />
          )}
        </View>

        <View style={styles.addButtonRow}>
          <Button
            title="Log a chore"
            variant="primary"
            icon="add"
            onPress={() => setSheetVisible(true)}
            fullWidth
          />
        </View>

        <View style={styles.addButtonRow}>
          <Button
            title="Leave group"
            variant="danger"
            loading={leaveMutation.isPending}
            onPress={handleLeaveGroup}
            fullWidth
          />
        </View>
      </ScrollView>

      <LedgerList
        entries={myEntries}
        chores={choresQuery.data ?? []}
        members={members}
        isHead={false}
        groupId={id ?? ''}
        loans={loansQuery.data ?? []}
        refreshing={myLedgerQuery.isFetching}
        onRefresh={() => myLedgerQuery.refetch()}
        emptyTitle="No entries yet"
        emptySubtitle="Log a chore to start earning"
      />

      <AddEntrySheet
        visible={sheetVisible}
        onClose={() => setSheetVisible(false)}
        groupId={id ?? ''}
        chores={choresQuery.data ?? []}
        mode="member"
        selfUserId={user?.id}
      />
    </View>
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
  header: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: 16,
    backgroundColor: theme.color.surface,
    borderBottomWidth: 1,
    borderBottomColor: theme.color.border,
  },
  memberCount: {
    fontSize: 15,
    color: theme.color.textSecondary,
  },
  addButtonRow: {
    padding: 16,
  },
  sectionTitle: {
    fontSize: 15,
    fontWeight: '600',
    color: theme.color.textSecondary,
    paddingHorizontal: 16,
    paddingBottom: 8,
  },
  listContent: {
    paddingBottom: 16,
  },
  summaryCard: {
    backgroundColor: theme.color.surface,
    padding: 24,
    alignItems: 'center',
    borderBottomWidth: 1,
    borderBottomColor: theme.color.border,
  },
  summaryLabel: {
    fontSize: 15,
    color: theme.color.textSecondary,
    marginBottom: 8,
  },
  summaryHint: {
    fontSize: 13,
    color: theme.color.textSecondary,
    marginTop: 4,
  },
  memberScroll: {
    flexGrow: 0,
  },
});
