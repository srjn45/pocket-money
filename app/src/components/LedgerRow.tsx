import { ActivityIndicator, StyleSheet, Text, View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { theme } from '../theme';
import type { LedgerEntry, Chore, Member } from '../api';
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

export interface LedgerRowProps {
  entry: LedgerEntry;
  chores: Chore[];
  members: Member[];
  isHead: boolean;
  onApprove?: (id: string) => void;
  onReject?: (id: string) => void;
  processing?: boolean;
}

export function LedgerRow({
  entry,
  chores,
  members,
  isHead,
  onApprove,
  onReject,
  processing = false,
}: LedgerRowProps) {
  const isPending = entry.status === 'pending_approval';
  const isRejected = entry.status === 'rejected';
  const icon = TYPE_ICONS[entry.entry_type] ?? 'ellipse-outline';
  const title = entryTitle(entry, chores);
  const dateStr = new Date(entry.created_at).toLocaleDateString();
  const subtitle = entry.note ? `${dateStr} · ${entry.note}` : dateStr;
  const caption = decidedByCaption(entry, members);

  const rightSlot = (
    <View style={styles.rightCol}>
      <AmountText
        minorUnits={entry.amount}
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
              />
              <Button
                title="✕"
                variant="danger"
                size="sm"
                onPress={() => onReject?.(entry.id)}
                disabled={processing}
              />
            </>
          )}
        </View>
      )}
      {isRejected && <StatusBadge label="Rejected" tone="danger" />}
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
