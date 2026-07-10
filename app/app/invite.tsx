import { useState, useEffect } from 'react';
import { View, StyleSheet } from 'react-native';
import { useLocalSearchParams, router } from 'expo-router';
import { useAuth } from '../src/auth-context';
import { setPendingInviteToken, clearPendingInviteToken } from '../src/storage';
import { useJoinGroup } from '../src/hooks/useGroups';
import { LoadingSpinner, ErrorMessage, Button, ScreenContainer } from '../src/components';
import { theme } from '../src/theme';

type ErrorKind = 'invalid' | 'already-member' | 'network';

function classifyError(err: unknown): { message: string; kind: ErrorKind } {
  const msg = err instanceof Error ? err.message : '';
  const lower = msg.toLowerCase();
  if (lower.includes('already')) {
    return { message: "You're already in this group.", kind: 'already-member' };
  }
  if (lower.includes('expired') || lower.includes('invalid')) {
    return {
      message: 'This invite link is invalid or has expired. Ask for a new one.',
      kind: 'invalid',
    };
  }
  return {
    message: msg || "Couldn't join right now. Check your connection and try again.",
    kind: 'network',
  };
}

export default function InviteScreen() {
  const { token } = useLocalSearchParams<{ token?: string }>();
  const { user, isLoading: authLoading } = useAuth();
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const [errorKind, setErrorKind] = useState<ErrorKind | null>(null);
  const joinMutation = useJoinGroup();

  useEffect(() => {
    const handleInvite = async () => {
      if (authLoading) return;

      if (!token) {
        setError('This invite link is invalid or incomplete.');
        setErrorKind('invalid');
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
        await clearPendingInviteToken();
        router.replace(`/(app)/groups/${group.id}`);
      } catch (err) {
        const { message, kind } = classifyError(err);
        setError(message);
        setErrorKind(kind);
        setIsLoading(false);
      }
    };

    handleInvite();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token, user, authLoading]);

  if (isLoading && !error) {
    return (
      <View testID="invite-loading">
        <LoadingSpinner message="Joining group…" />
      </View>
    );
  }

  if (error) {
    return (
      <ScreenContainer style={styles.container} testID="invite-error">
        <ErrorMessage message={error} />
        {errorKind === 'network' ? (
          <Button
            title="Retry"
            onPress={() =>
              router.replace({ pathname: '/invite', params: { token: token ?? '' } })
            }
            testID="invite-back"
          />
        ) : (
          <Button
            title="Back to groups"
            onPress={() => router.replace('/(app)')}
            testID="invite-back"
          />
        )}
      </ScreenContainer>
    );
  }

  return null;
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: theme.color.background,
    justifyContent: 'center',
    alignItems: 'center',
    padding: 24,
  },
});
