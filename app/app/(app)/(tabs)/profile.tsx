import { useEffect, useRef, useState } from 'react';
import { Text, StyleSheet, Pressable } from 'react-native';
import Constants from 'expo-constants';
import { useAuth } from '../../../src/auth-context';
import { Card, Avatar, Button, Sheet, TextField, ScreenContainer, useToast } from '../../../src/components';
import { useChangePassword } from '../../../src/hooks/useHygiene';
import { theme } from '../../../src/theme';
import { getApiBaseUrl, getDefaultApiBaseUrl, setApiBaseUrlOverride } from '../../../src/api';

const DEV_TAP_THRESHOLD = 7;

export default function ProfileScreen() {
  const { user, logout } = useAuth();
  const { show: showToast } = useToast();

  const [sheetVisible, setSheetVisible] = useState(false);
  const [currentPw, setCurrentPw] = useState('');
  const [newPw, setNewPw] = useState('');
  const [confirmPw, setConfirmPw] = useState('');

  const changeMutation = useChangePassword();

  // Hidden dev-only API URL override — tap the version text DEV_TAP_THRESHOLD
  // times to reveal it. Not gated behind __DEV__: this is meant for testing a
  // release/preview build against a backend whose LAN address changes between
  // sessions, not just local development.
  const [devTapCount, setDevTapCount] = useState(0);
  const [devSheetVisible, setDevSheetVisible] = useState(false);
  const [devUrlInput, setDevUrlInput] = useState('');

  function handleVersionTap() {
    const next = devTapCount + 1;
    if (next >= DEV_TAP_THRESHOLD) {
      setDevTapCount(0);
      setDevUrlInput(getApiBaseUrl());
      setDevSheetVisible(true);
    } else {
      setDevTapCount(next);
    }
  }

  async function handleSaveApiUrl() {
    const trimmed = devUrlInput.trim();
    await setApiBaseUrlOverride(trimmed || null);
    showToast({ tone: 'success', message: trimmed ? 'API URL overridden' : 'API URL override cleared' });
    setDevSheetVisible(false);
  }

  async function handleClearApiUrl() {
    await setApiBaseUrlOverride(null);
    setDevUrlInput(getDefaultApiBaseUrl());
    showToast({ tone: 'success', message: 'API URL override cleared — using default' });
  }

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
    <ScreenContainer style={styles.container} testID="profile-root">
      {/* In-body title — the one meaningful header now that the tab header is off. */}
      <Text style={styles.title}>Profile</Text>
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
        testID="profile-change-password"
      />
      <Button
        title="Logout"
        variant="danger"
        onPress={handleLogout}
        fullWidth
        style={styles.logoutButton}
        testID="profile-logout"
      />

      <Sheet visible={sheetVisible} onClose={closeSheet} title="Change password">
        <TextField
          label="Current password"
          secureTextEntry
          value={currentPw}
          onChangeText={setCurrentPw}
          autoCapitalize="none"
          testID="cp-current-password"
        />
        <TextField
          label="New password"
          secureTextEntry
          value={newPw}
          onChangeText={setNewPw}
          autoCapitalize="none"
          testID="cp-new-password"
        />
        <TextField
          label="Confirm new password"
          secureTextEntry
          value={confirmPw}
          onChangeText={setConfirmPw}
          autoCapitalize="none"
          testID="cp-confirm-password"
        />
        <Button
          title="Change password"
          variant="primary"
          loading={changeMutation.isPending}
          onPress={handleChangePassword}
          fullWidth
          testID="cp-submit"
        />
        <Button
          title="Cancel"
          variant="ghost"
          onPress={closeSheet}
          fullWidth
          style={styles.cancelButton}
        />
      </Sheet>

      {/* Hidden trigger: tap DEV_TAP_THRESHOLD times to reveal the API URL override. */}
      <Pressable onPress={handleVersionTap} testID="profile-version-tap">
        <Text style={styles.version}>v{Constants.expoConfig?.version ?? '1.0.0'}</Text>
      </Pressable>

      <Sheet visible={devSheetVisible} onClose={() => setDevSheetVisible(false)} title="Developer settings">
        <Text style={styles.devHint}>
          Overrides the API base URL on this device only. Useful for pointing a preview/test
          build at a backend on your LAN. Takes effect immediately — no restart needed.
        </Text>
        <TextField
          label="API base URL"
          value={devUrlInput}
          onChangeText={setDevUrlInput}
          autoCapitalize="none"
          autoCorrect={false}
          placeholder={getDefaultApiBaseUrl()}
          testID="dev-api-url-input"
        />
        <Button
          title="Save"
          variant="primary"
          onPress={handleSaveApiUrl}
          fullWidth
          testID="dev-api-url-save"
        />
        <Button
          title="Clear override (use default)"
          variant="secondary"
          onPress={handleClearApiUrl}
          fullWidth
          style={styles.cancelButton}
        />
        <Button
          title="Cancel"
          variant="ghost"
          onPress={() => setDevSheetVisible(false)}
          fullWidth
          style={styles.cancelButton}
        />
      </Sheet>
    </ScreenContainer>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: theme.color.background,
    padding: theme.spacing.lg,
  },
  title: {
    fontSize: theme.fontSize.xl,
    fontWeight: theme.fontWeight.bold,
    color: theme.color.text,
    marginBottom: theme.spacing.lg,
  },
  profileCard: {
    alignItems: 'center',
    padding: theme.spacing.xl,
  },
  name: {
    fontSize: theme.fontSize.xl,
    fontWeight: theme.fontWeight.bold,
    color: theme.color.text,
    marginTop: theme.spacing.lg,
  },
  email: {
    fontSize: theme.fontSize.md,
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
  version: {
    fontSize: theme.fontSize.sm,
    color: theme.color.textSecondary,
    textAlign: 'center',
    marginTop: theme.spacing.xl,
    padding: theme.spacing.md,
  },
  devHint: {
    fontSize: theme.fontSize.sm,
    color: theme.color.textSecondary,
    marginBottom: theme.spacing.md,
  },
});
