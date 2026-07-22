import { RefreshControl, ScrollView, SectionList, StyleSheet, View } from 'react-native';
import type { ReactElement } from 'react';
import type { StyleProp, ViewStyle } from 'react-native';
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
  /** Corrections (D3): thread admin edit/delete + session "Edited" badge to rows. */
  canEdit?: boolean;
  onEditEntry?: (entry: LedgerEntry) => void;
  onDeleteEntry?: (entry: LedgerEntry) => void;
  /** Entry ids that were edited this session (in-memory, §4.3). */
  editedIds?: Set<string>;
  /**
   * Optional content rendered above/below the entries and scrolled together
   * with the list — lets a screen make its whole body scrollable through this
   * single list (e.g. the member ledger header + remove-member footer) instead
   * of stacking a fixed header over a separately-scrolling list.
   */
  ListHeaderComponent?: ReactElement | null;
  ListFooterComponent?: ReactElement | null;
  /** Style for the list container (e.g. flex:1 to fill the screen). */
  style?: StyleProp<ViewStyle>;
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
  canEdit,
  onEditEntry,
  onDeleteEntry,
  editedIds,
  ListHeaderComponent,
  ListFooterComponent,
  style,
}: LedgerListProps) {
  const sections = groupEntriesByMonth(entries);

  const emptyState = (
    <EmptyState icon="wallet-outline" title={emptyTitle} subtitle={emptySubtitle} />
  );

  if (sections.length === 0) {
    // With no header/footer, keep the bare centered empty state (other callers).
    // With them, scroll header + empty + footer together so the screen still scrolls.
    if (!ListHeaderComponent && !ListFooterComponent) return emptyState;
    return (
      <ScrollView
        style={style}
        contentContainerStyle={styles.emptyScroll}
        refreshControl={
          onRefresh ? (
            <RefreshControl refreshing={refreshing} onRefresh={onRefresh} />
          ) : undefined
        }
      >
        {ListHeaderComponent}
        {emptyState}
        {ListFooterComponent}
      </ScrollView>
    );
  }

  return (
    <SectionList<LedgerEntry, MonthGroup>
      style={style}
      sections={sections}
      keyExtractor={(item) => item.id}
      ListHeaderComponent={ListHeaderComponent}
      ListFooterComponent={ListFooterComponent}
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
          canEdit={canEdit}
          onEdit={onEditEntry}
          onDelete={onDeleteEntry}
          edited={editedIds?.has(item.id)}
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
  emptyScroll: {
    flexGrow: 1,
  },
});
