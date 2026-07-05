import { View, Text, StyleSheet } from 'react-native';
import { useAuth } from '../../src/auth-context';
import { Card, Avatar, Button } from '../../src/components';
import { theme } from '../../src/theme';

export default function ProfileScreen() {
  const { user, logout } = useAuth();

  const handleLogout = async () => {
    await logout();
    // Root AuthGate in app/_layout.tsx reacts to token=null and redirects to login.
  };

  return (
    <View style={styles.container}>
      <Card style={styles.profileCard}>
        <Avatar name={user?.name ?? 'User'} id={user?.id ?? ''} size={80} />
        <Text style={styles.name}>{user?.name || 'User'}</Text>
        <Text style={styles.email}>{user?.email}</Text>
      </Card>
      <Button
        title="Logout"
        variant="danger"
        onPress={handleLogout}
        fullWidth
        style={styles.logoutButton}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: theme.color.background,
    padding: theme.spacing.lg,
  },
  profileCard: {
    alignItems: 'center',
    padding: theme.spacing.xl,
  },
  name: {
    fontSize: 24,
    fontWeight: 'bold',
    color: theme.color.text,
    marginTop: theme.spacing.lg,
  },
  email: {
    fontSize: 16,
    color: theme.color.textSecondary,
    marginTop: theme.spacing.xs,
  },
  logoutButton: {
    marginTop: theme.spacing.xl,
  },
});
