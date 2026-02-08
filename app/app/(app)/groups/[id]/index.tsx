import { useState, useEffect, useCallback } from 'react';
import { View, Text, FlatList, TouchableOpacity, StyleSheet, RefreshControl, ActivityIndicator, Alert, Share, Platform } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import * as Clipboard from 'expo-clipboard';
import { groupsApi, ledgerApi, GroupDetail, Balance, Member } from '../../../../src/api';
import { useAuth } from '../../../../src/auth-context';

export default function GroupOverviewScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { user } = useAuth();
  const router = useRouter();
  const [group, setGroup] = useState<GroupDetail | null>(null);
  const [balances, setBalances] = useState<Balance[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState('');
  const [isHead, setIsHead] = useState(false);
  const [inviteLoading, setInviteLoading] = useState(false);

  const loadData = async () => {
    if (!id || !user?.id) return;
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
  }, [id, user?.id]);

  const onRefresh = useCallback(() => {
    setRefreshing(true);
    loadData();
  }, [id]);

  const handleGenerateInvite = async () => {
    if (!id) return;
    setInviteLoading(true);
    try {
      const invite = await groupsApi.createInvite(id);
      
      if (Platform.OS === 'web') {
        // Web: Copy to clipboard
        await Clipboard.setStringAsync(invite.invite_url);
        Alert.alert('Invite Created', 'Invite link copied to clipboard!');
      } else {
        // Mobile: Native share
        await Share.share({
          message: `Join my group "${group?.name}" on Pocket Money!\n\n${invite.invite_url}`,
          url: invite.invite_url,  // iOS only
          title: 'Join My Group',
        });
      }
    } catch (err) {
      Alert.alert('Error', err instanceof Error ? err.message : 'Failed to create invite');
    } finally {
      setInviteLoading(false);
    }
  };

  const handleMemberPress = (memberId: string, memberName: string) => {
    // Navigate to ledger screen filtered by this member
    router.push({
      pathname: `/(app)/groups/${id}/ledger`,
      params: { member_id: memberId, member_name: memberName },
    });
  };

  const renderMember = ({ item }: { item: Balance }) => (
    <TouchableOpacity 
      style={styles.memberCard}
      onPress={() => handleMemberPress(item.user_id, item.name)}
      activeOpacity={0.7}
    >
      <View style={styles.memberInfo}>
        <View style={styles.avatar}>
          <Text style={styles.avatarText}>
            {item.name.charAt(0).toUpperCase()}
          </Text>
        </View>
        <Text style={styles.memberName}>{item.name}</Text>
      </View>
      <View style={styles.balanceContainer}>
        <Text style={[styles.balance, item.balance >= 0 ? styles.positive : styles.negative]}>
          ${Math.abs(item.balance).toFixed(2)}
        </Text>
        {item.balance > 0 && <Text style={styles.balanceLabel}>owed</Text>}
        {item.balance < 0 && <Text style={styles.balanceLabel}>overpaid</Text>}
        <Ionicons name="chevron-forward" size={20} color="#ccc" />
      </View>
    </TouchableOpacity>
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
          {balances.length} {balances.length === 1 ? 'member' : 'members'}
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

      <Text style={styles.sectionTitle}>Members & Balances</Text>
      <Text style={styles.sectionSubtitle}>Tap a member to view their ledger</Text>

      {balances.length === 0 ? (
        <View style={styles.empty}>
          <Ionicons name="people-outline" size={64} color="#ccc" />
          <Text style={styles.emptyText}>No members yet</Text>
          <Text style={styles.emptySubtext}>Invite members to get started</Text>
        </View>
      ) : (
        <FlatList
          data={balances}
          renderItem={renderMember}
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
    paddingTop: 16,
    color: '#333',
  },
  sectionSubtitle: {
    fontSize: 14,
    color: '#666',
    paddingHorizontal: 16,
    paddingBottom: 8,
  },
  list: {
    padding: 16,
    paddingTop: 8,
  },
  memberCard: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    backgroundColor: '#fff',
    padding: 16,
    borderRadius: 12,
    marginBottom: 8,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 1 },
    shadowOpacity: 0.1,
    shadowRadius: 2,
    elevation: 2,
  },
  memberInfo: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 12,
  },
  avatar: {
    width: 44,
    height: 44,
    borderRadius: 22,
    backgroundColor: '#007AFF',
    justifyContent: 'center',
    alignItems: 'center',
  },
  avatarText: {
    fontSize: 18,
    fontWeight: '600',
    color: '#fff',
  },
  memberName: {
    fontSize: 16,
    fontWeight: '500',
    color: '#333',
  },
  balanceContainer: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  balance: {
    fontSize: 18,
    fontWeight: '700',
  },
  balanceLabel: {
    fontSize: 12,
    color: '#666',
  },
  positive: {
    color: '#34c759',
  },
  negative: {
    color: '#ff3b30',
  },
  empty: {
    flex: 1,
    padding: 32,
    alignItems: 'center',
    justifyContent: 'center',
  },
  emptyText: {
    fontSize: 18,
    fontWeight: '600',
    color: '#333',
    marginTop: 16,
  },
  emptySubtext: {
    fontSize: 14,
    color: '#666',
    marginTop: 4,
  },
});
