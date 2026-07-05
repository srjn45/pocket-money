import { useState } from 'react';
import { FlatList, RefreshControl, StyleSheet, Text, View } from 'react-native';
import { useLocalSearchParams } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { Chore } from '../../../../src/api';
import { useAuth } from '../../../../src/auth-context';
import { useChores, useCreateChore, useUpdateChore, useDeleteChore } from '../../../../src/hooks/useChores';
import { useGroup } from '../../../../src/hooks/useGroup';
import {
  Button,
  ListRow,
  AmountText,
  StatusBadge,
  Sheet,
  TextField,
  EmptyState,
  LoadingSpinner,
  useToast,
} from '../../../../src/components';
import { parseMoneyToMinorUnits } from '../../../../src/money';
import { confirmAsync } from '../../../../src/confirm';
import { theme } from '../../../../src/theme';

export default function ChoresScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { user } = useAuth();
  const toast = useToast();

  const [modalVisible, setModalVisible] = useState(false);
  const [choreName, setChoreName] = useState('');
  const [choreDescription, setChoreDescription] = useState('');
  const [choreAmount, setChoreAmount] = useState('');
  const [editingChore, setEditingChore] = useState<Chore | null>(null);
  const [nameError, setNameError] = useState('');
  const [amountError, setAmountError] = useState('');

  const choresQuery = useChores(id ?? '');
  const groupQuery = useGroup(id ?? '');

  const chores = choresQuery.data ?? [];
  const isHead = groupQuery.data?.members.find(m => m.user_id === user?.id)?.role === 'head';

  const isLoading = choresQuery.isLoading || groupQuery.isLoading;
  const isRefetching = choresQuery.isRefetching || groupQuery.isRefetching;
  const loadError = choresQuery.error instanceof Error ? choresQuery.error.message
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
      setChoreAmount((chore.amount / 100).toFixed(2));
    } else {
      setEditingChore(null);
      setChoreName('');
      setChoreDescription('');
      setChoreAmount('');
    }
    setNameError('');
    setAmountError('');
    setModalVisible(true);
  };

  const closeModal = () => setModalVisible(false);

  const isFormValid = () => {
    return choreName.trim().length > 0 && parseMoneyToMinorUnits(choreAmount) !== null;
  };

  const saving = createChoreMutation.isPending || updateChoreMutation.isPending;

  const handleSave = async () => {
    if (!id) return;
    const trimmedName = choreName.trim();
    const parsedAmount = parseMoneyToMinorUnits(choreAmount);

    setNameError('');
    setAmountError('');

    if (!trimmedName) {
      setNameError('Chore name is required.');
      return;
    }
    if (parsedAmount === null) {
      setAmountError('Enter a valid amount (e.g. 12.50).');
      return;
    }

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
      closeModal();
    } catch (err) {
      setAmountError(err instanceof Error ? err.message : 'Failed to save chore');
    }
  };

  const handleDelete = async (chore: Chore) => {
    const ok = await confirmAsync({
      title: 'Delete Chore',
      message: `Delete "${chore.name}"? This can't be undone.`,
      confirmLabel: 'Delete',
      destructive: true,
    });
    if (!ok) return;
    try {
      await deleteChoreMutation.mutateAsync(chore.id);
    } catch (err) {
      const errMsg = err instanceof Error ? err.message : 'Failed to delete chore';
      toast.show({ tone: 'danger', message: errMsg });
    }
  };

  const renderChore = ({ item }: { item: Chore }) => {
    const isSystemChore = item.is_system;
    const canEdit = isHead && !isSystemChore;

    const leftSlot = isSystemChore
      ? <StatusBadge label="System" tone="neutral" />
      : <Ionicons name="checkbox-outline" size={22} color={theme.color.primary} />;

    const rightSlot = isSystemChore
      ? <Text style={styles.variableText}>Variable</Text>
      : (
        <View style={styles.rightSlot}>
          <AmountText minorUnits={item.amount} variant="neutral" />
          {canEdit && (
            <Button
              variant="ghost"
              title=""
              size="sm"
              icon="trash-outline"
              onPress={() => handleDelete(item)}
            />
          )}
        </View>
      );

    return (
      <ListRow
        title={item.name}
        subtitle={
          isSystemChore
            ? 'Used to record payouts — can\'t be edited or deleted'
            : item.description ?? undefined
        }
        left={leftSlot}
        right={rightSlot}
        onPress={canEdit ? () => openModal(item) : undefined}
        style={isSystemChore ? styles.systemRow : undefined}
      />
    );
  };

  if (isLoading) {
    return <LoadingSpinner />;
  }

  const sheetFooter = (
    <>
      <Button
        variant="ghost"
        title="Cancel"
        onPress={closeModal}
        style={styles.footerBtn}
      />
      <Button
        variant="primary"
        title="Save"
        onPress={handleSave}
        loading={saving}
        disabled={!isFormValid()}
        style={styles.footerBtnPrimary}
      />
    </>
  );

  return (
    <View style={styles.container}>
      {loadError ? (
        <Text style={styles.loadError}>{loadError}</Text>
      ) : null}

      {isHead && (
        <View style={styles.addButtonWrapper}>
          <Button
            title="Add Chore"
            icon="add"
            variant="primary"
            fullWidth
            onPress={() => openModal()}
          />
        </View>
      )}

      {chores.length === 0 ? (
        <EmptyState
          icon="list-outline"
          title="No chores yet"
          subtitle={isHead ? 'Add chores your family can earn money for' : undefined}
        />
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

      <Sheet
        visible={modalVisible}
        onClose={closeModal}
        title={editingChore ? 'Edit Chore' : 'Add Chore'}
        footer={sheetFooter}
      >
        <TextField
          label="Chore name"
          placeholder="e.g. Wash dishes"
          value={choreName}
          onChangeText={(v) => { setChoreName(v); setNameError(''); }}
          error={nameError || undefined}
        />
        <TextField
          label="Description (optional)"
          placeholder="What needs to be done"
          value={choreDescription}
          onChangeText={setChoreDescription}
        />
        <TextField
          label="Amount (₹)"
          placeholder="e.g. 25"
          value={choreAmount}
          onChangeText={(v) => { setChoreAmount(v); setAmountError(''); }}
          keyboardType="decimal-pad"
          error={amountError || undefined}
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
  loadError: {
    color: theme.color.danger,
    padding: theme.spacing.lg,
    textAlign: 'center',
    fontSize: theme.fontSize.sm,
  },
  addButtonWrapper: {
    margin: theme.spacing.lg,
  },
  list: {
    paddingHorizontal: theme.spacing.lg,
    paddingBottom: theme.spacing.lg,
  },
  systemRow: {
    backgroundColor: theme.color.surfaceMuted,
  },
  rightSlot: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: theme.spacing.sm,
  },
  variableText: {
    fontSize: theme.fontSize.sm,
    color: theme.color.textSecondary,
    fontStyle: 'italic',
    fontWeight: theme.fontWeight.medium,
  },
  footerBtn: {
    flex: 1,
  },
  footerBtnPrimary: {
    flex: 1,
  },
});
