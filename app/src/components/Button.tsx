import { ActivityIndicator, Pressable, StyleSheet, Text, View } from 'react-native';
import type { ViewStyle } from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { theme } from '../theme';

interface ButtonProps {
  title: string;
  onPress: () => void;
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost'; // default 'primary'
  size?: 'sm' | 'md';                                       // default 'md'
  disabled?: boolean;
  /** Shows a spinner in place of the title and blocks presses. */
  loading?: boolean;
  /** Leading icon. */
  icon?: keyof typeof Ionicons.glyphMap;
  /** Stretch to container width (default false). */
  fullWidth?: boolean;
  style?: ViewStyle;
  testID?: string;
}

const variantStyle = {
  primary:   { bg: theme.color.primary,      fg: theme.color.primaryText },
  secondary: { bg: theme.color.primaryMuted, fg: theme.color.primary     },
  danger:    { bg: theme.color.danger,        fg: '#FFFFFF'               },
  ghost:     { bg: 'transparent',             fg: theme.color.primary     },
} as const;

export function Button({
  title,
  onPress,
  variant = 'primary',
  size = 'md',
  disabled = false,
  loading = false,
  icon,
  fullWidth = false,
  style,
  testID,
}: ButtonProps) {
  const blocked = disabled || loading;
  const colors = variantStyle[variant];
  const iconSize = size === 'sm' ? 16 : 20;

  return (
    <Pressable
      onPress={blocked ? undefined : onPress}
      style={[
        styles.base,
        size === 'sm' ? styles.sm : styles.md,
        { backgroundColor: colors.bg },
        fullWidth && styles.fullWidth,
        blocked && styles.blocked,
        style,
      ]}
      accessibilityRole="button"
      accessibilityState={{ disabled: blocked }}
      testID={testID}
    >
      {loading ? (
        <ActivityIndicator color={colors.fg} size="small" />
      ) : (
        <View style={styles.inner}>
          {icon && (
            <Ionicons name={icon} size={iconSize} color={colors.fg} />
          )}
          {title.length > 0 && (
            <Text style={[styles.label, { color: colors.fg }, size === 'sm' && styles.labelSm]}>
              {title}
            </Text>
          )}
        </View>
      )}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  base: {
    borderRadius: theme.radius.md,
    alignItems: 'center',
    justifyContent: 'center',
    alignSelf: 'flex-start',
  },
  md: {
    paddingVertical: theme.spacing.lg,
    paddingHorizontal: theme.spacing.lg,
  },
  sm: {
    paddingVertical: theme.spacing.sm,
    paddingHorizontal: theme.spacing.md,
    minHeight: 44, // ≥44pt touch target (§4.4)
  },
  fullWidth: {
    alignSelf: 'stretch',
  },
  blocked: {
    opacity: 0.5,
  },
  inner: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: theme.spacing.sm,
  },
  label: {
    fontSize: theme.fontSize.sm,
    fontWeight: theme.fontWeight.semibold,
  },
  labelSm: {
    fontSize: theme.fontSize.xs,
  },
});
