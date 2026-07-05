import { useState, useEffect } from 'react';
import { useLocalSearchParams, router } from 'expo-router';
import { useAuth } from '../src/auth-context';
import { setPendingInviteToken } from '../src/storage';
import { useJoinGroup } from '../src/hooks/useGroups';
import { LoadingSpinner, ErrorMessage } from '../src/components';

export default function InviteScreen() {
  const { token } = useLocalSearchParams<{ token: string }>();
  const { user, isLoading: authLoading } = useAuth();
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const joinMutation = useJoinGroup();

  useEffect(() => {
    const handleInvite = async () => {
      if (authLoading) return;

      if (!token) {
        setError('Invalid invite link');
        setIsLoading(false);
        return;
      }

      if (!user) {
        await setPendingInviteToken(token);
        router.replace('/(auth)/login');
        return;
      }

      try {
        const group = await joinMutation.mutateAsync(token);
        router.replace(`/(app)/groups/${group.id}`);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to join group');
        setIsLoading(false);
      }
    };

    handleInvite();
  }, [token, user, authLoading]);

  if (isLoading && !error) {
    return <LoadingSpinner message="Joining group…" />;
  }

  if (error) {
    return <ErrorMessage message={error} />;
  }

  return null;
}
