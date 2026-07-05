import { useEffect } from 'react';
import { AppState, AppStateStatus } from 'react-native';
import { Stack, useRouter, useSegments } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { QueryClient, QueryClientProvider, focusManager } from '@tanstack/react-query';
import { AuthProvider, useAuth } from '../src/auth-context';
import { ToastProvider } from '../src/components';
import { getPendingInviteToken } from '../src/storage';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      staleTime: 30_000,
      refetchOnWindowFocus: true,
      refetchOnReconnect: true,
    },
    mutations: {
      retry: 0,
    },
  },
});

// Single global auth guard — reacts to every auth-state change across the entire route tree.
// Must be rendered inside AuthProvider so useAuth() is available.
function AuthGate() {
  const segments = useSegments();
  const router = useRouter();
  const { token, isLoading } = useAuth();

  useEffect(() => {
    if (isLoading) return;
    const inAuthGroup = segments[0] === '(auth)';
    // The /invite route owns the unauthenticated case itself (it saves the
    // pending token before redirecting to login). AuthGate must NOT redirect it
    // away, or it races that save and the register/login-via-invite resume loses
    // the token (WP-4.6 §12 G1).
    const onInvite = segments[0] === 'invite';
    if (!token && !inAuthGroup && !onInvite) {
      router.replace('/(auth)/login');
      return;
    }
    if (token && inAuthGroup) {
      // Single post-auth navigation authority: honor a pending invite so
      // register/login-via-invite lands the user inside the group; otherwise
      // drop into the app. Centralizing this here (rather than also in the auth
      // screens) removes the AuthGate-vs-auth-screen navigation race.
      let cancelled = false;
      (async () => {
        const pending = await getPendingInviteToken();
        if (cancelled) return;
        if (pending) {
          router.replace({ pathname: '/invite', params: { token: pending } });
        } else {
          router.replace('/(app)');
        }
      })();
      return () => {
        cancelled = true;
      };
    }
  }, [token, isLoading, segments, router]);

  return null;
}

export default function RootLayout() {
  // Wire AppState → focusManager so pull-to-refresh and focus-refetch work on RN.
  useEffect(() => {
    const sub = AppState.addEventListener('change', (state: AppStateStatus) => {
      focusManager.setFocused(state === 'active');
    });
    return () => sub.remove();
  }, []);

  return (
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <ToastProvider>
          <AuthGate />
          <Stack>
            <Stack.Screen name="index" options={{ headerShown: false }} />
            <Stack.Screen name="(auth)" options={{ headerShown: false }} />
            <Stack.Screen name="(app)" options={{ headerShown: false }} />
            <Stack.Screen name="invite" options={{ title: 'Join Group' }} />
          </Stack>
          <StatusBar style="auto" />
        </ToastProvider>
      </AuthProvider>
    </QueryClientProvider>
  );
}
