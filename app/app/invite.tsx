import { useState, useEffect } from 'react';
import { View, Text, ActivityIndicator, StyleSheet } from 'react-native';
import { useLocalSearchParams, router } from 'expo-router';
import { groupsApi } from '../src/api';
import { useAuth } from '../src/auth-context';
import { setPendingInviteToken } from '../src/storage';

export default function InviteScreen() {
  const { token } = useLocalSearchParams<{ token: string }>();
  const { user, isLoading: authLoading } = useAuth();
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    const handleInvite = async () => {
      if (authLoading) return;

      if (!token) {
        setError('Invalid invite link');
        setIsLoading(false);
        return;
      }

      if (!user) {
        // Not logged in, save token and redirect to login
        await setPendingInviteToken(token);
        router.replace('/(auth)/login');
        return;
      }

      // User is logged in, try to join
      try {
        const group = await groupsApi.join(token);
        router.replace(`/(app)/groups/${group.id}`);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to join group');
        setIsLoading(false);
      }
    };

    handleInvite();
  }, [token, user, authLoading]);

  if (isLoading && !error) {
    return (
      <View style={styles.container}>
        <ActivityIndicator size="large" color="#007AFF" />
        <Text style={styles.text}>Joining group...</Text>
      </View>
    );
  }

  if (error) {
    return (
      <View style={styles.container}>
        <Text style={styles.error}>{error}</Text>
      </View>
    );
  }

  return null;
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: '#fff',
    padding: 24,
  },
  text: {
    marginTop: 16,
    fontSize: 16,
    color: '#666',
  },
  error: {
    fontSize: 16,
    color: '#ff3b30',
    textAlign: 'center',
  },
});
