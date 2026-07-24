// Shared contract for the biometric app-lock, split like DateField:
// biometric-lock.tsx (native, expo-local-authentication) and
// biometric-lock.web.tsx (stub — web sessions live behind the browser, no
// device biometrics to offer). Keeping the type here forces both
// implementations to stay in sync.

export interface BiometricLockValue {
  /** Device has biometric hardware AND at least one biometric enrolled. */
  supported: boolean;
  /** User turned the app-lock on (implies supported at the time of enabling). */
  enabled: boolean;
  /** App is currently locked behind the biometric prompt. */
  locked: boolean;
  /**
   * Toggle the lock. Enabling (and disabling) requires passing the biometric
   * prompt first. Resolves true if the setting was changed.
   */
  setEnabled: (on: boolean) => Promise<boolean>;
  /** Re-run the unlock prompt. Resolves true on success. */
  unlock: () => Promise<boolean>;
}
