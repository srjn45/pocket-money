import { useState } from 'react';
import { View, Text, StyleSheet, KeyboardAvoidingView, Platform } from 'react-native';
import { router } from 'expo-router';
import { groupsApi } from '../../../src/api';
import type { CurrencyCode } from '../../../src/api';
import { Button, TextField, ScreenContainer } from '../../../src/components';
import { currencySymbol } from '../../../src/money';
import { theme } from '../../../src/theme';

const CURRENCY_CODES: CurrencyCode[] = ['INR', 'EUR', 'USD'];

export default function CreateGroupScreen() {
  const [name, setName] = useState('');
  const [currency, setCurrency] = useState<CurrencyCode>('INR');
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
      const group = await groupsApi.create({ name: name.trim(), currency });
      router.replace(`/(app)/groups/${group.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create group');
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <KeyboardAvoidingView
      behavior={Platform.OS === 'ios' ? 'padding' : 'height'}
      style={styles.flex}
    >
      <ScreenContainer style={styles.container}>
        <Text style={styles.label}>Group Name</Text>

      {error ? <Text style={styles.error}>{error}</Text> : null}

      <TextField
        placeholder="Enter group name"
        value={name}
        onChangeText={setName}
        autoFocus
        testID="create-group-name"
      />

      <Text style={styles.label}>Currency</Text>
      <View style={styles.currencyRow} testID="create-group-currency">
        {CURRENCY_CODES.map((code) => (
          <Button
            key={code}
            title={`${code} ${currencySymbol(code)}`}
            variant={currency === code ? 'primary' : 'secondary'}
            onPress={() => setCurrency(code)}
            style={styles.currencyOption}
            testID={`create-group-currency-${code}`}
          />
        ))}
      </View>
      <Text style={styles.helper}>
        A group&apos;s currency is permanent — it can&apos;t be changed later.
      </Text>

        <View style={styles.submitRow}>
          <Button title="Create Group" onPress={handleCreate} loading={isLoading} fullWidth testID="create-group-submit" />
        </View>
      </ScreenContainer>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  flex: {
    flex: 1,
    backgroundColor: theme.color.surface,
  },
  container: {
    flex: 1,
    backgroundColor: theme.color.surface,
    padding: theme.spacing.lg,
  },
  label: {
    fontSize: theme.fontSize.md,
    fontWeight: theme.fontWeight.semibold,
    marginBottom: theme.spacing.sm,
    marginTop: theme.spacing.md,
    color: theme.color.text,
  },
  error: {
    color: theme.color.danger,
    marginBottom: theme.spacing.md,
  },
  currencyRow: {
    flexDirection: 'row',
    gap: theme.spacing.sm,
  },
  currencyOption: {
    flex: 1,
  },
  helper: {
    fontSize: theme.fontSize.sm,
    color: theme.color.textSecondary,
    marginTop: theme.spacing.sm,
  },
  submitRow: {
    marginTop: theme.spacing.lg,
  },
});
