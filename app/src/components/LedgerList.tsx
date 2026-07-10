import { RefreshControl, SectionList, StyleSheet, View } from 'react-native';
import { theme } from '../theme';
import type { LedgerEntry, Chore, Member, Loan, CurrencyCode } from '../api';
import { groupEntriesByMonth, type MonthGroup } from '../ledger-format';
import { LedgerRow } from './LedgerRow';
import { MonthHeader } from './MonthHeader';
import { EmptyState } from './EmptyState';

export interface LedgerListProps {
  entries: LedgerEntry[];
  chores: Chore[];
  members: Member[];
  isHead: boolean;
  groupId: string;
  /** Group currency, for month-total formatting. */
  currency: CurrencyCode;
  onApprove?: (id: string) => void;
  onReject?: (id: string) => void;
  processingId?: string | null;
  refreshing?: boolean;
  onRefresh?: () => void;
  emptyTitle: string;
  emptySubtitle?: string;
  /** Optional loans so EMI rows can render "EMI k/n"; omit for the plain fallback. */
  loans?: Loan[];
}

export function LedgerList({
  entries,
  chores,
  members,
  isHead,
  currency,
  onApprove,
  onReject,
  processingId,
  refreshing = false,
  onRefresh,
  emptyTitle,
  emptySubtitle,
  loans,
}: LedgerListProps) {
  const sections = groupEntriesByMonth(entries);

  if (sections.length === 0) {
    return (
      <EmptyState
        icon="wallet-outline"
        title={emptyTitle}
        subtitle={emptySubtitle}
      />
    );
  }

  return (
    <SectionList<LedgerEntry, MonthGroup>
      sections={sections}
      keyExtractor={(item) => item.id}
      renderItem={({ item }) => (
        <LedgerRow
          entry={item}
          chores={chores}
          members={members}
          isHead={isHead}
          onApprove={onApprove}
          onReject={onReject}
          processing={processingId === item.id}
          loans={loans}
        />
      )}
      renderSectionHeader={({ section }) => (
        <MonthHeader
          period={section.period}
          totalMinorUnits={section.monthTotal}
          currency={currency}
          totalVariant={section.monthTotal < 0 ? 'debit' : 'credit'}
        />
      )}
      ItemSeparatorComponent={() => <View style={styles.separator} />}
      refreshControl={
        onRefresh ? (
          <RefreshControl refreshing={refreshing} onRefresh={onRefresh} />
        ) : undefined
      }
      stickySectionHeadersEnabled={false}
    />
  );
}

const styles = StyleSheet.create({
  separator: {
    height: StyleSheet.hairlineWidth,
    backgroundColor: theme.color.border,
  },
});
