import { useState, useEffect, useCallback } from 'react';
import { View, Text, FlatList, TouchableOpacity, StyleSheet, RefreshControl, ActivityIndicator, TextInput, Modal, Alert } from 'react-native';
import { useLocalSearchParams } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { choresApi, groupsApi, Chore, Member } from '../../../../src/api';
import { useAuth } from '../../../../src/auth-context';

export default function ChoresScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { user } = useAuth();
  const [chores, setChores] = useState<Chore[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState('');
  const [isHead, setIsHead] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [choreName, setChoreName] = useState('');
  const [choreDescription, setChoreDescription] = useState('');
  const [choreAmount, setChoreAmount] = useState('');
  const [editingChore, setEditingChore] = useState<Chore | null>(null);
  const [saving, setSaving] = useState(false);

  const loadData = async () => {
    if (!id) return;
    try {
      setError('');
      const [choresData, groupData] = await Promise.all([
        choresApi.list(id),
        groupsApi.get(id),
      ]);
      setChores(choresData || []);
      const currentMember = groupData.members.find((m: Member) => m.user_id === user?.id);
      setIsHead(currentMember?.role === 'head');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load chores');
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

  const openModal = (chore?: Chore) => {
    if (chore) {
      setEditingChore(chore);
      setChoreName(chore.name);
      setChoreDescription(chore.description || '');
      setChoreAmount(chore.amount.toString());
    } else {
      setEditingChore(null);
      setChoreName('');
      setChoreDescription('');
      setChoreAmount('');
    }
    setModalVisible(true);
  };

  const handleSave = async () => {
    if (!id || !choreName.trim() || !choreAmount) return;

    setSaving(true);
    try {
      if (editingChore) {
        await choresApi.update(editingChore.id, {
          name: choreName.trim(),
          description: choreDescription.trim() || undefined,
          amount: parseFloat(choreAmount),
        });
      } else {
        await choresApi.create(id, {
          name: choreName.trim(),
          description: choreDescription.trim() || undefined,
          amount: parseFloat(choreAmount),
        });
      }
      setModalVisible(false);
      loadData();
    } catch (err) {
      Alert.alert('Error', err instanceof Error ? err.message : 'Failed to save chore');
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = (chore: Chore) => {
    Alert.alert('Delete Chore', `Are you sure you want to delete "${chore.name}"?`, [
      { text: 'Cancel', style: 'cancel' },
      {
        text: 'Delete',
        style: 'destructive',
        onPress: async () => {
          try {
            await choresApi.delete(chore.id);
            loadData();
          } catch (err) {
            Alert.alert('Error', err instanceof Error ? err.message : 'Failed to delete chore');
          }
        },
      },
    ]);
  };

  const renderChore = ({ item }: { item: Chore }) => (
    <TouchableOpacity 
      style={styles.choreCard}
      onPress={() => isHead && openModal(item)}
      disabled={!isHead}
    >
      <View style={styles.choreInfo}>
        <Text style={styles.choreName}>{item.name}</Text>
        {item.description && (
          <Text style={styles.choreDescription}>{item.description}</Text>
        )}
      </View>
      <View style={styles.choreRight}>
        <Text style={styles.choreAmount}>${item.amount.toFixed(2)}</Text>
        {isHead && (
          <TouchableOpacity onPress={() => handleDelete(item)}>
            <Ionicons name="trash-outline" size={20} color="#ff3b30" />
          </TouchableOpacity>
        )}
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

      {isHead && (
        <TouchableOpacity style={styles.addButton} onPress={() => openModal()}>
          <Ionicons name="add" size={24} color="#fff" />
          <Text style={styles.addButtonText}>Add Chore</Text>
        </TouchableOpacity>
      )}

      {chores.length === 0 ? (
        <View style={styles.empty}>
          <Ionicons name="list-outline" size={64} color="#ccc" />
          <Text style={styles.emptyText}>No chores yet</Text>
        </View>
      ) : (
        <FlatList
          data={chores}
          renderItem={renderChore}
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
            <Text style={styles.modalTitle}>
              {editingChore ? 'Edit Chore' : 'Add Chore'}
            </Text>

            <TextInput
              style={styles.input}
              placeholder="Chore name"
              value={choreName}
              onChangeText={setChoreName}
            />

            <TextInput
              style={styles.input}
              placeholder="Description (optional)"
              value={choreDescription}
              onChangeText={setChoreDescription}
            />

            <TextInput
              style={styles.input}
              placeholder="Amount"
              value={choreAmount}
              onChangeText={setChoreAmount}
              keyboardType="decimal-pad"
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
                onPress={handleSave}
                disabled={saving}
              >
                {saving ? (
                  <ActivityIndicator color="#fff" />
                ) : (
                  <Text style={styles.saveButtonText}>Save</Text>
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
  choreCard: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: '#fff',
    padding: 16,
    borderRadius: 8,
    marginBottom: 8,
  },
  choreInfo: {
    flex: 1,
  },
  choreName: {
    fontSize: 16,
    fontWeight: '600',
    color: '#333',
  },
  choreDescription: {
    fontSize: 14,
    color: '#666',
    marginTop: 4,
  },
  choreRight: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 12,
  },
  choreAmount: {
    fontSize: 18,
    fontWeight: '600',
    color: '#34c759',
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
