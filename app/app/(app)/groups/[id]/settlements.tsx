import { useState, useEffect, useCallback } from 'react';
import { View, Text, FlatList, TouchableOpacity, StyleSheet, RefreshControl, ActivityIndicator, TextInput, Modal, Alert } from 'react-native';
import { useLocalSearchParams } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { Picker } from '@react-native-picker/picker';
import { settlementsApi, groupsApi, Settlement, Member } from '../../../../src/api';
import { useAuth } from '../../../../src/auth-context';

export default function SettlementsScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { user } = useAuth();
  const [settlements, setSettlements] = useState<Settlement[]>([]);
  const [members, setMembers] = useState<Member[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState('');
  const [isHead, setIsHead] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [selectedMember, setSelectedMember] = useState<string>('');
  const [amount, setAmount] = useState('');
  const [note, setNote] = useState('');
  const [saving, setSaving] = useState(false);

  const loadData = async () => {
    if (!id) return;
    try {
      setError('');
      const [settlementsData, groupData] = await Promise.all([
        settlementsApi.list(id),
        groupsApi.get(id),
      ]);
      setSettlements(settlementsData || []);
      setMembers(groupData.members || []);
      const currentMember = groupData.members.find((m: Member) => m.user_id === user?.id);
      setIsHead(currentMember?.role === 'head');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load settlements');
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
    setSelectedMember(members[0]?.user_id || '');
    setAmount('');
    setNote('');
    setModalVisible(true);
  };

  const handleCreate = async () => {
    if (!id || !selectedMember || !amount) return;

    const amountNum = parseFloat(amount);
    if (isNaN(amountNum) || amountNum <= 0) {
      Alert.alert('Error', 'Please enter a valid amount');
      return;
    }

    setSaving(true);
    try {
      const today = new Date().toISOString().split('T')[0];
      await settlementsApi.create(id, {
        user_id: selectedMember,
        amount: amountNum,
        date: today,
        note: note.trim() || undefined,
      });
      setModalVisible(false);
      loadData();
    } catch (err) {
      Alert.alert('Error', err instanceof Error ? err.message : 'Failed to create settlement');
    } finally {
      setSaving(false);
    }
  };

  const getMemberByID = (userId: string) => members.find(m => m.user_id === userId);

  const renderSettlement = ({ item }: { item: Settlement }) => {
    const member = getMemberByID(item.user_id);
    
    return (
      <View style={styles.settlementCard}>
        <View style={styles.settlementInfo}>
          <Text style={styles.memberName}>{member?.name || 'Unknown'}</Text>
          <Text style={styles.date}>
            {new Date(item.date).toLocaleDateString()}
          </Text>
          {item.note && <Text style={styles.note}>{item.note}</Text>}
        </View>
        <Text style={styles.amount}>${item.amount.toFixed(2)}</Text>
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

      {isHead && (
        <TouchableOpacity style={styles.addButton} onPress={openModal}>
          <Ionicons name="add" size={24} color="#fff" />
          <Text style={styles.addButtonText}>Add Settlement</Text>
        </TouchableOpacity>
      )}

      {settlements.length === 0 ? (
        <View style={styles.empty}>
          <Ionicons name="cash-outline" size={64} color="#ccc" />
          <Text style={styles.emptyText}>No settlements yet</Text>
        </View>
      ) : (
        <FlatList
          data={settlements}
          renderItem={renderSettlement}
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
            <Text style={styles.modalTitle}>Add Settlement</Text>

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

            <TextInput
              style={styles.input}
              placeholder="Amount"
              value={amount}
              onChangeText={setAmount}
              keyboardType="decimal-pad"
            />

            <TextInput
              style={styles.input}
              placeholder="Note (optional)"
              value={note}
              onChangeText={setNote}
            />

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
                disabled={saving}
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
  settlementCard: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: '#fff',
    padding: 16,
    borderRadius: 8,
    marginBottom: 8,
  },
  settlementInfo: {
    flex: 1,
  },
  memberName: {
    fontSize: 16,
    fontWeight: '600',
    color: '#333',
  },
  date: {
    fontSize: 14,
    color: '#666',
    marginTop: 2,
  },
  note: {
    fontSize: 14,
    color: '#999',
    marginTop: 4,
    fontStyle: 'italic',
  },
  amount: {
    fontSize: 18,
    fontWeight: '600',
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
    marginBottom: 12,
    fontSize: 16,
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
