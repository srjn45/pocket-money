import { useState, useCallback } from 'react';
import { View, Text, SectionList, StyleSheet, RefreshControl } from 'react-native';
import { router } from 'expo-router';
import { useQueries, useQueryClient } from '@tanstack/react-query';
import { groupsApi, ledgerApi, Group } from '../../src/api';
import { formatMinorUnits } from '../../src/money';
import { useAuth } from '../../src/auth-context';
import { useGroups, useJoinGroup } from '../../src/hooks/useGroups';
import { qk } from '../../src/query-keys';
import { Button, Card, ListRow, Sheet, TextField, EmptyState, LoadingSpinner, useToast } from '../../src/components';
import { theme } from '../../src/theme';

interface GroupWithDetails extends Group {
  memberCount?: number;
  totalBalance?: number;
  userRole: 'head' | 'member';
}

export default function DashboardScreen() {
  const { user } = useAuth();
  const qc = useQueryClient();
  const toast = useToast();
  const [joinModalVisible, setJoinModalVisible] = useState(false);
  const [inviteToken, setInviteToken] = useState('');

  const groupsQuery = useGroups();
  const groups = groupsQuery.data ?? [];

  const detailQueries = useQueries({
    queries: groups.map(g => ({
      queryKey: qk.group(g.id),
      queryFn: () => groupsApi.get(g.id),
    })),
  });

  const balanceQueries = useQueries({
    queries: groups.map(g => ({
      queryKey: qk.balance(g.id),
      queryFn: () => ledgerApi.getBalance(g.id),
    })),
  });

  const isLoading =
    groupsQuery.isLoading ||
    (groups.length > 0 && (
      detailQueries.some(q => q.isLoading) ||
      balanceQueries.some(q => q.isLoading)
    ));

  const isRefetching =
    groupsQuery.isRefetching ||
    detailQueries.some(q => q.isRefetching) ||
    balanceQueries.some(q => q.isRefetching);

  const error = groupsQuery.error instanceof Error ? groupsQuery.error.message : '';

  const onRefresh = useCallback(() => {
    qc.invalidateQueries({ queryKey: qk.groups() });
    groups.forEach(g => {
      qc.invalidateQueries({ queryKey: qk.group(g.id) });
      qc.invalidateQueries({ queryKey: qk.balance(g.id) });
    });
  }, [qc, groups]);

  const enrichedGroups: GroupWithDetails[] = groups.map((group, i) => {
    const isHead = group.head_user_id === user?.id;
    const detail = detailQueries[i]?.data;
    const balance = balanceQueries[i]?.data ?? [];

    const memberCount = detail?.members.length;
    let totalBalance = 0;
    if (isHead) {
      totalBalance = balance.reduce((sum, b) => sum + b.balance, 0);
    } else {
      const userBalance = balance.find(b => b.user_id === user?.id);
      totalBalance = userBalance?.balance ?? 0;
    }

    return { ...group, memberCount, totalBalance, userRole: isHead ? 'head' : 'member' };
  });

  const headGroups = enrichedGroups.filter(g => g.userRole === 'head');
  const memberGroups = enrichedGroups.filter(g => g.userRole === 'member');

  const joinMutation = useJoinGroup();

  const handleJoinGroup = async () => {
    if (!inviteToken.trim()) {
      toast.show({ tone: 'danger', message: 'Please enter an invite token' });
      return;
    }

    let joinToken = inviteToken.trim();
    const tokenMatch = joinToken.match(/token=([^&]+)/);
    if (tokenMatch) {
      joinToken = tokenMatch[1];
    }

    try {
      await joinMutation.mutateAsync(joinToken);
      setJoinModalVisible(false);
      setInviteToken('');
      toast.show({ tone: 'success', message: 'You have joined the group!' });
    } catch (err) {
      toast.show({ tone: 'danger', message: err instanceof Error ? err.message : 'Failed to join group' });
    }
  };

  const renderHeadGroup = ({ item }: { item: GroupWithDetails }) => (
    <Card onPress={() => router.push(`/(app)/groups/${item.id}`)} padded={false}>
      <ListRow
        title={item.name}
        subtitle={`${item.memberCount || 0} ${(item.memberCount || 0) === 1 ? 'member' : 'members'}`}
        right={
          <View style={styles.groupRight}>
            <Text style={[styles.balanceText, styles.owedAmount]}>
              {formatMinorUnits(item.totalBalance || 0)}
            </Text>
            <Text style={styles.balanceLabel}>owed</Text>
          </View>
        }
      />
    </Card>
  );

  const renderMemberGroup = ({ item }: { item: GroupWithDetails }) => (
    <Card onPress={() => router.push(`/(app)/groups/${item.id}`)} padded={false}>
      <ListRow
        title={item.name}
        right={
          <View style={styles.groupRight}>
            <Text style={[styles.balanceText, (item.totalBalance || 0) >= 0 ? styles.earnedAmount : styles.owedAmount]}>
              {formatMinorUnits(Math.abs(item.totalBalance || 0))}
            </Text>
            {(item.totalBalance || 0) > 0 && <Text style={styles.balanceLabel}>earned</Text>}
            {(item.totalBalance || 0) < 0 && <Text style={styles.balanceLabel}>owed</Text>}
          </View>
        }
      />
    </Card>
  );

  if (isLoading) {
    return <LoadingSpinner />;
  }

  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <Text style={styles.welcome}>Welcome, {user?.name || 'User'}!</Text>
      </View>

      {error ? <Text style={styles.error}>{error}</Text> : null}

      <View style={styles.actions}>
        <Button
          variant="secondary"
          icon="add-circle"
          title="Create Group"
          onPress={() => router.push('/(app)/groups/create')}
          style={{ flex: 1 }}
        />
        <Button
          variant="secondary"
          icon="enter"
          title="Join Group"
          onPress={() => setJoinModalVisible(true)}
          style={{ flex: 1 }}
        />
      </View>

      {headGroups.length === 0 && memberGroups.length === 0 ? (
        <EmptyState
          icon="people-outline"
          title="No groups yet"
          subtitle="Create a group or join one with an invite link"
        />
      ) : (
        <SectionList
          sections={[
            { title: 'Groups You Manage', data: headGroups, renderItem: renderHeadGroup },
            { title: "Groups You're In", data: memberGroups, renderItem: renderMemberGroup },
          ].filter(s => s.data.length > 0)}
          keyExtractor={(item) => item.id}
          renderSectionHeader={({ section }) => (
            <Text style={styles.sectionTitle}>{section.title}</Text>
          )}
          refreshControl={
            <RefreshControl refreshing={isRefetching} onRefresh={onRefresh} />
          }
          contentContainerStyle={styles.list}
          stickySectionHeadersEnabled={false}
        />
      )}

      <Sheet
        visible={joinModalVisible}
        onClose={() => { setJoinModalVisible(false); setInviteToken(''); }}
        title="Join Group"
        footer={
          <>
            <Button
              variant="ghost"
              title="Cancel"
              onPress={() => { setJoinModalVisible(false); setInviteToken(''); }}
              style={{ flex: 1 }}
            />
            <Button
              title="Join"
              onPress={handleJoinGroup}
              loading={joinMutation.isPending}
              style={{ flex: 1 }}
            />
          </>
        }
      >
        <Text style={styles.modalSubtitle}>Paste the invite link or token</Text>
        <TextField
          placeholder="Invite link or token"
          value={inviteToken}
          onChangeText={setInviteToken}
          autoCapitalize="none"
          autoCorrect={false}
        />
      </Sheet>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: theme.color.background,
  },
  header: {
    padding: theme.spacing.lg,
    backgroundColor: theme.color.surface,
    borderBottomWidth: 1,
    borderBottomColor: theme.color.border,
  },
  welcome: {
    fontSize: 24,
    fontWeight: 'bold',
    color: theme.color.text,
  },
  error: {
    color: theme.color.danger,
    padding: theme.spacing.lg,
    textAlign: 'center',
  },
  actions: {
    flexDirection: 'row',
    padding: theme.spacing.lg,
    gap: theme.spacing.md,
  },
  sectionTitle: {
    fontSize: 18,
    fontWeight: '600',
    paddingHorizontal: theme.spacing.lg,
    paddingVertical: theme.spacing.sm,
    color: theme.color.text,
  },
  list: {
    padding: theme.spacing.lg,
    gap: theme.spacing.md,
  },
  groupRight: {
    alignItems: 'flex-end',
    gap: 2,
  },
  balanceText: {
    fontSize: 16,
    fontWeight: '700',
  },
  balanceLabel: {
    fontSize: 12,
    color: theme.color.textSecondary,
  },
  earnedAmount: {
    color: theme.color.success,
  },
  owedAmount: {
    color: theme.color.warning,
  },
  modalSubtitle: {
    fontSize: theme.fontSize.sm,
    color: theme.color.textSecondary,
    marginBottom: theme.spacing.md,
  },
});
