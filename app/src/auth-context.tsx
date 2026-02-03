import React, { createContext, useContext, useState, useEffect, ReactNode } from 'react';
import { authApi, User, setOnUnauthorized } from './api';
import { getToken, setToken, clearToken } from './storage';

interface AuthContextValue {
  user: User | null;
  token: string | null;
  isLoading: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string, name: string) => Promise<void>;
  logout: () => Promise<void>;
  loadMe: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [token, setTokenState] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const logout = async () => {
    await clearToken();
    setTokenState(null);
    setUser(null);
  };

  // Set up unauthorized handler
  useEffect(() => {
    setOnUnauthorized(() => {
      logout();
    });
  }, []);

  const loadMe = async () => {
    try {
      const storedToken = await getToken();
      if (storedToken) {
        setTokenState(storedToken);
        const userData = await authApi.me();
        setUser(userData);
      }
    } catch (error) {
      // Token invalid or expired
      await clearToken();
      setTokenState(null);
      setUser(null);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    loadMe();
  }, []);

  const login = async (email: string, password: string) => {
    const response = await authApi.login({ email, password });
    await setToken(response.token);
    setTokenState(response.token);
    setUser(response.user);
  };

  const register = async (email: string, password: string, name: string) => {
    await authApi.register({ email, password, name });
  };

  return (
    <AuthContext.Provider value={{ user, token, isLoading, login, register, logout, loadMe }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
