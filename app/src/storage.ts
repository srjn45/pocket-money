import { Platform } from 'react-native';
import * as SecureStore from 'expo-secure-store';
import AsyncStorage from '@react-native-async-storage/async-storage';

const TOKEN_KEY = 'auth_token';
const PENDING_TOKEN_KEY = 'pending_invite_token';
const API_URL_OVERRIDE_KEY = 'dev_api_url_override';

// Use SecureStore on native, AsyncStorage on web
const isWeb = Platform.OS === 'web';

export async function getToken(): Promise<string | null> {
  try {
    if (isWeb) {
      return await AsyncStorage.getItem(TOKEN_KEY);
    }
    return await SecureStore.getItemAsync(TOKEN_KEY);
  } catch {
    return null;
  }
}

export async function setToken(token: string): Promise<void> {
  try {
    if (isWeb) {
      await AsyncStorage.setItem(TOKEN_KEY, token);
    } else {
      await SecureStore.setItemAsync(TOKEN_KEY, token);
    }
  } catch (error) {
    console.error('Failed to save token:', error);
  }
}

export async function clearToken(): Promise<void> {
  try {
    if (isWeb) {
      await AsyncStorage.removeItem(TOKEN_KEY);
    } else {
      await SecureStore.deleteItemAsync(TOKEN_KEY);
    }
  } catch (error) {
    console.error('Failed to clear token:', error);
  }
}

export async function getPendingInviteToken(): Promise<string | null> {
  try {
    if (isWeb) {
      return await AsyncStorage.getItem(PENDING_TOKEN_KEY);
    }
    return await SecureStore.getItemAsync(PENDING_TOKEN_KEY);
  } catch {
    return null;
  }
}

export async function setPendingInviteToken(token: string): Promise<void> {
  try {
    if (isWeb) {
      await AsyncStorage.setItem(PENDING_TOKEN_KEY, token);
    } else {
      await SecureStore.setItemAsync(PENDING_TOKEN_KEY, token);
    }
  } catch (error) {
    console.error('Failed to save pending invite token:', error);
  }
}

export async function clearPendingInviteToken(): Promise<void> {
  try {
    if (isWeb) {
      await AsyncStorage.removeItem(PENDING_TOKEN_KEY);
    } else {
      await SecureStore.deleteItemAsync(PENDING_TOKEN_KEY);
    }
  } catch (error) {
    console.error('Failed to clear pending invite token:', error);
  }
}

// Dev-only API base URL override — not sensitive, plain AsyncStorage on every
// platform (unlike the token, no need for SecureStore here).
export async function getApiUrlOverride(): Promise<string | null> {
  try {
    return await AsyncStorage.getItem(API_URL_OVERRIDE_KEY);
  } catch {
    return null;
  }
}

export async function setApiUrlOverride(url: string): Promise<void> {
  await AsyncStorage.setItem(API_URL_OVERRIDE_KEY, url);
}

export async function clearApiUrlOverride(): Promise<void> {
  await AsyncStorage.removeItem(API_URL_OVERRIDE_KEY);
}
