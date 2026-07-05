import {
  KeyboardAvoidingView,
  Modal,
  Platform,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { theme } from '../theme';

interface SheetProps {
  visible: boolean;
  onClose: () => void;
  title?: string;
  children: React.ReactNode;
  /** Pinned action region at the bottom (e.g. Cancel / Save buttons). */
  footer?: React.ReactNode;
}

export function Sheet({ visible, onClose, title, children, footer }: SheetProps) {
  const isWeb = Platform.OS === 'web';

  const headerNode = title ? (
    <View style={styles.header}>
      <Text style={styles.headerTitle}>{title}</Text>
      <Pressable onPress={onClose} style={styles.closeBtn} hitSlop={8}>
        <Ionicons name="close" size={24} color={theme.color.textSecondary} />
      </Pressable>
    </View>
  ) : null;

  const sheetContent = (
    <Pressable
      style={isWeb ? styles.webContent : styles.nativeContent}
      onPress={(e) => e.stopPropagation()}
    >
      {headerNode}
      <ScrollView
        style={styles.body}
        contentContainerStyle={styles.bodyContent}
        keyboardShouldPersistTaps="handled"
      >
        {children}
      </ScrollView>
      {footer ? (
        <View style={styles.footer}>{footer}</View>
      ) : null}
    </Pressable>
  );

  return (
    <Modal
      visible={visible}
      transparent
      onRequestClose={onClose}
      animationType={isWeb ? 'fade' : 'slide'}
    >
      {isWeb ? (
        <Pressable style={styles.webOverlay} onPress={onClose}>
          {sheetContent}
        </Pressable>
      ) : (
        <View style={styles.nativeOverlay}>
          <Pressable style={StyleSheet.absoluteFillObject} onPress={onClose} />
          <View style={styles.nativePositioner}>
            <KeyboardAvoidingView
              behavior={Platform.OS === 'ios' ? 'padding' : undefined}
            >
              {sheetContent}
            </KeyboardAvoidingView>
          </View>
        </View>
      )}
    </Modal>
  );
}

const styles = StyleSheet.create({
  // ── native ──────────────────────────────────────────────────────────────
  nativeOverlay: {
    flex: 1,
    backgroundColor: theme.color.overlay,
  },
  nativePositioner: {
    flex: 1,
    justifyContent: 'flex-end',
    // Allow the Pressable scrim above to receive taps in the empty area.
    pointerEvents: 'box-none' as never,
  },
  nativeContent: {
    backgroundColor: theme.color.surface,
    borderTopLeftRadius: theme.radius.lg,
    borderTopRightRadius: theme.radius.lg,
    paddingBottom: theme.spacing.xl,
    maxHeight: '85%' as never,
  },
  // ── web ─────────────────────────────────────────────────────────────────
  webOverlay: {
    flex: 1,
    backgroundColor: theme.color.overlay,
    alignItems: 'center',
    justifyContent: 'center',
    padding: theme.spacing.lg,
  },
  webContent: {
    backgroundColor: theme.color.surface,
    borderRadius: theme.radius.md,
    maxWidth: 480,
    width: '100%' as never,
    maxHeight: '90%' as never,
  },
  // ── shared ──────────────────────────────────────────────────────────────
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: theme.spacing.lg,
    paddingBottom: theme.spacing.md,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: theme.color.border,
  },
  headerTitle: {
    fontSize: theme.fontSize.lg,
    fontWeight: theme.fontWeight.bold,
    color: theme.color.text,
  },
  closeBtn: {
    padding: theme.spacing.xs,
  },
  body: {
    flexShrink: 1,
  },
  bodyContent: {
    padding: theme.spacing.lg,
  },
  footer: {
    padding: theme.spacing.lg,
    paddingTop: theme.spacing.md,
    borderTopWidth: StyleSheet.hairlineWidth,
    borderTopColor: theme.color.border,
    flexDirection: 'row',
    gap: theme.spacing.sm,
  },
});
