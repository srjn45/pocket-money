import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { choresApi } from '../api';
import { qk } from '../query-keys';

export function useChores(groupId: string) {
  return useQuery({
    queryKey: qk.chores(groupId),
    queryFn: () => choresApi.list(groupId),
    enabled: !!groupId,
  });
}

export function useCreateChore(groupId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { name: string; description?: string; amount: number }) =>
      choresApi.create(groupId, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.chores(groupId) });
      qc.invalidateQueries({ queryKey: qk.group(groupId) });
    },
  });
}

export function useUpdateChore(groupId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, ...data }: { id: string; name?: string; description?: string; amount?: number }) =>
      choresApi.update(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.chores(groupId) }),
  });
}

export function useDeleteChore(groupId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => choresApi.delete(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.chores(groupId) });
      qc.invalidateQueries({ queryKey: qk.group(groupId) });
    },
  });
}
