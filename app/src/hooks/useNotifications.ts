import { useInfiniteQuery, useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { notificationsApi } from '../api';
import { qk } from '../query-keys';

// Paged list, newest first. Pages flatten in the screen via data.pages.flatMap(p => p.items).
export function useNotifications() {
  return useInfiniteQuery({
    queryKey: qk.notifications(),
    queryFn: ({ pageParam }) => notificationsApi.list(pageParam),
    initialPageParam: null as string | null,
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
}

// Badge source. staleTime kept modest so re-entering the app / a screen refocus
// re-reads it; also invalidated explicitly by the mark mutations below.
export function useUnreadCount() {
  return useQuery({
    queryKey: qk.unreadCount(),
    queryFn: () => notificationsApi.unreadCount(),
    staleTime: 30_000,
  });
}

export function useMarkNotificationRead() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => notificationsApi.markRead(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.notifications() });
      qc.invalidateQueries({ queryKey: qk.unreadCount() });
    },
  });
}

export function useMarkAllNotificationsRead() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => notificationsApi.markAllRead(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.notifications() });
      qc.invalidateQueries({ queryKey: qk.unreadCount() });
    },
  });
}
