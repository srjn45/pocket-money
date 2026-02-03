import { useState, useEffect, useCallback } from 'react';
import { View, Text, FlatList, TouchableOpacity, StyleSheet, RefreshControl, ActivityIndicator, Alert } from 'react-native';
import { useLocalSearchParams } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import * as Clipboard from 'expo-clipboard';
import { groupsApi, ledgerApi, GroupDetail, Balance, Member } from '../../../../src/api';
import { useAuth } from '../../../../src/auth-context';

export default function GroupOverviewScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { user } = useAuth();
  const [group, setGroup] = useState<GroupDetail | null>(null);
  const [balances, setBalances] = useState<Balance[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState('');
  const [isHead, setIsHead] = useState(false);
  const [inviteLoading, setInviteLoading] = useState(false);

  const loadData = async () => {
    if (!id) return;
    try {
      setError('');
      const [groupData, balanceData] = await Promise.all([
        groupsApi.get(id),
        ledgerApi.getBalance(id),
      ]);
      setGroup(groupData);
      setBalances(balanceData || []);
      const currentMember = groupData.members.find((m: Member) => m.user_id === user?.id);
      setIsHead(currentMember?.role === 'head');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load group');
    } finally {
      setIsLoading(false);
      setRefreshing(false);
    }
  };

  useEffect(() => {
    loadData();
  }, [id]);

  const onRefresh = useCallback(() => {
    setRefreshing(true);
    loadData();
  }, [id]);

  const handleGenerateInvite = async () => {
    if (!id) return;
    setInviteLoading(true);
    try {
      const invite = await groupsApi.createInvite(id);
      await Clipboard.setStringAsync(invite.invite_url);
      Alert.alert('Invite Created', 'Invite link copied to clipboard!');
    } catch (err) {
      Alert.alert('Error', err instanceof Error ? err.message : 'Failed to create invite');
    } finally {
      setInviteLoading(false);
    }
  };

  const renderBalance = ({ item }: { item: Balance }) => (
    <View style={styles.balanceCard}>
      <Text style={styles.memberName}>{item.name}</Text>
      <Text style={[styles.balance, item.balance >= 0 ? styles.positive : styles.negative]}>
        ${item.balance.toFixed(2)}
      </Text>
    </View>
  );

  if (isLoading) {
    return (
      <View style={styles.centered}>
        <ActivityIndicator size="large" color="#007AFF" />
      </View>
    );
  }

  return (
    <View style={styles.container}>
      {error ? <Text style={styles.error}>{error}</Text> : null}

      <View style={styles.header}>
        <Text style={styles.membersCount}>
          {group?.members.length || 0} members
        </Text>
        {isHead && (
          <TouchableOpacity 
            style={styles.inviteButton}
            onPress={handleGenerateInvite}
            disabled={inviteLoading}
          >
            {inviteLoading ? (
              <ActivityIndicator size="small" color="#007AFF" />
            ) : (
              <>
                <Ionicons name="person-add" size={20} color="#007AFF" />
                <Text style={styles.inviteText}>Invite</Text>
              </>
            )}
          </TouchableOpacity>
        )}
      </View>

      <Text style={styles.sectionTitle}>Balances</Text>

      {balances.length === 0 ? (
        <View style={styles.empty}>
          <Text style={styles.emptyText}>No balance data yet</Text>
        </View>
      ) : (
        <FlatList
          data={balances}
          renderItem={renderBalance}
          keyExtractor={(item) => item.user_id}
          refreshControl={
            <RefreshControl refreshing={refreshing} onRefresh={onRefresh} />
          }
          contentContainerStyle={styles.list}
        />
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f5f5f5',
  },
  centered: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
  },
  error: {
    color: '#ff3b30',
    padding: 16,
    textAlign: 'center',
  },
  header: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: 16,
    backgroundColor: '#fff',
    borderBottomWidth: 1,
    borderBottomColor: '#eee',
  },
  membersCount: {
    fontSize: 16,
    color: '#666',
  },
  inviteButton: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 4,
    padding: 8,
  },
  inviteText: {
    color: '#007AFF',
    fontWeight: '600',
  },
  sectionTitle: {
    fontSize: 18,
    fontWeight: '600',
    paddingHorizontal: 16,
    paddingVertical: 12,
    color: '#333',
  },
  list: {
    padding: 16,
    paddingTop: 0,
  },
  balanceCard: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    backgroundColor: '#fff',
    padding: 16,
    borderRadius: 8,
    marginBottom: 8,
  },
  memberName: {
    fontSize: 16,
    color: '#333',
  },
  balance: {
    fontSize: 18,
    fontWeight: '600',
  },
  positive: {
    color: '#34c759',
  },
  negative: {
    color: '#ff3b30',
  },
  empty: {
    padding: 32,
    alignItems: 'center',
  },
  emptyText: {
    fontSize: 16,
    color: '#666',
  },
});
