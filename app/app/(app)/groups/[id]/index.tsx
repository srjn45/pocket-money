import { useState } from 'react';
import { FlatList, Platform, ScrollView, Share, StyleSheet, Text, View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import * as Clipboard from 'expo-clipboard';
import { useAuth } from '../../../../src/auth-context';
import { useGroup } from '../../../../src/hooks/useGroup';
import { useChores } from '../../../../src/hooks/useChores';
import { useLedger } from '../../../../src/hooks/useLedger';
import { useStatement } from '../../../../src/hooks/useStatement';
import { useAllowances } from '../../../../src/hooks/useAllowances';
import { useLoans } from '../../../../src/hooks/useLoans';
import {
  currentPeriod,
  prevPeriod,
  nextPeriod,
  currentAllowanceFor,
  upcomingAllowanceFor,
} from '../../../../src/allowance-format';
import { groupsApi } from '../../../../src/api';
import type { MemberStatement } from '../../../../src/api';
import { currencySymbol } from '../../../../src/money';
import { confirmAsync } from '../../../../src/confirm';
import { useLeaveGroup } from '../../../../src/hooks/useHygiene';
import { useDeleteGroup } from '../../../../src/hooks/useGroups';
import { INVITES_ENABLED } from '../../../../src/flags';
import { theme } from '../../../../src/theme';
import {
  Button,
  Sheet,
  TextField,
  AmountText,
  MonthHeader,
  StatementRow,
  RecordPaymentSheet,
  LedgerList,
  AddEntrySheet,
  AddMemberSheet,
  AllowanceSummary,
  EmptyState,
  ErrorMessage,
  LoadingSpinner,
  ScreenContainer,
  GroupSectionTabs,
  useToast,
} from '../../../../src/components';

export default function GroupOverviewScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const gid = id ?? '';
  const { user } = useAuth();
  const router = useRouter();
  const insets = useSafeAreaInsets(); // native: home-indicator inset; web: 0
  const { show: showToast } = useToast();

  // Statement month — LOCAL state, deliberately NOT a route param (keeps the
  // expo-router web-param gotcha off the statement fetch; only `id` comes from the URL).
  const [period, setPeriod] = useState(currentPeriod());

  const [sheetVisible, setSheetVisible] = useState(false);
  const [addMemberVisible, setAddMemberVisible] = useState(false);
  const [inviteLoading, setInviteLoading] = useState(false);
  const [fallbackUrl, setFallbackUrl] = useState<string | null>(null);
  const [paymentTarget, setPaymentTarget] = useState<MemberStatement | null>(null);

  const groupQuery = useGroup(gid);
  const statementQuery = useStatement(gid, period);
  const choresQuery = useChores(gid);
  const allowancesQuery = useAllowances(gid);
  const loansQuery = useLoans(gid);

  const group = groupQuery.data;
  const members = group?.members ?? [];
  const isHead = members.find(m => m.user_id === user?.id)?.role === 'admin';

  const myLedgerQuery = useLedger(gid, isHead ? undefined : {});
  const leaveMutation = useLeaveGroup(gid);
  const deleteGroupMutation = useDeleteGroup();

  const memberCurrentAllow = currentAllowanceFor(allowancesQuery.data ?? [], currentPeriod());
  const memberUpcomingAllow = upcomingAllowanceFor(allowancesQuery.data ?? [], currentPeriod());

  // Shadow status lives on the group membership, not the statement row — join by user_id.
  const shadowById = new Map(members.map(m => [m.user_id, m.status === 'shadow']));

  const isCurrentMonth = period === currentPeriod();

  async function handleLeaveGroup() {
    const confirmed = await confirmAsync({
      title: 'Leave group',
      message: `Leave ${group?.name ?? 'this group'}? You can be re-added by the group admin. This only works if your balance is ${currencySymbol(group?.currency ?? 'INR')}0 and you have no active or pending loans.`,
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

  async function handleDeleteGroup() {
    const confirmed = await confirmAsync({
      title: 'Delete group',
      message: `Delete ${group?.name ?? 'this group'}? It will be removed from everyone's list and no longer appear anywhere in the app. Its history is archived, not permanently erased.`,
      confirmLabel: 'Delete',
      destructive: true,
    });
    if (!confirmed) return;
    try {
      await deleteGroupMutation.mutateAsync(gid);
      showToast({ tone: 'success', message: 'Group deleted' });
      router.replace('/(app)' as never);
    } catch (e) {
      showToast({ tone: 'danger', message: e instanceof Error ? e.message : 'Failed to delete group' });
    }
  }

  async function handleInvite() {
    if (!gid) return;
    setInviteLoading(true);
    try {
      const invite = await groupsApi.createInvite(gid);
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

  if (groupQuery.isLoading) {
    return <View style={styles.centered}><LoadingSpinner /></View>;
  }

  if (groupQuery.error) {
    return <ErrorMessage message={groupQuery.error instanceof Error ? groupQuery.error.message : 'Failed to load group'} />;
  }

  if (!group) {
    return <ErrorMessage message="Failed to load group" />;
  }

  const currency = statementQuery.data?.currency ?? group.currency;
  const stmt = statementQuery.data;
  const nonHeadMembers = members.filter(m => m.role !== 'admin');

  const monthSwitcher = (
    <MonthHeader
      period={period}
      onPrev={() => setPeriod(prevPeriod(period))}
      onNext={() => setPeriod(nextPeriod(period))}
      nextDisabled={isCurrentMonth}
      totalMinorUnits={stmt?.group_total?.closing_balance.value}
      currency={stmt?.group_total ? currency : undefined}
      totalVariant="neutral"
    />
  );

  // ─── HEAD VIEW ─────────────────────────────────────────────────────────────
  if (isHead) {
    const groupTotal = stmt?.group_total ?? null;

    return (
      <ScreenContainer style={styles.screen} testID="group-overview-root">
        <GroupSectionTabs groupId={gid} active="overview" />
        <View style={styles.header}>
          <Text style={styles.memberCount}>
            {nonHeadMembers.length} {nonHeadMembers.length === 1 ? 'member' : 'members'}
          </Text>
          <View style={styles.headerActions}>
            <Button
              title="Chores"
              variant="ghost"
              icon="list-outline"
              onPress={() => router.push(`/(app)/groups/${gid}/chores` as never)}
              size="sm"
              testID="overview-chores-link"
            />
            <Button
              title="Add member"
              variant="ghost"
              icon="person-add"
              onPress={() => setAddMemberVisible(true)}
              size="sm"
              testID="group-add-member-button"
            />
            {INVITES_ENABLED && (
              <Button
                title="Invite"
                variant="ghost"
                icon="person-add"
                loading={inviteLoading}
                onPress={handleInvite}
                size="sm"
                testID="group-invite-button"
              />
            )}
          </View>
        </View>

        {monthSwitcher}

        {groupTotal && (
          <View style={styles.totalCard} testID="statement-group-total">
            <Text style={styles.totalLabel}>Remaining to pay this month</Text>
            <AmountText
              minorUnits={groupTotal.closing_balance.value}
              currency={currency}
              variant={groupTotal.closing_balance.value === 0 ? 'neutral' : 'credit'}
              size="xl"
            />
            <View style={styles.totalSubRow}>
              <Text style={styles.totalSub}>
                Payable <AmountText minorUnits={groupTotal.total_due.value} currency={currency} variant="neutral" size="sm" />
              </Text>
              <Text style={styles.totalSub}>
                Paid <AmountText minorUnits={groupTotal.cleared.value} currency={currency} variant="neutral" size="sm" />
              </Text>
            </View>
          </View>
        )}

        <View style={styles.addButtonRow}>
          <Button
            title="Add entry"
            variant="primary"
            icon="add"
            onPress={() => setSheetVisible(true)}
            fullWidth
            testID="statement-add-entry"
          />
        </View>

        {statementQuery.isLoading ? (
          <View style={styles.centered}><LoadingSpinner /></View>
        ) : (
          <FlatList
            data={stmt?.members ?? []}
            keyExtractor={m => m.user_id}
            renderItem={({ item }) => (
              <StatementRow
                member={item}
                currency={currency}
                isShadow={shadowById.get(item.user_id)}
                onPress={() =>
                  router.push({
                    pathname: `/(app)/groups/${gid}/members/${item.user_id}`,
                    params: { name: item.name },
                  })
                }
                onRecordPayment={isCurrentMonth ? () => setPaymentTarget(item) : undefined}
              />
            )}
            ListEmptyComponent={
              <View testID="statement-empty">
                <EmptyState
                  icon="people-outline"
                  title="No members yet"
                  subtitle="Tap Add member to add your family"
                />
              </View>
            }
            refreshing={groupQuery.isFetching || statementQuery.isFetching}
            onRefresh={() => {
              groupQuery.refetch();
              statementQuery.refetch();
            }}
            ListFooterComponent={
              <View style={styles.deleteButtonRow}>
                <Button
                  title="Delete group"
                  variant="danger"
                  loading={deleteGroupMutation.isPending}
                  onPress={handleDeleteGroup}
                  fullWidth
                  testID="group-delete-button"
                />
              </View>
            }
            contentContainerStyle={[styles.listContent, { paddingBottom: theme.spacing.lg + insets.bottom }]}
          />
        )}

        <AddEntrySheet
          visible={sheetVisible}
          onClose={() => setSheetVisible(false)}
          groupId={gid}
          currency={currency}
          chores={choresQuery.data ?? []}
          mode="head"
          members={nonHeadMembers}
        />

        <RecordPaymentSheet
          visible={!!paymentTarget}
          onClose={() => setPaymentTarget(null)}
          groupId={gid}
          currency={currency}
          member={paymentTarget}
        />

        <AddMemberSheet
          visible={addMemberVisible}
          onClose={() => setAddMemberVisible(false)}
          groupId={gid}
          currency={currency}
        />

        {INVITES_ENABLED && (
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
        )}
      </ScreenContainer>
    );
  }

  // ─── MEMBER VIEW ────────────────────────────────────────────────────────────
  const myRow = stmt?.members?.[0] ?? null;
  const myEntries = myLedgerQuery.data ?? [];

  return (
    <ScreenContainer style={styles.screen} testID="group-overview-root">
      <GroupSectionTabs groupId={gid} active="overview" />
      <ScrollView contentContainerStyle={styles.memberScroll}>
        {monthSwitcher}

        {statementQuery.isLoading ? (
          <View style={styles.centered}><LoadingSpinner /></View>
        ) : myRow ? (
          <>
            <View style={styles.bannerCard} testID="statement-receive-banner">
              {myRow.closing_balance.value >= 0 ? (
                <Text style={styles.bannerText}>
                  You&apos;ll receive{' '}
                  <AmountText minorUnits={myRow.closing_balance.value} currency={currency} variant="credit" size="lg" />
                  {' '}this month.
                </Text>
              ) : (
                <Text style={styles.bannerText}>
                  You owe{' '}
                  <AmountText minorUnits={myRow.closing_balance.value} currency={currency} variant="debit" size="lg" />
                  {' '}this month.
                </Text>
              )}
            </View>

            <StatementRow member={myRow} currency={currency} />
          </>
        ) : (
          <View testID="statement-empty">
            <EmptyState
              icon="calendar-outline"
              title="No statement for this month"
              subtitle="Nothing has been posted for you in this period yet."
            />
          </View>
        )}

        {!allowancesQuery.isError && (
          <View style={styles.allowanceCard}>
            <AllowanceSummary
              current={memberCurrentAllow}
              upcoming={memberUpcomingAllow}
              currency={currency}
            />
          </View>
        )}

        <View style={styles.addButtonRow}>
          <Button
            title="Log a chore"
            variant="primary"
            icon="add"
            onPress={() => setSheetVisible(true)}
            fullWidth
            testID="member-log-chore"
          />
        </View>

        <View style={styles.addButtonRow}>
          <Button
            title="Leave group"
            variant="danger"
            loading={leaveMutation.isPending}
            onPress={handleLeaveGroup}
            fullWidth
            testID="group-leave-button"
          />
        </View>
      </ScrollView>

      <LedgerList
        entries={myEntries}
        chores={choresQuery.data ?? []}
        members={members}
        isHead={false}
        groupId={gid}
        currency={currency}
        loans={loansQuery.data ?? []}
        refreshing={myLedgerQuery.isFetching}
        onRefresh={() => myLedgerQuery.refetch()}
        emptyTitle="No entries yet"
        emptySubtitle="Log a chore to start earning"
      />

      <AddEntrySheet
        visible={sheetVisible}
        onClose={() => setSheetVisible(false)}
        groupId={gid}
        currency={currency}
        chores={choresQuery.data ?? []}
        mode="member"
        selfUserId={user?.id}
      />
    </ScreenContainer>
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
    padding: theme.spacing.xl,
  },
  header: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: theme.spacing.lg,
    backgroundColor: theme.color.surface,
    borderBottomWidth: 1,
    borderBottomColor: theme.color.border,
  },
  memberCount: {
    fontSize: theme.fontSize.sm,
    color: theme.color.textSecondary,
  },
  headerActions: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: theme.spacing.sm,
  },
  addButtonRow: {
    padding: theme.spacing.lg,
  },
  deleteButtonRow: {
    padding: theme.spacing.lg,
    paddingTop: theme.spacing.xl,
  },
  listContent: {
    paddingBottom: theme.spacing.lg,
  },
  totalCard: {
    backgroundColor: theme.color.surface,
    padding: theme.spacing.xl,
    alignItems: 'center',
    borderBottomWidth: 1,
    borderBottomColor: theme.color.border,
    gap: theme.spacing.xs,
  },
  totalLabel: {
    fontSize: theme.fontSize.sm,
    color: theme.color.textSecondary,
  },
  totalSubRow: {
    flexDirection: 'row',
    gap: theme.spacing.lg,
    marginTop: theme.spacing.xs,
  },
  totalSub: {
    fontSize: theme.fontSize.xs,
    color: theme.color.textSecondary,
  },
  bannerCard: {
    backgroundColor: theme.color.surface,
    padding: theme.spacing.xl,
    borderBottomWidth: 1,
    borderBottomColor: theme.color.border,
  },
  bannerText: {
    fontSize: theme.fontSize.md,
    color: theme.color.text,
    lineHeight: 26,
  },
  allowanceCard: {
    paddingHorizontal: theme.spacing.lg,
  },
  memberScroll: {
    flexGrow: 0,
  },
});
