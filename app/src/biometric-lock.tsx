import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  ReactNode,
} from 'react';
import { AppState, AppStateStatus, Modal, StyleSheet, Text, View } from 'react-native';
import * as LocalAuthentication from 'expo-local-authentication';
import { Ionicons } from '@expo/vector-icons';
import { getBiometricLockEnabled, setBiometricLockEnabled } from './storage';
import { useAuth } from './auth-context';
import { Button } from './components';
import { theme } from './theme';

import type { BiometricLockValue } from './biometric-lock.types';

const BiometricLockContext = createContext<BiometricLockValue | undefined>(undefined);

async function promptBiometric(promptMessage: string): Promise<boolean> {
  const result = await LocalAuthentication.authenticateAsync({
    promptMessage,
    cancelLabel: 'Cancel',
    // Allow PIN/pattern fallback: same trust level as the phone's lock screen,
    // and the only way in when a wet/injured finger keeps failing.
    disableDeviceFallback: false,
  });
  return result.success;
}

export function BiometricLockProvider({ children }: { children: ReactNode }) {
  const [supported, setSupported] = useState(false);
  const [enabled, setEnabledState] = useState(false);
  const [locked, setLocked] = useState(false);
  // The OS biometric dialog can briefly background the activity on some
  // Android devices — without this guard the AppState listener would re-lock
  // mid-prompt and loop forever.
  const authInFlight = useRef(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const [hasHardware, isEnrolled, storedEnabled] = await Promise.all([
        LocalAuthentication.hasHardwareAsync(),
        LocalAuthentication.isEnrolledAsync(),
        getBiometricLockEnabled(),
      ]);
      if (cancelled) return;
      const isSupported = hasHardware && isEnrolled;
      setSupported(isSupported);
      // If biometrics were un-enrolled since enabling, fail open rather than
      // brick the app behind a prompt that can never succeed.
      const effective = storedEnabled && isSupported;
      setEnabledState(effective);
      setLocked(effective); // cold start begins locked
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  // Re-lock whenever the app is sent to the background.
  useEffect(() => {
    if (!enabled) return;
    const sub = AppState.addEventListener('change', (state: AppStateStatus) => {
      if (state === 'background' && !authInFlight.current) {
        setLocked(true);
      }
    });
    return () => sub.remove();
  }, [enabled]);

  const unlock = useCallback(async (): Promise<boolean> => {
    if (authInFlight.current) return false;
    authInFlight.current = true;
    try {
      const ok = await promptBiometric('Unlock Pocket Money');
      if (ok) setLocked(false);
      return ok;
    } finally {
      authInFlight.current = false;
    }
  }, []);

  const setEnabled = useCallback(
    async (on: boolean): Promise<boolean> => {
      if (on && !supported) return false;
      if (authInFlight.current) return false;
      authInFlight.current = true;
      try {
        // Both directions confirm via the prompt: enabling proves biometrics
        // actually work on this device; disabling stops anyone who picks up an
        // unlocked phone from quietly switching the lock off.
        const ok = await promptBiometric(
          on ? 'Confirm to enable app lock' : 'Confirm to disable app lock'
        );
        if (!ok) return false;
        await setBiometricLockEnabled(on);
        setEnabledState(on);
        if (!on) setLocked(false);
        return true;
      } finally {
        authInFlight.current = false;
      }
    },
    [supported]
  );

  return (
    <BiometricLockContext.Provider value={{ supported, enabled, locked, setEnabled, unlock }}>
      {children}
    </BiometricLockContext.Provider>
  );
}

export function useBiometricLock(): BiometricLockValue {
  const context = useContext(BiometricLockContext);
  if (context === undefined) {
    throw new Error('useBiometricLock must be used within a BiometricLockProvider');
  }
  return context;
}

// Full-screen lock. A Modal (not an absolute View) so it reliably covers the
// native screens react-native-screens hosts. Only engages when a session
// exists — the login screen has nothing worth hiding.
export function BiometricLockOverlay() {
  const { enabled, locked, unlock } = useBiometricLock();
  const { token } = useAuth();
  const show = enabled && locked && !!token;

  // Auto-prompt once per lock engagement, and only while the app is in the
  // foreground (the lock usually engages on the way INTO the background —
  // prompting then would surface the dialog at the wrong moment). After a
  // cancel/failure the user retries via the button.
  const autoPrompted = useRef(false);
  useEffect(() => {
    if (!show) {
      autoPrompted.current = false;
      return;
    }
    const tryPrompt = () => {
      if (!autoPrompted.current && AppState.currentState === 'active') {
        autoPrompted.current = true;
        unlock();
      }
    };
    tryPrompt();
    const sub = AppState.addEventListener('change', (state: AppStateStatus) => {
      if (state === 'active') tryPrompt();
    });
    return () => sub.remove();
  }, [show, unlock]);

  if (!show) return null;

  return (
    <Modal visible animationType="fade" statusBarTranslucent onRequestClose={() => {}}>
      <View style={styles.container} testID="biometric-lock-screen">
        <Ionicons name="lock-closed" size={56} color={theme.color.primary} />
        <Text style={styles.title}>Pocket Money is locked</Text>
        <Text style={styles.subtitle}>Unlock with your fingerprint or face</Text>
        <Button
          title="Unlock"
          variant="primary"
          onPress={unlock}
          style={styles.unlockButton}
          testID="biometric-unlock"
        />
      </View>
    </Modal>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: theme.color.background,
    alignItems: 'center',
    justifyContent: 'center',
    padding: theme.spacing.xl,
  },
  title: {
    fontSize: theme.fontSize.lg,
    fontWeight: theme.fontWeight.bold,
    color: theme.color.text,
    marginTop: theme.spacing.lg,
  },
  subtitle: {
    fontSize: theme.fontSize.md,
    color: theme.color.textSecondary,
    marginTop: theme.spacing.xs,
  },
  unlockButton: {
    marginTop: theme.spacing.xl,
    minWidth: 200,
  },
});
