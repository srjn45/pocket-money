import { useQuery } from '@tanstack/react-query';
import { statementApi } from '../api';
import { qk } from '../query-keys';

/**
 * The group's monthly statement for `period` ('YYYY-MM'). The endpoint triggers
 * PostDue, so opening a statement materializes that month's allowance/EMI (the
 * dashboard's GET /groups deliberately does not — V3-4.2 §4.2). Keyed by
 * (groupId, period); `period` is screen-local state, never a route param, to
 * keep the expo-router web-param gotcha off the fetch.
 */
export function useStatement(groupId: string, period: string) {
  return useQuery({
    queryKey: qk.statement(groupId, period),
    queryFn: () => statementApi.get(groupId, period),
    enabled: !!groupId,
  });
}
