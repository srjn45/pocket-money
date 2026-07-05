import { Pressable, StyleSheet, Text, View } from 'react-native';
import type { StyleProp, ViewStyle } from 'react-native';
import { theme } from '../theme';

interface ListRowProps {
  title: string;
  subtitle?: string;
  /** Leading slot: icon, avatar, type glyph. */
  left?: React.ReactNode;
  /** Trailing slot: AmountText, StatusBadge, action buttons. */
  right?: React.ReactNode;
  onPress?: () => void;
  disabled?: boolean;
  /** Renders the title with strikethrough (rejected ledger entries). Default false. */
  strikethrough?: boolean;
  style?: ViewStyle;
}

export function ListRow({
  title,
  subtitle,
  left,
  right,
  onPress,
  disabled = false,
  strikethrough = false,
  style,
}: ListRowProps) {
  const content = (
    <>
      {left && <View style={styles.leftSlot}>{left}</View>}
      <View style={styles.textCol}>
        <Text
          style={[
            styles.title,
            strikethrough && styles.titleStrikethrough,
          ]}
          numberOfLines={2}
        >
          {title}
        </Text>
        {subtitle ? (
          <Text style={styles.subtitle} numberOfLines={2}>
            {subtitle}
          </Text>
        ) : null}
      </View>
      {right && <View style={styles.rightSlot}>{right}</View>}
    </>
  );

  const rowStyle: StyleProp<ViewStyle> = [styles.row, disabled && styles.disabled, style];

  if (onPress) {
    return (
      <Pressable
        onPress={disabled ? undefined : onPress}
        style={rowStyle}
        accessibilityRole="button"
        accessibilityState={{ disabled }}
      >
        {content}
      </Pressable>
    );
  }

  return <View style={rowStyle}>{content}</View>;
}

const styles = StyleSheet.create({
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: theme.spacing.md,
    padding: theme.spacing.lg,
    backgroundColor: theme.color.surface,
  },
  leftSlot: {
    alignItems: 'center',
    justifyContent: 'center',
  },
  textCol: {
    flex: 1,
  },
  title: {
    fontSize: theme.fontSize.md,
    fontWeight: theme.fontWeight.semibold,
    color: theme.color.text,
  },
  titleStrikethrough: {
    textDecorationLine: 'line-through',
    color: theme.color.textMuted,
  },
  subtitle: {
    fontSize: theme.fontSize.xs,
    color: theme.color.textSecondary,
    marginTop: theme.spacing.xs,
  },
  rightSlot: {
    alignItems: 'center',
    justifyContent: 'center',
  },
  disabled: {
    opacity: 0.5,
  },
});
