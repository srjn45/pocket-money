import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { theme } from '../theme';

type ToastTone = 'success' | 'danger' | 'info';

interface ShowToastOptions {
  message: string;
  tone?: ToastTone;   // default 'info'
  durationMs?: number; // default 3000
}

interface ToastContextValue {
  show: (opts: ShowToastOptions) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

interface ToastState {
  message: string;
  tone: ToastTone;
}

const toneColor = {
  success: theme.color.success,
  danger:  theme.color.danger,
  info:    theme.color.primary,
} as const;

const toneIcon: Record<ToastTone, keyof typeof Ionicons.glyphMap> = {
  success: 'checkmark-circle',
  danger:  'alert-circle',
  info:    'information-circle',
};

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toast, setToast] = useState<ToastState | null>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const dismiss = useCallback(() => {
    if (timerRef.current) clearTimeout(timerRef.current);
    setToast(null);
  }, []);

  const show = useCallback((opts: ShowToastOptions) => {
    const { message, tone = 'info', durationMs = 3000 } = opts;
    if (timerRef.current) clearTimeout(timerRef.current);
    setToast({ message, tone });
    timerRef.current = setTimeout(dismiss, durationMs);
  }, [dismiss]);

  useEffect(() => {
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, []);

  return (
    <ToastContext.Provider value={{ show }}>
      <View style={styles.wrapper}>
        {children}
        {toast ? (
          <View style={styles.overlay} pointerEvents="box-none">
            <Pressable
              onPress={dismiss}
              style={[styles.card, { borderLeftColor: toneColor[toast.tone] }]}
            >
              <Ionicons
                name={toneIcon[toast.tone]}
                size={20}
                color={toneColor[toast.tone]}
              />
              <Text style={styles.message} numberOfLines={3}>
                {toast.message}
              </Text>
            </Pressable>
          </View>
        ) : null}
      </View>
    </ToastContext.Provider>
  );
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error('useToast must be used inside ToastProvider');
  return ctx;
}

const styles = StyleSheet.create({
  wrapper: {
    flex: 1,
  },
  overlay: {
    ...StyleSheet.absoluteFillObject,
    alignItems: 'center',
    justifyContent: 'flex-start',
    paddingTop: 48,
    paddingHorizontal: theme.spacing.lg,
    // pointerEvents box-none set via prop above
  },
  card: {
    backgroundColor: theme.color.surface,
    borderRadius: theme.radius.md,
    borderLeftWidth: 4,
    padding: theme.spacing.md,
    flexDirection: 'row',
    alignItems: 'center',
    gap: theme.spacing.sm,
    // Shadow
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.12,
    shadowRadius: 8,
    elevation: 4,
    maxWidth: 480,
    width: '100%' as never,
  },
  message: {
    flex: 1,
    fontSize: theme.fontSize.sm,
    color: theme.color.text,
    fontWeight: theme.fontWeight.medium,
  },
});
