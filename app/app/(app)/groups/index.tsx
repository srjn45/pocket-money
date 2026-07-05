import { useState, useEffect, useCallback } from 'react';
import { View, Text, FlatList, StyleSheet, RefreshControl } from 'react-native';
import { router } from 'expo-router';
import { groupsApi, Group } from '../../../src/api';
import { Button, Card, ListRow, Sheet, TextField, EmptyState, LoadingSpinner, useToast } from '../../../src/components';
import { theme } from '../../../src/theme';

export default function GroupsListScreen() {
  const toast = useToast();
  const [groups, setGroups] = useState<Group[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState('');
  const [joinModalVisible, setJoinModalVisible] = useState(false);
  const [inviteToken, setInviteToken] = useState('');
  const [joining, setJoining] = useState(false);

  const loadGroups = async () => {
    try {
      setError('');
      const data = await groupsApi.list();
      setGroups(data || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load groups');
    } finally {
      setIsLoading(false);
      setRefreshing(false);
    }
  };

  useEffect(() => {
    loadGroups();
  }, []);

  const onRefresh = useCallback(() => {
    setRefreshing(true);
    loadGroups();
  }, []);

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

    setJoining(true);
    try {
      await groupsApi.join(joinToken);
      setJoinModalVisible(false);
      setInviteToken('');
      loadGroups();
      toast.show({ tone: 'success', message: 'You have joined the group!' });
    } catch (err) {
      toast.show({ tone: 'danger', message: err instanceof Error ? err.message : 'Failed to join group' });
    } finally {
      setJoining(false);
    }
  };

  const renderGroup = ({ item }: { item: Group }) => (
    <Card onPress={() => router.push(`/(app)/groups/${item.id}`)} padded={false}>
      <ListRow
        title={item.name}
        subtitle={`Created ${new Date(item.created_at).toLocaleDateString()}`}
      />
    </Card>
  );

  if (isLoading) {
    return <LoadingSpinner />;
  }

  return (
    <View style={styles.container}>
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

      {groups.length === 0 ? (
        <EmptyState
          icon="people-outline"
          title="No groups yet"
          subtitle="Create a group or join one with an invite link"
        />
      ) : (
        <FlatList
          data={groups}
          renderItem={renderGroup}
          keyExtractor={(item) => item.id}
          refreshControl={
            <RefreshControl refreshing={refreshing} onRefresh={onRefresh} />
          }
          contentContainerStyle={styles.list}
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
              loading={joining}
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
  list: {
    padding: theme.spacing.lg,
    paddingTop: 0,
  },
  modalSubtitle: {
    fontSize: theme.fontSize.sm,
    color: theme.color.textSecondary,
    marginBottom: theme.spacing.md,
  },
});
