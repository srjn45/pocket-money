import { useState, useEffect, useCallback } from 'react';
import { View, Text, SectionList, TouchableOpacity, StyleSheet, RefreshControl, ActivityIndicator, TextInput, Modal, Alert } from 'react-native';
import { router } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { groupsApi, ledgerApi, Group } from '../../src/api';
import { useAuth } from '../../src/auth-context';

interface GroupWithDetails extends Group {
  memberCount?: number;
  totalBalance?: number;
  userRole: 'head' | 'member';
}

export default function DashboardScreen() {
  const { user } = useAuth();
  const [headGroups, setHeadGroups] = useState<GroupWithDetails[]>([]);
  const [memberGroups, setMemberGroups] = useState<GroupWithDetails[]>([]);
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
      
      // Fetch details for each group
      const groupsWithDetails = await Promise.all(
        (data || []).map(async (group) => {
          const isHead = group.head_user_id === user?.id;
          try {
            const [detail, balance] = await Promise.all([
              groupsApi.get(group.id),
              ledgerApi.getBalance(group.id),
            ]);
            
            const memberCount = detail.members.length;
            let totalBalance = 0;
            
            if (isHead) {
              // Sum all member balances (total owed to members)
              totalBalance = balance.reduce((sum, b) => sum + b.balance, 0);
            } else {
              // Find current user's balance
              const userBalance = balance.find(b => b.user_id === user?.id);
              totalBalance = userBalance?.balance ?? 0;
            }
            
            return { ...group, memberCount, totalBalance, userRole: isHead ? 'head' : 'member' } as GroupWithDetails;
          } catch {
            return { ...group, userRole: isHead ? 'head' : 'member' } as GroupWithDetails;
          }
        })
      );
      
      setHeadGroups(groupsWithDetails.filter(g => g.userRole === 'head'));
      setMemberGroups(groupsWithDetails.filter(g => g.userRole === 'member'));
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
      Alert.alert('Error', 'Please enter an invite token');
      return;
    }

    // Extract token from URL if full URL is pasted
    let token = inviteToken.trim();
    const tokenMatch = token.match(/token=([^&]+)/);
    if (tokenMatch) {
      token = tokenMatch[1];
    }

    setJoining(true);
    try {
      await groupsApi.join(token);
      setJoinModalVisible(false);
      setInviteToken('');
      loadGroups();
      Alert.alert('Success', 'You have joined the group!');
    } catch (err) {
      Alert.alert('Error', err instanceof Error ? err.message : 'Failed to join group');
    } finally {
      setJoining(false);
    }
  };

  const renderHeadGroup = ({ item }: { item: GroupWithDetails }) => (
    <TouchableOpacity 
      style={styles.groupCard}
      onPress={() => router.push(`/(app)/groups/${item.id}`)}
    >
      <View style={styles.groupInfo}>
        <Text style={styles.groupName}>{item.name}</Text>
        <Text style={styles.groupMeta}>
          {item.memberCount || 0} {(item.memberCount || 0) === 1 ? 'member' : 'members'}
        </Text>
      </View>
      <View style={styles.groupRight}>
        <Text style={[styles.balanceText, styles.owedAmount]}>
          ${(item.totalBalance || 0).toFixed(2)}
        </Text>
        <Text style={styles.balanceLabel}>owed</Text>
        <Ionicons name="chevron-forward" size={24} color="#ccc" />
      </View>
    </TouchableOpacity>
  );

  const renderMemberGroup = ({ item }: { item: GroupWithDetails }) => (
    <TouchableOpacity 
      style={styles.groupCard}
      onPress={() => router.push(`/(app)/groups/${item.id}`)}
    >
      <View style={styles.groupInfo}>
        <Text style={styles.groupName}>{item.name}</Text>
      </View>
      <View style={styles.groupRight}>
        <Text style={[styles.balanceText, (item.totalBalance || 0) >= 0 ? styles.earnedAmount : styles.owedAmount]}>
          ${Math.abs(item.totalBalance || 0).toFixed(2)}
        </Text>
        {(item.totalBalance || 0) > 0 && <Text style={styles.balanceLabel}>earned</Text>}
        {(item.totalBalance || 0) < 0 && <Text style={styles.balanceLabel}>owed</Text>}
        <Ionicons name="chevron-forward" size={24} color="#ccc" />
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
      <View style={styles.header}>
        <Text style={styles.welcome}>Welcome, {user?.name || 'User'}!</Text>
      </View>

      {error ? <Text style={styles.error}>{error}</Text> : null}

      <View style={styles.actions}>
        <TouchableOpacity 
          style={styles.actionButton}
          onPress={() => router.push('/(app)/groups/create')}
        >
          <Ionicons name="add-circle" size={24} color="#007AFF" />
          <Text style={styles.actionText}>Create Group</Text>
        </TouchableOpacity>
        <TouchableOpacity 
          style={styles.actionButton}
          onPress={() => setJoinModalVisible(true)}
        >
          <Ionicons name="enter" size={24} color="#007AFF" />
          <Text style={styles.actionText}>Join Group</Text>
        </TouchableOpacity>
      </View>

      {headGroups.length === 0 && memberGroups.length === 0 ? (
        <View style={styles.empty}>
          <Ionicons name="people-outline" size={64} color="#ccc" />
          <Text style={styles.emptyText}>No groups yet</Text>
          <Text style={styles.emptySubtext}>Create a group or join one with an invite link</Text>
        </View>
      ) : (
        <SectionList
          sections={[
            { title: 'Groups You Manage', data: headGroups, renderItem: renderHeadGroup },
            { title: 'Groups You\'re In', data: memberGroups, renderItem: renderMemberGroup },
          ].filter(s => s.data.length > 0)}
          keyExtractor={(item) => item.id}
          renderSectionHeader={({ section }) => (
            <Text style={styles.sectionTitle}>{section.title}</Text>
          )}
          refreshControl={
            <RefreshControl refreshing={refreshing} onRefresh={onRefresh} />
          }
          contentContainerStyle={styles.list}
          stickySectionHeadersEnabled={false}
        />
      )}

      <Modal visible={joinModalVisible} animationType="slide" transparent>
        <View style={styles.modalOverlay}>
          <View style={styles.modalContent}>
            <Text style={styles.modalTitle}>Join Group</Text>
            <Text style={styles.modalSubtitle}>
              Paste the invite link or token
            </Text>
            
            <TextInput
              style={styles.input}
              placeholder="Invite link or token"
              value={inviteToken}
              onChangeText={setInviteToken}
              autoCapitalize="none"
              autoCorrect={false}
            />

            <View style={styles.modalButtons}>
              <TouchableOpacity
                style={[styles.modalButton, styles.cancelButton]}
                onPress={() => {
                  setJoinModalVisible(false);
                  setInviteToken('');
                }}
              >
                <Text style={styles.cancelButtonText}>Cancel</Text>
              </TouchableOpacity>
              <TouchableOpacity
                style={[styles.modalButton, styles.joinButton]}
                onPress={handleJoinGroup}
                disabled={joining}
              >
                {joining ? (
                  <ActivityIndicator color="#fff" />
                ) : (
                  <Text style={styles.joinButtonText}>Join</Text>
                )}
              </TouchableOpacity>
            </View>
          </View>
        </View>
      </Modal>
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
  header: {
    padding: 16,
    backgroundColor: '#fff',
    borderBottomWidth: 1,
    borderBottomColor: '#eee',
  },
  welcome: {
    fontSize: 24,
    fontWeight: 'bold',
    color: '#333',
  },
  error: {
    color: '#ff3b30',
    padding: 16,
    textAlign: 'center',
  },
  actions: {
    flexDirection: 'row',
    padding: 16,
    gap: 12,
  },
  actionButton: {
    flex: 1,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: '#fff',
    padding: 16,
    borderRadius: 8,
    gap: 8,
    borderWidth: 1,
    borderColor: '#007AFF',
  },
  actionText: {
    color: '#007AFF',
    fontWeight: '600',
  },
  sectionTitle: {
    fontSize: 18,
    fontWeight: '600',
    paddingHorizontal: 16,
    paddingVertical: 8,
    color: '#333',
  },
  list: {
    padding: 16,
    gap: 12,
  },
  groupCard: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: '#fff',
    padding: 16,
    borderRadius: 8,
    marginBottom: 12,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 1 },
    shadowOpacity: 0.1,
    shadowRadius: 2,
    elevation: 2,
  },
  groupInfo: {
    flex: 1,
  },
  groupRight: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  groupName: {
    fontSize: 16,
    fontWeight: '600',
    color: '#333',
  },
  groupMeta: {
    fontSize: 14,
    color: '#666',
    marginTop: 4,
  },
  balanceText: {
    fontSize: 16,
    fontWeight: '700',
  },
  balanceLabel: {
    fontSize: 12,
    color: '#666',
  },
  earnedAmount: {
    color: '#34c759',
  },
  owedAmount: {
    color: '#ff9500',
  },
  empty: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    padding: 32,
  },
  emptyText: {
    fontSize: 18,
    fontWeight: '600',
    color: '#666',
    marginTop: 16,
  },
  emptySubtext: {
    fontSize: 14,
    color: '#999',
    textAlign: 'center',
    marginTop: 8,
  },
  modalOverlay: {
    flex: 1,
    backgroundColor: 'rgba(0,0,0,0.5)',
    justifyContent: 'center',
    padding: 16,
  },
  modalContent: {
    backgroundColor: '#fff',
    borderRadius: 12,
    padding: 24,
  },
  modalTitle: {
    fontSize: 20,
    fontWeight: 'bold',
    marginBottom: 8,
  },
  modalSubtitle: {
    fontSize: 14,
    color: '#666',
    marginBottom: 16,
  },
  input: {
    borderWidth: 1,
    borderColor: '#ddd',
    borderRadius: 8,
    padding: 12,
    fontSize: 16,
    marginBottom: 16,
  },
  modalButtons: {
    flexDirection: 'row',
    gap: 12,
  },
  modalButton: {
    flex: 1,
    padding: 16,
    borderRadius: 8,
    alignItems: 'center',
  },
  cancelButton: {
    backgroundColor: '#f5f5f5',
  },
  cancelButtonText: {
    color: '#666',
    fontWeight: '600',
  },
  joinButton: {
    backgroundColor: '#007AFF',
  },
  joinButtonText: {
    color: '#fff',
    fontWeight: '600',
  },
});
