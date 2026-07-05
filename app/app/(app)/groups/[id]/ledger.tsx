import { useState, useEffect, useCallback } from 'react';
import { View, Text, FlatList, TouchableOpacity, StyleSheet, RefreshControl, ActivityIndicator, Modal, Alert, TextInput } from 'react-native';
import { useLocalSearchParams, useRouter, Stack } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { Picker } from '@react-native-picker/picker';
import { ledgerApi, choresApi, groupsApi, LedgerEntry, Chore, Member, Balance } from '../../../../src/api';
import { formatMinorUnits, parseMoneyToMinorUnits } from '../../../../src/money';
import { useAuth } from '../../../../src/auth-context';

export default function LedgerScreen() {
  const { id, member_id, member_name } = useLocalSearchParams<{ id: string; member_id?: string; member_name?: string }>();
  const { user } = useAuth();
  const router = useRouter();
  const [entries, setEntries] = useState<LedgerEntry[]>([]);
  const [chores, setChores] = useState<Chore[]>([]);
  const [members, setMembers] = useState<Member[]>([]);
  const [memberBalance, setMemberBalance] = useState<number | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState('');
  const [isHead, setIsHead] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [selectedChore, setSelectedChore] = useState<string>('');
  const [selectedMember, setSelectedMember] = useState<string>('');
  const [customAmount, setCustomAmount] = useState<string>('');
  const [saving, setSaving] = useState(false);
  const [processingId, setProcessingId] = useState<string | null>(null);

  // Determine which user's ledger to show
  const targetUserId = member_id || user?.id;
  const targetUserName = member_name || user?.name;

  const loadData = async () => {
    if (!id || !user?.id) {
      return;
    }
    try {
      setError('');
      const [entriesData, choresData, groupData, balanceData] = await Promise.all([
        ledgerApi.list(id, { user_id: member_id }),
        choresApi.list(id),
        groupsApi.get(id),
        ledgerApi.getBalance(id),
      ]);
      
      const currentMember = groupData.members.find((m: Member) => m.user_id === user?.id);
      const headStatus = currentMember?.role === 'head';
      setEntries(entriesData || []);
      setChores(choresData || []);
      setMembers(groupData.members || []);
      setIsHead(headStatus);

      // Get balance for the target user
      const userBalance = balanceData?.find((b: Balance) => b.user_id === targetUserId);
      setMemberBalance(userBalance?.balance ?? null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load ledger');
    } finally {
      setIsLoading(false);
      setRefreshing(false);
    }
  };

  useEffect(() => {
    loadData();
  }, [id, member_id, user?.id]);

  const onRefresh = useCallback(() => {
    setRefreshing(true);
    loadData();
  }, [id, member_id]);

  const openModal = () => {
    // Default to first non-system chore, or system chore if head
    const defaultChore = chores.find(c => !c.is_system) || chores[0];
    setSelectedChore(defaultChore?.id || '');
    setSelectedMember(member_id || user?.id || '');
    setCustomAmount('');
    setModalVisible(true);
  };

  const selectedChoreObj = chores.find(c => c.id === selectedChore);
  const isSystemChore = selectedChoreObj?.is_system || false;

  const handleCreate = async () => {
    if (!id || !selectedChore) return;

    const chore = chores.find(c => c.id === selectedChore);
    if (!chore) return;

    // Validate amount for system chores
    if (chore.is_system) {
      const amount = parseMoneyToMinorUnits(customAmount);
      if (amount === null) {
        Alert.alert('Error', 'Please enter a valid amount for settlement (e.g. 12.50)');
        return;
      }
    }

    setSaving(true);
    try {
      const settlementAmount = chore.is_system ? parseMoneyToMinorUnits(customAmount) : undefined;
      await ledgerApi.create(id, {
        user_id: isHead ? selectedMember : undefined,
        chore_id: selectedChore,
        amount: settlementAmount ?? undefined,
      });
      setModalVisible(false);
      loadData();
    } catch (err) {
      Alert.alert('Error', err instanceof Error ? err.message : 'Failed to create entry');
    } finally {
      setSaving(false);
    }
  };

  const handleApprove = async (entryId: string) => {
    setProcessingId(entryId);
    try {
      await ledgerApi.approve(entryId);
      loadData();
    } catch (err) {
      Alert.alert('Error', err instanceof Error ? err.message : 'Failed to approve entry');
    } finally {
      setProcessingId(null);
    }
  };

  const handleReject = async (entryId: string) => {
    Alert.alert('Reject Entry', 'Are you sure you want to reject this entry?', [
      { text: 'Cancel', style: 'cancel' },
      {
        text: 'Reject',
        style: 'destructive',
        onPress: async () => {
          setProcessingId(entryId);
          try {
            await ledgerApi.reject(entryId);
            loadData();
          } catch (err) {
            Alert.alert('Error', err instanceof Error ? err.message : 'Failed to reject entry');
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
    const isRejected = item.status === 'rejected';
    const isPending = item.status === 'pending_approval';
    const isProcessing = processingId === item.id;
    const isSettlement = chore?.is_system;
    
    return (
      <View style={[styles.entryCard, isRejected && styles.rejectedCard]}>
        <View style={styles.entryInfo}>
          <Text style={[styles.entryChore, isRejected && styles.strikethrough]}>
            {isSettlement ? '💰 Settlement' : chore?.name || 'Unknown Chore'}
          </Text>
          {!member_id && (
            <Text style={[styles.entryMember, isRejected && styles.strikethrough]}>
              {member?.name || 'Unknown'}
            </Text>
          )}
          <Text style={styles.entryDate}>
            {new Date(item.created_at).toLocaleDateString()}
          </Text>
        </View>
        <View style={styles.entryRight}>
          <Text style={[
            styles.entryAmount,
            isSettlement ? styles.settlementAmount : styles.earnedAmount,
            isRejected && styles.strikethrough
          ]}>
            {isSettlement ? '-' : '+'}{formatMinorUnits(item.amount)}
          </Text>
          
          {isPending && isHead ? (
            <View style={styles.actions}>
              <TouchableOpacity 
                style={[styles.actionButton, styles.approveButton]}
                onPress={() => handleApprove(item.id)}
                disabled={isProcessing}
              >
                {isProcessing ? (
                  <ActivityIndicator color="#fff" size="small" />
                ) : (
                  <Ionicons name="checkmark" size={18} color="#fff" />
                )}
              </TouchableOpacity>
              <TouchableOpacity 
                style={[styles.actionButton, styles.rejectButton]}
                onPress={() => handleReject(item.id)}
                disabled={isProcessing}
              >
                <Ionicons name="close" size={18} color="#fff" />
              </TouchableOpacity>
            </View>
          ) : (
            <View style={[styles.statusBadge, getStatusStyle(item.status)]}>
              <Text style={styles.statusText}>
                {item.status === 'pending_approval' ? 'pending' : item.status}
              </Text>
            </View>
          )}
        </View>
      </View>
    );
  };

  const getStatusStyle = (status: LedgerEntry['status']) => {
    switch (status) {
      case 'approved': return styles.approved;
      case 'pending_approval': return styles.pending;
      case 'rejected': return styles.rejected;
    }
  };

  // Filter chores for the modal
  const availableChores = isHead 
    ? chores  // Head can see all chores including Settlement
    : chores.filter(c => !c.is_system);  // Members can only see regular chores

  if (isLoading) {
    return (
      <View style={styles.centered}>
        <ActivityIndicator size="large" color="#007AFF" />
      </View>
    );
  }

  return (
    <>
      {member_id && (
        <Stack.Screen options={{ title: `${member_name}'s Ledger` }} />
      )}
      <View style={styles.container}>
        {error ? <Text style={styles.error}>{error}</Text> : null}

        {/* Balance Header */}
        {memberBalance !== null && (
          <View style={styles.balanceHeader}>
            <Text style={styles.balanceLabel}>
              {member_id ? `${member_name}'s Balance` : 'Your Balance'}
            </Text>
            <Text style={[styles.balanceAmount, memberBalance >= 0 ? styles.earnedAmount : styles.settlementAmount]}>
              {formatMinorUnits(Math.abs(memberBalance))}
            </Text>
            {memberBalance > 0 && <Text style={styles.balanceNote}>owed to you</Text>}
            {memberBalance < 0 && <Text style={styles.balanceNote}>overpaid</Text>}
          </View>
        )}

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

              {availableChores.length === 0 ? (
                <Text style={styles.noChores}>No chores available. Create chores first.</Text>
              ) : (
                <>
                  <Text style={styles.label}>Select Chore</Text>
                  <View style={styles.pickerContainer}>
                    <Picker
                      selectedValue={selectedChore}
                      onValueChange={setSelectedChore}
                    >
                      {availableChores.map(chore => (
                        <Picker.Item 
                          key={chore.id} 
                          label={chore.is_system ? `${chore.name} (Custom amount)` : `${chore.name} - ${formatMinorUnits(chore.amount)}`}
                          value={chore.id} 
                        />
                      ))}
                    </Picker>
                  </View>

                  {isSystemChore && (
                    <>
                      <Text style={styles.label}>Settlement Amount</Text>
                      <TextInput
                        style={styles.input}
                        value={customAmount}
                        onChangeText={setCustomAmount}
                        keyboardType="decimal-pad"
                        placeholder="Enter amount"
                        placeholderTextColor="#999"
                      />
                    </>
                  )}

                  {isHead && !member_id && (
                    <>
                      <Text style={styles.label}>Select Member</Text>
                      <View style={styles.pickerContainer}>
                        <Picker
                          selectedValue={selectedMember}
                          onValueChange={setSelectedMember}
                        >
                          {members.filter(m => m.role !== 'head').map(member => (
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
                  disabled={saving || availableChores.length === 0}
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
    </>
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
  balanceHeader: {
    backgroundColor: '#fff',
    padding: 20,
    alignItems: 'center',
    borderBottomWidth: 1,
    borderBottomColor: '#eee',
  },
  balanceLabel: {
    fontSize: 14,
    color: '#666',
    marginBottom: 4,
  },
  balanceAmount: {
    fontSize: 36,
    fontWeight: '700',
  },
  balanceNote: {
    fontSize: 14,
    color: '#666',
    marginTop: 4,
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
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 1 },
    shadowOpacity: 0.1,
    shadowRadius: 2,
    elevation: 1,
  },
  rejectedCard: {
    backgroundColor: '#fafafa',
    opacity: 0.7,
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
  strikethrough: {
    textDecorationLine: 'line-through',
    color: '#999',
  },
  entryRight: {
    alignItems: 'flex-end',
    justifyContent: 'space-between',
  },
  entryAmount: {
    fontSize: 18,
    fontWeight: '600',
  },
  earnedAmount: {
    color: '#34c759',
  },
  settlementAmount: {
    color: '#ff3b30',
  },
  actions: {
    flexDirection: 'row',
    gap: 8,
    marginTop: 8,
  },
  actionButton: {
    width: 32,
    height: 32,
    borderRadius: 16,
    justifyContent: 'center',
    alignItems: 'center',
  },
  approveButton: {
    backgroundColor: '#34c759',
  },
  rejectButton: {
    backgroundColor: '#ff3b30',
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
  input: {
    borderWidth: 1,
    borderColor: '#ddd',
    borderRadius: 8,
    padding: 12,
    fontSize: 16,
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
