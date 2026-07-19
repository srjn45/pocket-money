import { Tabs } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
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

  return (
    // headerShown:false → the (app) Tabs is the SOLE nav chrome (bottom bar on
    // native / top nav on web) with NO tab-level header. Each screen owns its one
    // meaningful header in-body (Dashboard/Profile) or via its own Stack (groups).
    <Tabs screenOptions={{ headerShown: false }}>
      <Tabs.Screen
        name="index"
        options={{
          title: 'Dashboard',
          tabBarIcon: ({ color, size }) => (
            <Ionicons name="home" size={size} color={color} />
          ),
        }}
      />
      <Tabs.Screen
        name="groups"
        // headerShown:false kills the leaked lowercase "groups" folder-name header
        // that the outer Tabs otherwise derived for this hidden route.
        options={{ href: null, headerShown: false }}
      />
      <Tabs.Screen
        // Notifications list (V3-5.2). Hidden route reached via the header bell's
        // router.push; href:null keeps it out of the bottom bar / top nav.
        name="notifications"
        options={{ href: null, headerShown: false }}
      />
      <Tabs.Screen
        name="profile"
        options={{
          title: 'Profile',
          tabBarIcon: ({ color, size }) => (
            <Ionicons name="person" size={size} color={color} />
          ),
        }}
      />
    </Tabs>
  );
}

const styles = StyleSheet.create({
  loading: {
    flex: 1,
    backgroundColor: theme.color.background,
  },
});
