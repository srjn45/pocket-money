import { Pressable, StyleSheet, View } from 'react-native';
import type { StyleProp, ViewStyle } from 'react-native';
import { theme } from '../theme';

interface CardProps {
  children: React.ReactNode;
  /** If set, the whole card is pressable. */
  onPress?: () => void;
  /** Apply default inner padding (theme.spacing.lg). Default true. */
  padded?: boolean;
  disabled?: boolean;
  style?: ViewStyle;
}

export function Card({ children, onPress, padded = true, disabled = false, style }: CardProps) {
  const cardStyle: StyleProp<ViewStyle> = [
    styles.card,
    padded && styles.padded,
    disabled && styles.disabled,
    style,
  ];

  if (onPress) {
    return (
      <Pressable onPress={disabled ? undefined : onPress} style={cardStyle}>
        {children}
      </Pressable>
    );
  }

  return <View style={cardStyle}>{children}</View>;
}

const styles = StyleSheet.create({
  card: {
    backgroundColor: theme.color.surface,
    borderRadius: theme.radius.md,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: theme.color.border,
    marginBottom: theme.spacing.sm,
  },
  padded: {
    padding: theme.spacing.lg,
  },
  disabled: {
    opacity: 0.5,
  },
});
