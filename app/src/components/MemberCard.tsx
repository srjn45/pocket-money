import { StyleSheet, Text, View } from 'react-native';
import { theme } from '../theme';
import type { Balance, Member } from '../api';
import { Card } from './Card';
import { Avatar } from './Avatar';
import { AmountText } from './AmountText';
import { StatusBadge } from './StatusBadge';

interface MemberCardProps {
  balance: Balance;
  member: Member;
  pendingCount: number;
  onPress: () => void;
}

export function MemberCard({ balance, member, pendingCount, onPress }: MemberCardProps) {
  return (
    <Card onPress={onPress} padded={false} style={styles.card}>
      <View style={styles.row}>
        <Avatar name={member.name} id={member.user_id} size={44} />
        <View style={styles.nameCol}>
          <Text style={styles.name}>{member.name}</Text>
          {pendingCount > 0 && (
            <StatusBadge label={`${pendingCount} pending`} tone="warning" />
          )}
        </View>
        <View style={styles.amountCol}>
          <AmountText
            minorUnits={balance.balance}
            variant={balance.balance < 0 ? 'debit' : 'credit'}
          />
          <Text style={styles.hint}>
            {balance.balance < 0 ? 'owes you' : 'owed'}
          </Text>
        </View>
      </View>
    </Card>
  );
}

const styles = StyleSheet.create({
  card: {
    marginHorizontal: theme.spacing.lg,
    marginBottom: theme.spacing.sm,
  },
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: theme.spacing.md,
    padding: theme.spacing.lg,
  },
  nameCol: {
    flex: 1,
    gap: theme.spacing.xs,
  },
  name: {
    fontSize: theme.fontSize.md,
    fontWeight: theme.fontWeight.semibold,
    color: theme.color.text,
  },
  amountCol: {
    alignItems: 'flex-end',
  },
  hint: {
    fontSize: theme.fontSize.xs,
    color: theme.color.textSecondary,
    marginTop: theme.spacing.xs,
  },
});
