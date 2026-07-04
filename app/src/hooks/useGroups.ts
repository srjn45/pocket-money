import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { groupsApi } from '../api';
import { qk } from '../query-keys';

export function useGroups() {
  return useQuery({
    queryKey: qk.groups(),
    queryFn: () => groupsApi.list(),
  });
}

export function useCreateGroup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { name: string }) => groupsApi.create(data),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.groups() }),
  });
}

export function useJoinGroup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (token: string) => groupsApi.join(token),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.groups() }),
  });
}
