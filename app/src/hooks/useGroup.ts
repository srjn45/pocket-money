import { useQuery } from '@tanstack/react-query';
import { groupsApi } from '../api';
import { qk } from '../query-keys';

export function useGroup(id: string) {
  return useQuery({
    queryKey: qk.group(id),
    queryFn: () => groupsApi.get(id),
    enabled: !!id,
  });
}
