import { useState } from 'react';
import { View, Text, StyleSheet } from 'react-native';
import { router } from 'expo-router';
import { groupsApi } from '../../../src/api';
import { Button, TextField } from '../../../src/components';
import { theme } from '../../../src/theme';

export default function CreateGroupScreen() {
  const [name, setName] = useState('');
  const [error, setError] = useState('');
  const [isLoading, setIsLoading] = useState(false);

  const handleCreate = async () => {
    if (!name.trim()) {
      setError('Please enter a group name');
      return;
    }

    setError('');
    setIsLoading(true);

    try {
      const group = await groupsApi.create({ name: name.trim() });
      router.replace(`/(app)/groups/${group.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create group');
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <View style={styles.container}>
      <Text style={styles.label}>Group Name</Text>

      {error ? <Text style={styles.error}>{error}</Text> : null}

      <TextField
        placeholder="Enter group name"
        value={name}
        onChangeText={setName}
        autoFocus
        testID="create-group-name"
      />

      <Button title="Create Group" onPress={handleCreate} loading={isLoading} fullWidth testID="create-group-submit" />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: theme.color.surface,
    padding: theme.spacing.lg,
  },
  label: {
    fontSize: 16,
    fontWeight: '600',
    marginBottom: theme.spacing.sm,
    color: theme.color.text,
  },
  error: {
    color: theme.color.danger,
    marginBottom: theme.spacing.md,
  },
});
