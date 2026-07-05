import { useEffect, useRef, useState } from 'react';
import { View, Text, StyleSheet } from 'react-native';
import { useAuth } from '../../src/auth-context';
import { Card, Avatar, Button, Sheet, TextField, useToast } from '../../src/components';
import { useChangePassword } from '../../src/hooks/useHygiene';
import { theme } from '../../src/theme';

export default function ProfileScreen() {
  const { user, logout } = useAuth();
  const { show: showToast } = useToast();

  const [sheetVisible, setSheetVisible] = useState(false);
  const [currentPw, setCurrentPw] = useState('');
  const [newPw, setNewPw] = useState('');
  const [confirmPw, setConfirmPw] = useState('');

  const changeMutation = useChangePassword();

  // Re-clear fields each time the sheet opens (closed→open reset pattern).
  const prevVisible = useRef(sheetVisible);
  useEffect(() => {
    if (sheetVisible && !prevVisible.current) {
      setCurrentPw('');
      setNewPw('');
      setConfirmPw('');
    }
    prevVisible.current = sheetVisible;
  }, [sheetVisible]);

  function closeSheet() {
    setSheetVisible(false);
    setCurrentPw('');
    setNewPw('');
    setConfirmPw('');
  }

  async function handleChangePassword() {
    if (!currentPw || !newPw || !confirmPw) {
      showToast({ tone: 'danger', message: 'All fields are required' });
      return;
    }
    if (newPw !== confirmPw) {
      showToast({ tone: 'danger', message: "New passwords don't match" });
      return;
    }
    if (newPw.length < 6) {
      showToast({ tone: 'danger', message: 'New password must be at least 6 characters' });
      return;
    }
    try {
      await changeMutation.mutateAsync({ current_password: currentPw, new_password: newPw });
      showToast({ tone: 'success', message: 'Password changed' });
      closeSheet();
    } catch (e) {
      showToast({ tone: 'danger', message: e instanceof Error ? e.message : 'Failed to change password' });
    }
  }

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
        title="Change password"
        variant="secondary"
        onPress={() => setSheetVisible(true)}
        fullWidth
        style={styles.changePasswordButton}
      />
      <Button
        title="Logout"
        variant="danger"
        onPress={handleLogout}
        fullWidth
        style={styles.logoutButton}
      />

      <Sheet visible={sheetVisible} onClose={closeSheet} title="Change password">
        <TextField
          label="Current password"
          secureTextEntry
          value={currentPw}
          onChangeText={setCurrentPw}
          autoCapitalize="none"
        />
        <TextField
          label="New password"
          secureTextEntry
          value={newPw}
          onChangeText={setNewPw}
          autoCapitalize="none"
        />
        <TextField
          label="Confirm new password"
          secureTextEntry
          value={confirmPw}
          onChangeText={setConfirmPw}
          autoCapitalize="none"
        />
        <Button
          title="Change password"
          variant="primary"
          loading={changeMutation.isPending}
          onPress={handleChangePassword}
          fullWidth
        />
        <Button
          title="Cancel"
          variant="ghost"
          onPress={closeSheet}
          fullWidth
          style={styles.cancelButton}
        />
      </Sheet>
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
  changePasswordButton: {
    marginTop: theme.spacing.xl,
  },
  logoutButton: {
    marginTop: theme.spacing.md,
  },
  cancelButton: {
    marginTop: theme.spacing.sm,
  },
});
