import { ActivityIndicator, StyleSheet, Text, View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { theme } from '../theme';
import type { LedgerEntry, Chore, Member, Loan } from '../api';
import { entryTitle, decidedByCaption } from '../ledger-format';
import { ListRow } from './ListRow';
import { AmountText } from './AmountText';
import { StatusBadge } from './StatusBadge';
import { Button } from './Button';

const TYPE_ICONS: Record<string, keyof typeof Ionicons.glyphMap> = {
  chore:      'checkmark-circle-outline',
  allowance:  'cash-outline',
  emi:        'card-outline',
  settlement: 'arrow-up-circle-outline',
  adjustment: 'create-outline',
};

/** Manual entry types (D3) — the only rows an admin may edit/delete. */
const MANUAL_TYPES = new Set(['chore', 'settlement', 'adjustment']);

export interface LedgerRowProps {
  entry: LedgerEntry;
  chores: Chore[];
  members: Member[];
  isHead: boolean;
  onApprove?: (id: string) => void;
  onReject?: (id: string) => void;
  processing?: boolean;
  /** Optional loans so EMI rows can render "EMI k/n"; omit for the plain fallback. */
  loans?: Loan[];
  /** Corrections (D3): admin-only edit/delete on manual rows. */
  canEdit?: boolean;
  onEdit?: (entry: LedgerEntry) => void;
  onDelete?: (entry: LedgerEntry) => void;
  /** Session-local "Edited" badge (no durable API field — §4.3). */
  edited?: boolean;
}

export function LedgerRow({
  entry,
  chores,
  members,
  isHead,
  onApprove,
  onReject,
  processing = false,
  loans,
  canEdit = false,
  onEdit,
  onDelete,
  edited = false,
}: LedgerRowProps) {
  const isPending = entry.status === 'pending_approval';
  const isRejected = entry.status === 'rejected';
  // Manual, non-rejected rows are correctable by an admin (backend enforces the
  // same — PUT/DELETE 403 for allowance/emi and non-admins).
  const canCorrect = canEdit && isHead && !isRejected && MANUAL_TYPES.has(entry.entry_type);
  const icon = TYPE_ICONS[entry.entry_type] ?? 'ellipse-outline';
  const title = entryTitle(entry, chores, loans);
  const dateStr = new Date(entry.created_at).toLocaleDateString();
  const subtitle = entry.note ? `${dateStr} · ${entry.note}` : dateStr;
  const caption = decidedByCaption(entry, members);

  const rightSlot = (
    <View style={styles.rightCol}>
      <AmountText
        minorUnits={entry.amount.value}
        currency={entry.amount.currency}
        variant={entry.direction === 'credit' ? 'credit' : 'debit'}
        size="sm"
      />
      {isPending && !isHead && (
        <StatusBadge label="Pending" tone="warning" />
      )}
      {isPending && isHead && (
        <View style={styles.actions}>
          {processing ? (
            <ActivityIndicator size="small" color={theme.color.primary} />
          ) : (
            <>
              <Button
                title="✓"
                variant="secondary"
                size="sm"
                onPress={() => onApprove?.(entry.id)}
                disabled={processing}
                testID={`ledger-approve-${entry.id}`}
              />
              <Button
                title="✕"
                variant="danger"
                size="sm"
                onPress={() => onReject?.(entry.id)}
                disabled={processing}
                testID={`ledger-reject-${entry.id}`}
              />
            </>
          )}
        </View>
      )}
      {isRejected && <StatusBadge label="Rejected" tone="danger" />}
      {edited && (
        <View testID={`ledger-edited-${entry.id}`}>
          <StatusBadge label="Edited" tone="neutral" />
        </View>
      )}
      {canCorrect && (
        <View style={styles.actions}>
          <Button
            title=""
            variant="ghost"
            size="sm"
            icon="pencil-outline"
            onPress={() => onEdit?.(entry)}
            testID={`ledger-edit-${entry.id}`}
          />
          <Button
            title=""
            variant="ghost"
            size="sm"
            icon="trash-outline"
            onPress={() => onDelete?.(entry)}
            testID={`ledger-delete-${entry.id}`}
          />
        </View>
      )}
    </View>
  );

  return (
    <View>
      <ListRow
        left={
          <Ionicons
            name={icon}
            size={20}
            color={theme.color.textSecondary}
          />
        }
        title={title}
        subtitle={subtitle}
        strikethrough={isRejected}
        right={rightSlot}
      />
      {caption ? (
        <Text style={styles.decidedCaption}>{caption}</Text>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  rightCol: {
    alignItems: 'flex-end',
    gap: theme.spacing.xs,
  },
  actions: {
    flexDirection: 'row',
    gap: theme.spacing.xs,
    marginTop: theme.spacing.xs,
  },
  decidedCaption: {
    fontSize: theme.fontSize.xs,
    color: theme.color.textSecondary,
    paddingHorizontal: theme.spacing.lg,
    paddingBottom: theme.spacing.sm,
    paddingLeft: theme.spacing.lg + 20 + theme.spacing.md,
  },
});
