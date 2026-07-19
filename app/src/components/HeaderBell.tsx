import { Pressable, StyleSheet, Text, View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { router } from 'expo-router';
import { theme } from '../theme';
import { useUnreadCount } from '../hooks/useNotifications';

/**
 * Notifications bell for the app header. A shared RN component (no Platform split,
 * no native Stack headerRight wiring) so it renders identically on web / Android /
 * iOS from one source (§5). The unread badge is driven by useUnreadCount and is
 * rendered ONLY when count > 0 (so the e2e asserts presence/absence via toHaveCount).
 */
export function HeaderBell() {
  const { data } = useUnreadCount();
  const count = data?.count ?? 0;

  return (
    <Pressable
      testID="header-bell"
      onPress={() => router.push('/(app)/notifications')}
      style={styles.bell}
      accessibilityRole="button"
      accessibilityLabel={count > 0 ? `Notifications, ${count} unread` : 'Notifications'}
      hitSlop={8}
    >
      <Ionicons name="notifications-outline" size={24} color={theme.color.text} />
      {count > 0 ? (
        <View testID="header-bell-badge" style={styles.badge}>
          <Text style={styles.badgeText}>{count > 99 ? '99+' : count}</Text>
        </View>
      ) : null}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  bell: {
    width: 44,
    height: 44,
    alignItems: 'center',
    justifyContent: 'center',
  },
  badge: {
    position: 'absolute',
    top: 4,
    right: 4,
    minWidth: 18,
    height: 18,
    paddingHorizontal: 4,
    borderRadius: theme.radius.pill,
    backgroundColor: theme.color.danger,
    alignItems: 'center',
    justifyContent: 'center',
  },
  badgeText: {
    color: theme.color.primaryText,
    fontSize: theme.fontSize.xs,
    fontWeight: theme.fontWeight.bold,
  },
});
