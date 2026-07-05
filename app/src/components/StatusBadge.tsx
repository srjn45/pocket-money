import { StyleSheet, Text, View } from 'react-native';
import { theme } from '../theme';

// Tint backgrounds derived from token hues — badge-specific local constants.
const toneBg = {
  neutral: theme.color.surfaceMuted,
  success: '#DCFCE7',
  warning: '#FEF3C7',
  danger:  '#FEE2E2',
  info:    theme.color.primaryMuted,
} as const;

const toneFg = {
  neutral: theme.color.textSecondary,
  success: theme.color.success,
  warning: theme.color.warning,
  danger:  theme.color.danger,
  info:    theme.color.primary,
} as const;

interface StatusBadgeProps {
  label: string;
  /** Default 'neutral'. */
  tone?: 'neutral' | 'success' | 'warning' | 'danger' | 'info';
}

export function StatusBadge({ label, tone = 'neutral' }: StatusBadgeProps) {
  return (
    <View style={[styles.badge, { backgroundColor: toneBg[tone] }]}>
      <Text style={[styles.label, { color: toneFg[tone] }]}>
        {label.toUpperCase()}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  badge: {
    borderRadius: theme.radius.pill,
    paddingHorizontal: theme.spacing.sm,
    paddingVertical: theme.spacing.xs,
    alignSelf: 'flex-start',
  },
  label: {
    fontSize: theme.fontSize.xs,
    fontWeight: theme.fontWeight.semibold,
  },
});
