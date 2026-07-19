import { useState, useCallback } from 'react';
import { View, Text, SectionList, StyleSheet, RefreshControl } from 'react-native';
import { router } from 'expo-router';
import { useQueryClient } from '@tanstack/react-query';
import { useAuth } from '../../src/auth-context';
import { useGroups, useJoinGroup } from '../../src/hooks/useGroups';
import { qk } from '../../src/query-keys';
import {
  Button,
  Card,
  ListRow,
  Avatar,
  Sheet,
  TextField,
  AmountText,
  EmptyState,
  ErrorMessage,
  LoadingSpinner,
  ScreenContainer,
  HeaderBell,
  useToast,
} from '../../src/components';
import { theme } from '../../src/theme';
import { INVITES_ENABLED } from '../../src/flags';
import type { GroupSummary } from '../../src/api';

export default function DashboardScreen() {
  const { user } = useAuth();
  const qc = useQueryClient();
  const toast = useToast();
  const [joinVisible, setJoinVisible] = useState(false);
  const [inviteToken, setInviteToken] = useState('');

  const groupsQuery = useGroups();
  const joinMutation = useJoinGroup();

  const groups = groupsQuery.data ?? [];
  const headGroups = groups.filter(g => g.role === 'admin');
  const memberGroups = groups.filter(g => g.role === 'member');

  const onRefresh = useCallback(() => {
    qc.invalidateQueries({ queryKey: qk.groups() });
  }, [qc]);

  const closeJoin = useCallback(() => {
    setJoinVisible(false);
    setInviteToken('');
  }, []);

  const handleJoin = useCallback(async () => {
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
      closeJoin();
      toast.show({ tone: 'success', message: 'You have joined the group!' });
    } catch (err) {
      toast.show({
        tone: 'danger',
        message: err instanceof Error ? err.message : 'Failed to join group',
      });
    }
  }, [inviteToken, joinMutation, closeJoin, toast]);

  if (groupsQuery.isLoading) {
    return <LoadingSpinner />;
  }

  if (groupsQuery.isError) {
    return (
      <ErrorMessage
        message={groupsQuery.error instanceof Error ? groupsQuery.error.message : 'Failed to load groups'}
        onRetry={() => groupsQuery.refetch()}
      />
    );
  }

  const renderHeadGroup = ({ item }: { item: GroupSummary }) => (
    <View testID={`group-card-${item.id}`}>
      <Card onPress={() => router.push(`/(app)/groups/${item.id}`)} padded={false}>
        <ListRow
          left={<Avatar name={item.name} id={item.id} />}
          title={item.name}
          subtitle={`${item.member_count} ${item.member_count === 1 ? 'member' : 'members'}`}
          right={
            <View style={styles.groupRight}>
              <AmountText minorUnits={item.summary_balance.value} currency={item.summary_balance.currency} variant="neutral" size="md" />
              <Text style={styles.balanceCaption}>remaining to pay this month</Text>
            </View>
          }
        />
      </Card>
    </View>
  );

  const renderMemberGroup = ({ item }: { item: GroupSummary }) => (
    <View testID={`group-card-${item.id}`}>
      <Card onPress={() => router.push(`/(app)/groups/${item.id}`)} padded={false}>
        <ListRow
          left={<Avatar name={item.name} id={item.id} />}
          title={item.name}
          right={
            <View style={styles.groupRight}>
              <AmountText
                minorUnits={item.summary_balance.value}
                currency={item.summary_balance.currency}
                variant={item.summary_balance.value < 0 ? 'debit' : item.summary_balance.value === 0 ? 'neutral' : 'credit'}
                size="md"
              />
              <Text style={styles.balanceCaption}>to receive this month</Text>
            </View>
          }
        />
      </Card>
    </View>
  );

  const sections = [
    { title: 'Groups You Manage', data: headGroups, renderItem: renderHeadGroup },
    { title: "Groups You're In", data: memberGroups, renderItem: renderMemberGroup },
  ].filter(s => s.data.length > 0);

  return (
    <ScreenContainer style={styles.container} testID="dashboard-root">
      <View style={styles.header}>
        <Text style={styles.welcome}>Welcome, {user?.name || 'User'}!</Text>
        <HeaderBell />
      </View>

      <View style={styles.actions}>
        <Button
          variant="secondary"
          icon="add-circle"
          title="Create Group"
          onPress={() => router.push('/(app)/groups/create')}
          style={{ flex: 1 }}
          testID="dashboard-create-group"
        />
        {INVITES_ENABLED && (
          <Button
            variant="secondary"
            icon="enter"
            title="Join Group"
            onPress={() => setJoinVisible(true)}
            style={{ flex: 1 }}
            testID="dashboard-join-group"
          />
        )}
      </View>

      {groups.length === 0 ? (
        <View testID="dashboard-empty">
          <EmptyState
            icon="people-outline"
            title="No groups yet"
            subtitle="Create a group, or ask a group admin to add you by email."
          />
        </View>
      ) : (
        <SectionList
          sections={sections}
          keyExtractor={(item) => item.id}
          renderSectionHeader={({ section }) => (
            <Text style={styles.sectionTitle}>{section.title}</Text>
          )}
          refreshControl={
            <RefreshControl
              refreshing={groupsQuery.isRefetching}
              onRefresh={onRefresh}
            />
          }
          contentContainerStyle={styles.list}
          stickySectionHeadersEnabled={false}
        />
      )}

      {INVITES_ENABLED && (
        <Sheet
          visible={joinVisible}
          onClose={closeJoin}
          title="Join Group"
          footer={
            <>
              <Button
                variant="ghost"
                title="Cancel"
                onPress={closeJoin}
                style={{ flex: 1 }}
              />
              <Button
                title="Join"
                onPress={handleJoin}
                loading={joinMutation.isPending}
                style={{ flex: 1 }}
              />
            </>
          }
        >
          <Text style={styles.sheetSubtitle}>Paste the invite link or token</Text>
          <TextField
            placeholder="Invite link or token"
            value={inviteToken}
            onChangeText={setInviteToken}
            autoCapitalize="none"
            autoCorrect={false}
          />
        </Sheet>
      )}
    </ScreenContainer>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: theme.color.background,
  },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: theme.spacing.lg,
    backgroundColor: theme.color.surface,
    borderBottomWidth: 1,
    borderBottomColor: theme.color.border,
  },
  welcome: {
    flex: 1,
    fontSize: theme.fontSize.xl,
    fontWeight: theme.fontWeight.bold,
    color: theme.color.text,
  },
  actions: {
    flexDirection: 'row',
    padding: theme.spacing.lg,
    gap: theme.spacing.md,
  },
  sectionTitle: {
    fontSize: theme.fontSize.lg,
    fontWeight: theme.fontWeight.semibold,
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
  balanceCaption: {
    fontSize: theme.fontSize.xs,
    color: theme.color.textSecondary,
  },
  sheetSubtitle: {
    fontSize: theme.fontSize.sm,
    color: theme.color.textSecondary,
    marginBottom: theme.spacing.md,
  },
});
