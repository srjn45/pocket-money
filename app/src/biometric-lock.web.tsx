// Web stub — no device biometrics in the browser. Keeps the provider tree and
// hook identical across platforms so callers never branch on Platform.OS.
import React, { createContext, useContext, ReactNode } from 'react';

import type { BiometricLockValue } from './biometric-lock.types';

const stub: BiometricLockValue = {
  supported: false,
  enabled: false,
  locked: false,
  setEnabled: async () => false,
  unlock: async () => true,
};

const BiometricLockContext = createContext<BiometricLockValue>(stub);

export function BiometricLockProvider({ children }: { children: ReactNode }) {
  return <BiometricLockContext.Provider value={stub}>{children}</BiometricLockContext.Provider>;
}

export function useBiometricLock(): BiometricLockValue {
  return useContext(BiometricLockContext);
}

export function BiometricLockOverlay() {
  return null;
}
