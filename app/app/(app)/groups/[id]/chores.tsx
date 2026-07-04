import { useState } from 'react';
import { View, Text, FlatList, TouchableOpacity, StyleSheet, RefreshControl, ActivityIndicator, TextInput, Modal, Alert, Platform } from 'react-native';
import { useLocalSearchParams } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { Chore } from '../../../../src/api';
import { useAuth } from '../../../../src/auth-context';
import { useChores, useCreateChore, useUpdateChore, useDeleteChore } from '../../../../src/hooks/useChores';
import { useGroup } from '../../../../src/hooks/useGroup';

export default function ChoresScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { user } = useAuth();
  const [modalVisible, setModalVisible] = useState(false);
  const [choreName, setChoreName] = useState('');
  const [choreDescription, setChoreDescription] = useState('');
  const [choreAmount, setChoreAmount] = useState('');
  const [editingChore, setEditingChore] = useState<Chore | null>(null);
  const [formError, setFormError] = useState('');

  const choresQuery = useChores(id ?? '');
  const groupQuery = useGroup(id ?? '');

  const chores = choresQuery.data ?? [];
  const isHead = groupQuery.data?.members.find(m => m.user_id === user?.id)?.role === 'head';

  const isLoading = choresQuery.isLoading || groupQuery.isLoading;
  const isRefetching = choresQuery.isRefetching || groupQuery.isRefetching;
  const error = choresQuery.error instanceof Error ? choresQuery.error.message
    : groupQuery.error instanceof Error ? groupQuery.error.message : '';

  const onRefresh = () => {
    choresQuery.refetch();
    groupQuery.refetch();
  };

  const createChoreMutation = useCreateChore(id ?? '');
  const updateChoreMutation = useUpdateChore(id ?? '');
  const deleteChoreMutation = useDeleteChore(id ?? '');

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
    setFormError('');
    setModalVisible(true);
  };

  const isFormValid = () => {
    const parsed = parseFloat(choreAmount);
    return choreName.trim().length > 0 && choreAmount.trim().length > 0 && !isNaN(parsed) && parsed > 0;
  };

  const saving = createChoreMutation.isPending || updateChoreMutation.isPending;

  const handleSave = async () => {
    if (!id) return;
    const trimmedName = choreName.trim();
    const parsedAmount = parseFloat(choreAmount);

    if (!trimmedName) {
      setFormError('Chore name is required.');
      return;
    }
    if (!choreAmount.trim() || isNaN(parsedAmount) || parsedAmount <= 0) {
      setFormError('Amount must be a number greater than 0.');
      return;
    }

    setFormError('');
    try {
      if (editingChore) {
        await updateChoreMutation.mutateAsync({
          id: editingChore.id,
          name: trimmedName,
          description: choreDescription.trim() || undefined,
          amount: parsedAmount,
        });
      } else {
        await createChoreMutation.mutateAsync({
          name: trimmedName,
          description: choreDescription.trim() || undefined,
          amount: parsedAmount,
        });
      }
      setModalVisible(false);
    } catch (err) {
      setFormError(err instanceof Error ? err.message : 'Failed to save chore');
    }
  };

  const handleDelete = async (chore: Chore) => {
    const msg = `Delete "${chore.name}"? This can't be undone.`;
    let confirmed: boolean;
    if (Platform.OS === 'web') {
      confirmed = (globalThis as unknown as { confirm: (m: string) => boolean }).confirm(msg);
    } else {
      confirmed = await new Promise<boolean>((resolve) => {
        Alert.alert('Delete Chore', msg, [
          { text: 'Cancel', style: 'cancel', onPress: () => resolve(false) },
          { text: 'Delete', style: 'destructive', onPress: () => resolve(true) },
        ]);
      });
    }
    if (!confirmed) return;
    try {
      await deleteChoreMutation.mutateAsync(chore.id);
    } catch (err) {
      Alert.alert('Error', err instanceof Error ? err.message : 'Failed to delete chore');
    }
  };

  const renderChore = ({ item }: { item: Chore }) => {
    const isSystemChore = item.is_system;
    const canEdit = isHead && !isSystemChore;

    return (
      <TouchableOpacity
        style={[styles.choreCard, isSystemChore && styles.systemChoreCard]}
        onPress={() => canEdit && openModal(item)}
        disabled={!canEdit}
      >
        <View style={styles.choreInfo}>
          <View style={styles.choreHeader}>
            <Text style={styles.choreName}>{item.name}</Text>
            {isSystemChore && (
              <View style={styles.systemBadge}>
                <Text style={styles.systemBadgeText}>System</Text>
              </View>
            )}
          </View>
          {isSystemChore && (
            <Text style={styles.systemChoreCaption}>Used to record payouts — can't be edited or deleted</Text>
          )}
          {!isSystemChore && item.description && (
            <Text style={styles.choreDescription}>{item.description}</Text>
          )}
        </View>
        <View style={styles.choreRight}>
          {isSystemChore ? (
            <Text style={styles.choreAmountVariable}>Variable</Text>
          ) : (
            <Text style={styles.choreAmount}>₹{item.amount.toFixed(2)}</Text>
          )}
          {canEdit && (
            <TouchableOpacity onPress={() => handleDelete(item)}>
              <Ionicons name="trash-outline" size={20} color="#ff3b30" />
            </TouchableOpacity>
          )}
        </View>
      </TouchableOpacity>
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
        <TouchableOpacity style={styles.addButton} onPress={() => openModal()}>
          <Ionicons name="add" size={24} color="#fff" />
          <Text style={styles.addButtonText}>Add Chore</Text>
        </TouchableOpacity>
      )}

      {chores.length === 0 ? (
        <View style={styles.empty}>
          <Ionicons name="list-outline" size={64} color="#ccc" />
          <Text style={styles.emptyText}>No chores yet</Text>
          {isHead && <Text style={styles.emptyHint}>Add chores your family can earn money for</Text>}
        </View>
      ) : (
        <FlatList
          data={chores}
          renderItem={renderChore}
          keyExtractor={(item) => item.id}
          refreshControl={
            <RefreshControl refreshing={isRefetching} onRefresh={onRefresh} />
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

            {formError ? <Text style={styles.formError}>{formError}</Text> : null}

            <TextInput
              style={styles.input}
              placeholder="Chore name"
              value={choreName}
              onChangeText={(v) => { setChoreName(v); setFormError(''); }}
            />

            <TextInput
              style={styles.input}
              placeholder="Description (optional)"
              value={choreDescription}
              onChangeText={setChoreDescription}
            />

            <TextInput
              style={styles.input}
              placeholder="Amount (₹)"
              value={choreAmount}
              onChangeText={(v) => { setChoreAmount(v); setFormError(''); }}
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
                style={[styles.modalButton, styles.saveButton, (!isFormValid() || saving) && styles.saveButtonDisabled]}
                onPress={handleSave}
                disabled={!isFormValid() || saving}
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
  formError: {
    color: '#ff3b30',
    fontSize: 14,
    marginBottom: 12,
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
  systemChoreCard: {
    backgroundColor: '#f8f9fa',
    borderWidth: 1,
    borderColor: '#e9ecef',
  },
  choreInfo: {
    flex: 1,
  },
  choreHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  choreName: {
    fontSize: 16,
    fontWeight: '600',
    color: '#333',
  },
  systemBadge: {
    backgroundColor: '#6c757d',
    paddingHorizontal: 8,
    paddingVertical: 2,
    borderRadius: 4,
  },
  systemBadgeText: {
    color: '#fff',
    fontSize: 10,
    fontWeight: '600',
    textTransform: 'uppercase',
  },
  systemChoreCaption: {
    fontSize: 12,
    color: '#6c757d',
    marginTop: 4,
    fontStyle: 'italic',
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
  choreAmountVariable: {
    fontSize: 14,
    fontWeight: '500',
    color: '#6c757d',
    fontStyle: 'italic',
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
  emptyHint: {
    fontSize: 14,
    color: '#999',
    marginTop: 8,
    textAlign: 'center',
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
  saveButtonDisabled: {
    opacity: 0.5,
  },
  saveButtonText: {
    color: '#fff',
    fontWeight: '600',
  },
});
