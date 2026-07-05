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
    <Tabs screenOptions={{ headerShown: true }}>
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
        options={{ href: null }}
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
