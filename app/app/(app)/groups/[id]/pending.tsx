import { useState, useEffect, useCallback } from 'react';
import { View, Text, FlatList, TouchableOpacity, StyleSheet, RefreshControl, ActivityIndicator, Alert } from 'react-native';
import { useLocalSearchParams } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { ledgerApi, choresApi, groupsApi, LedgerEntry, Chore, Member } from '../../../../src/api';

export default function PendingScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const [entries, setEntries] = useState<LedgerEntry[]>([]);
  const [chores, setChores] = useState<Chore[]>([]);
  const [members, setMembers] = useState<Member[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState('');
  const [processingId, setProcessingId] = useState<string | null>(null);

  const loadData = async () => {
    if (!id) return;
    try {
      setError('');
      const [pendingData, choresData, groupData] = await Promise.all([
        ledgerApi.listPending(id),
        choresApi.list(id),
        groupsApi.get(id),
      ]);
      setEntries(pendingData || []);
      setChores(choresData || []);
      setMembers(groupData.members || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load pending entries');
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

  const handleApprove = async (entry: LedgerEntry) => {
    setProcessingId(entry.id);
    try {
      await ledgerApi.approve(entry.id);
      loadData();
    } catch (err) {
      Alert.alert('Error', err instanceof Error ? err.message : 'Failed to approve');
    } finally {
      setProcessingId(null);
    }
  };

  const handleReject = async (entry: LedgerEntry) => {
    Alert.alert('Reject Entry', 'Are you sure you want to reject this entry?', [
      { text: 'Cancel', style: 'cancel' },
      {
        text: 'Reject',
        style: 'destructive',
        onPress: async () => {
          setProcessingId(entry.id);
          try {
            await ledgerApi.reject(entry.id);
            loadData();
          } catch (err) {
            Alert.alert('Error', err instanceof Error ? err.message : 'Failed to reject');
          } finally {
            setProcessingId(null);
          }
        },
      },
    ]);
  };

  const getChoreByID = (choreId: string) => chores.find(c => c.id === choreId);
  const getMemberByID = (userId: string) => members.find(m => m.user_id === userId);

  const renderEntry = ({ item }: { item: LedgerEntry }) => {
    const chore = getChoreByID(item.chore_id);
    const member = getMemberByID(item.user_id);
    const isProcessing = processingId === item.id;
    
    return (
      <View style={styles.entryCard}>
        <View style={styles.entryInfo}>
          <Text style={styles.entryChore}>{chore?.name || 'Unknown Chore'}</Text>
          <Text style={styles.entryMember}>Submitted by: {member?.name || 'Unknown'}</Text>
          <Text style={styles.entryDate}>
            {new Date(item.created_at).toLocaleDateString()}
          </Text>
        </View>
        <View style={styles.entryRight}>
          <Text style={styles.entryAmount}>${item.amount.toFixed(2)}</Text>
          <View style={styles.actions}>
            <TouchableOpacity 
              style={[styles.actionButton, styles.approveButton]}
              onPress={() => handleApprove(item)}
              disabled={isProcessing}
            >
              {isProcessing ? (
                <ActivityIndicator color="#fff" size="small" />
              ) : (
                <Ionicons name="checkmark" size={20} color="#fff" />
              )}
            </TouchableOpacity>
            <TouchableOpacity 
              style={[styles.actionButton, styles.rejectButton]}
              onPress={() => handleReject(item)}
              disabled={isProcessing}
            >
              <Ionicons name="close" size={20} color="#fff" />
            </TouchableOpacity>
          </View>
        </View>
      </View>
    );
  };

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

      {entries.length === 0 ? (
        <View style={styles.empty}>
          <Ionicons name="checkmark-circle-outline" size={64} color="#34c759" />
          <Text style={styles.emptyText}>No pending entries</Text>
          <Text style={styles.emptySubtext}>All caught up!</Text>
        </View>
      ) : (
        <FlatList
          data={entries}
          renderItem={renderEntry}
          keyExtractor={(item) => item.id}
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
  list: {
    padding: 16,
  },
  entryCard: {
    flexDirection: 'row',
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
  entryInfo: {
    flex: 1,
  },
  entryChore: {
    fontSize: 16,
    fontWeight: '600',
    color: '#333',
  },
  entryMember: {
    fontSize: 14,
    color: '#666',
    marginTop: 4,
  },
  entryDate: {
    fontSize: 12,
    color: '#999',
    marginTop: 4,
  },
  entryRight: {
    alignItems: 'flex-end',
    justifyContent: 'space-between',
  },
  entryAmount: {
    fontSize: 18,
    fontWeight: '600',
    color: '#34c759',
  },
  actions: {
    flexDirection: 'row',
    gap: 8,
    marginTop: 8,
  },
  actionButton: {
    width: 36,
    height: 36,
    borderRadius: 18,
    justifyContent: 'center',
    alignItems: 'center',
  },
  approveButton: {
    backgroundColor: '#34c759',
  },
  rejectButton: {
    backgroundColor: '#ff3b30',
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
    marginTop: 8,
  },
});
