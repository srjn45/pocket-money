import { useCallback } from 'react';
import { FlatList, Pressable, StyleSheet, Text, View } from 'react-native';
import { router } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import {
  ScreenContainer,
  EmptyState,
  ErrorMessage,
  LoadingSpinner,
  useToast,
} from '../../src/components';
import {
  useNotifications,
  useUnreadCount,
  useMarkNotificationRead,
  useMarkAllNotificationsRead,
} from '../../src/hooks/useNotifications';
import { formatNotification } from '../../src/notification-format';
import { theme } from '../../src/theme';
import type { Notification } from '../../src/api';

export default function NotificationsScreen() {
  const toast = useToast();
  const listQuery = useNotifications();
  const unread = useUnreadCount();
  const markRead = useMarkNotificationRead();
  const markAllRead = useMarkAllNotificationsRead();

  const items = listQuery.data?.pages.flatMap((p) => p.items) ?? [];
  const unreadCount = unread.data?.count ?? 0;

  // Robust on native, where a hidden-tab leaf may have no pop target; web uses history.
  const goBack = useCallback(() => {
    if (router.canGoBack()) router.back();
    else router.replace('/(app)');
  }, []);

  const handleMarkAll = useCallback(() => {
    markAllRead.mutate(undefined, {
      onSuccess: () => toast.show({ tone: 'success', message: 'All notifications marked read' }),
    });
  }, [markAllRead, toast]);

  // Mark-read-on-open: mark this one read (idempotent) + deep-link to its group.
  // The mutation is fire-and-forget — its invalidation lands whether or not we nav.
  const openRow = useCallback(
    (n: Notification) => {
      markRead.mutate(n.id);
      const { groupId } = formatNotification(n);
      if (groupId) router.push(`/(app)/groups/${groupId}`);
    },
    [markRead],
  );

  const renderRow = useCallback(
    ({ item }: { item: Notification }) => {
      const view = formatNotification(item);
      const isUnread = item.read_at == null;
      return (
        <Pressable
          testID={`notification-row-${item.id}`}
          onPress={() => openRow(item)}
          style={styles.row}
        >
          <Ionicons
            name={view.icon as keyof typeof Ionicons.glyphMap}
            size={24}
            color={theme.color.primary}
            style={styles.rowIcon}
          />
          <View style={styles.rowBody}>
            <Text style={styles.rowTitle}>{view.title}</Text>
            {view.body ? <Text style={styles.rowSubtitle}>{view.body}</Text> : null}
          </View>
          {isUnread ? <View testID={`notification-unread-${item.id}`} style={styles.unreadDot} /> : null}
        </Pressable>
      );
    },
    [openRow],
  );

  return (
    <ScreenContainer style={styles.screen} testID="notifications-root">
      <View style={styles.header}>
        <Pressable
          testID="notifications-back"
          onPress={goBack}
          style={styles.headerButton}
          accessibilityRole="button"
          accessibilityLabel="Back"
          hitSlop={8}
        >
          <Ionicons name="chevron-back" size={26} color={theme.color.text} />
        </Pressable>
        <Text style={styles.headerTitle}>Notifications</Text>
        {unreadCount > 0 ? (
          <Pressable
            testID="notifications-mark-all-read"
            onPress={handleMarkAll}
            style={styles.headerButton}
            accessibilityRole="button"
            accessibilityLabel="Mark all read"
            hitSlop={8}
          >
            <Text style={styles.markAllText}>Mark all read</Text>
          </Pressable>
        ) : (
          <View style={styles.headerButton} />
        )}
      </View>

      {listQuery.isLoading ? (
        <LoadingSpinner />
      ) : listQuery.isError ? (
        <View testID="notifications-error" style={styles.stateFill}>
          <ErrorMessage
            message={listQuery.error instanceof Error ? listQuery.error.message : 'Failed to load notifications'}
            onRetry={() => listQuery.refetch()}
          />
        </View>
      ) : items.length === 0 ? (
        <View testID="notifications-empty" style={styles.stateFill}>
          <EmptyState icon="notifications-outline" title="No notifications yet." />
        </View>
      ) : (
        <FlatList
          data={items}
          keyExtractor={(item) => item.id}
          renderItem={renderRow}
          contentContainerStyle={styles.list}
          onEndReachedThreshold={0.5}
          onEndReached={() => {
            if (listQuery.hasNextPage && !listQuery.isFetchingNextPage) listQuery.fetchNextPage();
          }}
          ListFooterComponent={listQuery.isFetchingNextPage ? <LoadingSpinner /> : null}
        />
      )}
    </ScreenContainer>
  );
}

const styles = StyleSheet.create({
  screen: {
    flex: 1,
    backgroundColor: theme.color.background,
  },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: theme.spacing.sm,
    paddingVertical: theme.spacing.md,
    backgroundColor: theme.color.surface,
    borderBottomWidth: 1,
    borderBottomColor: theme.color.border,
  },
  headerButton: {
    minWidth: 44,
    minHeight: 44,
    alignItems: 'center',
    justifyContent: 'center',
    paddingHorizontal: theme.spacing.sm,
  },
  headerTitle: {
    flex: 1,
    textAlign: 'center',
    fontSize: theme.fontSize.lg,
    fontWeight: theme.fontWeight.bold,
    color: theme.color.text,
  },
  markAllText: {
    fontSize: theme.fontSize.sm,
    fontWeight: theme.fontWeight.semibold,
    color: theme.color.primary,
  },
  stateFill: {
    flex: 1,
  },
  list: {
    padding: theme.spacing.lg,
    gap: theme.spacing.md,
  },
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: theme.spacing.md,
    borderRadius: theme.radius.md,
    backgroundColor: theme.color.surface,
    borderWidth: 1,
    borderColor: theme.color.border,
  },
  rowIcon: {
    marginRight: theme.spacing.md,
  },
  rowBody: {
    flex: 1,
    gap: 2,
  },
  rowTitle: {
    fontSize: theme.fontSize.md,
    fontWeight: theme.fontWeight.semibold,
    color: theme.color.text,
  },
  rowSubtitle: {
    fontSize: theme.fontSize.sm,
    color: theme.color.textSecondary,
  },
  unreadDot: {
    width: 10,
    height: 10,
    borderRadius: theme.radius.pill,
    backgroundColor: theme.color.primary,
    marginLeft: theme.spacing.sm,
  },
});
