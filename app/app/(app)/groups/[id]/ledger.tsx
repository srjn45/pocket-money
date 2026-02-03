import { useState, useEffect, useCallback } from 'react';
import { View, Text, FlatList, TouchableOpacity, StyleSheet, RefreshControl, ActivityIndicator, Modal, Alert } from 'react-native';
import { useLocalSearchParams } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { Picker } from '@react-native-picker/picker';
import { ledgerApi, choresApi, groupsApi, LedgerEntry, Chore, Member } from '../../../../src/api';
import { useAuth } from '../../../../src/auth-context';

export default function LedgerScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { user } = useAuth();
  const [entries, setEntries] = useState<LedgerEntry[]>([]);
  const [chores, setChores] = useState<Chore[]>([]);
  const [members, setMembers] = useState<Member[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState('');
  const [isHead, setIsHead] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [selectedChore, setSelectedChore] = useState<string>('');
  const [selectedMember, setSelectedMember] = useState<string>('');
  const [saving, setSaving] = useState(false);

  const loadData = async () => {
    if (!id) return;
    try {
      setError('');
      const [entriesData, choresData, groupData] = await Promise.all([
        ledgerApi.list(id),
        choresApi.list(id),
        groupsApi.get(id),
      ]);
      setEntries(entriesData || []);
      setChores(choresData || []);
      setMembers(groupData.members || []);
      const currentMember = groupData.members.find((m: Member) => m.user_id === user?.id);
      setIsHead(currentMember?.role === 'head');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load ledger');
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

  const openModal = () => {
    setSelectedChore(chores[0]?.id || '');
    setSelectedMember(isHead ? members[0]?.user_id || '' : user?.id || '');
    setModalVisible(true);
  };

  const handleCreate = async () => {
    if (!id || !selectedChore) return;

    const chore = chores.find(c => c.id === selectedChore);
    if (!chore) return;

    setSaving(true);
    try {
      await ledgerApi.create(id, {
        user_id: isHead ? selectedMember : undefined,
        chore_id: selectedChore,
        amount: chore.amount,
      });
      setModalVisible(false);
      loadData();
    } catch (err) {
      Alert.alert('Error', err instanceof Error ? err.message : 'Failed to create entry');
    } finally {
      setSaving(false);
    }
  };

  const getStatusStyle = (status: LedgerEntry['status']) => {
    switch (status) {
      case 'approved': return styles.approved;
      case 'pending_approval': return styles.pending;
      case 'rejected': return styles.rejected;
    }
  };

  const getChoreByID = (choreId: string) => chores.find(c => c.id === choreId);
  const getMemberByID = (userId: string) => members.find(m => m.user_id === userId);

  const renderEntry = ({ item }: { item: LedgerEntry }) => {
    const chore = getChoreByID(item.chore_id);
    const member = getMemberByID(item.user_id);
    
    return (
      <View style={styles.entryCard}>
        <View style={styles.entryInfo}>
          <Text style={styles.entryChore}>{chore?.name || 'Unknown Chore'}</Text>
          <Text style={styles.entryMember}>{member?.name || 'Unknown'}</Text>
          <Text style={styles.entryDate}>
            {new Date(item.created_at).toLocaleDateString()}
          </Text>
        </View>
        <View style={styles.entryRight}>
          <Text style={styles.entryAmount}>${item.amount.toFixed(2)}</Text>
          <View style={[styles.statusBadge, getStatusStyle(item.status)]}>
            <Text style={styles.statusText}>{item.status.replace('_', ' ')}</Text>
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

      <TouchableOpacity style={styles.addButton} onPress={openModal}>
        <Ionicons name="add" size={24} color="#fff" />
        <Text style={styles.addButtonText}>Add Entry</Text>
      </TouchableOpacity>

      {entries.length === 0 ? (
        <View style={styles.empty}>
          <Ionicons name="wallet-outline" size={64} color="#ccc" />
          <Text style={styles.emptyText}>No ledger entries yet</Text>
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

      <Modal visible={modalVisible} animationType="slide" transparent>
        <View style={styles.modalOverlay}>
          <View style={styles.modalContent}>
            <Text style={styles.modalTitle}>Add Entry</Text>

            {chores.length === 0 ? (
              <Text style={styles.noChores}>No chores available. Create chores first.</Text>
            ) : (
              <>
                <Text style={styles.label}>Select Chore</Text>
                <View style={styles.pickerContainer}>
                  <Picker
                    selectedValue={selectedChore}
                    onValueChange={setSelectedChore}
                  >
                    {chores.map(chore => (
                      <Picker.Item 
                        key={chore.id} 
                        label={`${chore.name} - $${chore.amount}`} 
                        value={chore.id} 
                      />
                    ))}
                  </Picker>
                </View>

                {isHead && (
                  <>
                    <Text style={styles.label}>Select Member</Text>
                    <View style={styles.pickerContainer}>
                      <Picker
                        selectedValue={selectedMember}
                        onValueChange={setSelectedMember}
                      >
                        {members.map(member => (
                          <Picker.Item 
                            key={member.user_id} 
                            label={member.name} 
                            value={member.user_id} 
                          />
                        ))}
                      </Picker>
                    </View>
                  </>
                )}

                {!isHead && (
                  <Text style={styles.noteText}>
                    Your entry will be submitted for approval
                  </Text>
                )}
              </>
            )}

            <View style={styles.modalButtons}>
              <TouchableOpacity
                style={[styles.modalButton, styles.cancelButton]}
                onPress={() => setModalVisible(false)}
              >
                <Text style={styles.cancelButtonText}>Cancel</Text>
              </TouchableOpacity>
              <TouchableOpacity
                style={[styles.modalButton, styles.saveButton]}
                onPress={handleCreate}
                disabled={saving || chores.length === 0}
              >
                {saving ? (
                  <ActivityIndicator color="#fff" />
                ) : (
                  <Text style={styles.saveButtonText}>Add</Text>
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
  error: {
    color: '#ff3b30',
    padding: 16,
    textAlign: 'center',
  },
  addButton: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: '#007AFF',
    margin: 16,
    padding: 16,
    borderRadius: 8,
    gap: 8,
  },
  addButtonText: {
    color: '#fff',
    fontSize: 16,
    fontWeight: '600',
  },
  list: {
    padding: 16,
    paddingTop: 0,
  },
  entryCard: {
    flexDirection: 'row',
    backgroundColor: '#fff',
    padding: 16,
    borderRadius: 8,
    marginBottom: 8,
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
    marginTop: 2,
  },
  entryDate: {
    fontSize: 12,
    color: '#999',
    marginTop: 4,
  },
  entryRight: {
    alignItems: 'flex-end',
  },
  entryAmount: {
    fontSize: 18,
    fontWeight: '600',
    color: '#34c759',
  },
  statusBadge: {
    paddingHorizontal: 8,
    paddingVertical: 4,
    borderRadius: 4,
    marginTop: 4,
  },
  statusText: {
    fontSize: 12,
    fontWeight: '500',
    textTransform: 'capitalize',
  },
  approved: {
    backgroundColor: '#d4f5dc',
  },
  pending: {
    backgroundColor: '#fff3cd',
  },
  rejected: {
    backgroundColor: '#f8d7da',
  },
  empty: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    padding: 32,
  },
  emptyText: {
    fontSize: 18,
    color: '#666',
    marginTop: 16,
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
    marginBottom: 16,
  },
  label: {
    fontSize: 14,
    fontWeight: '600',
    color: '#666',
    marginBottom: 8,
  },
  pickerContainer: {
    borderWidth: 1,
    borderColor: '#ddd',
    borderRadius: 8,
    marginBottom: 16,
  },
  noChores: {
    color: '#666',
    textAlign: 'center',
    padding: 16,
  },
  noteText: {
    color: '#ff9500',
    fontSize: 14,
    textAlign: 'center',
    marginBottom: 16,
  },
  modalButtons: {
    flexDirection: 'row',
    gap: 12,
    marginTop: 8,
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
  saveButton: {
    backgroundColor: '#007AFF',
  },
  saveButtonText: {
    color: '#fff',
    fontWeight: '600',
  },
});
