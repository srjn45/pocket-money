import { Stack } from 'expo-router';
import { View, StyleSheet } from 'react-native';
import { useAuth } from '../../src/auth-context';
import { LoadingSpinner } from '../../src/components';
import { theme } from '../../src/theme';

export default function AppLayout() {
  const { token, user, isLoading } = useAuth();

  // Render gate: hold spinner while auth hydrates or token/user absent.
  // Redirect is handled by the root AuthGate in app/_layout.tsx.
  if (isLoading || !token || !user) {
    return (
      <View style={styles.loading}>
        <LoadingSpinner />
      </View>
    );
  }

  // (app) is a STACK, not Tabs. The two primary destinations (Dashboard, Profile)
  // live inside the nested (tabs) group; group detail and notifications are
  // push-in siblings. Making group entry a real Stack push/pop — instead of a
  // hidden-Tab focus/blur — is the fix for the stale-content nav leak: on the web
  // build the outer Tabs never set display:none on the inactive `groups` tab, so
  // its nested Stack accumulated a fresh layer on every re-entry. A Stack push
  // correctly hides the superseded screen and a pop unmounts it, so re-entering a
  // group is always clean. headerShown:false here → no duplicate header; each
  // section owns its header ((tabs) screens in-body, groups via their own Stack,
  // notifications in-body).
  return (
    <Stack screenOptions={{ headerShown: false }}>
      <Stack.Screen name="(tabs)" />
      <Stack.Screen name="groups" />
      <Stack.Screen name="notifications" />
    </Stack>
  );
}

const styles = StyleSheet.create({
  loading: {
    flex: 1,
    backgroundColor: theme.color.background,
  },
});
