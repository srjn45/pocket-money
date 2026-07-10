import { ScrollView, StyleSheet, View } from 'react-native';
import { Stack, useLocalSearchParams } from 'expo-router';
import { useAuth } from '../../../../../src/auth-context';
import { useGroup } from '../../../../../src/hooks/useGroup';
import { useLoans } from '../../../../../src/hooks/useLoans';
import {
  LoanCard,
  LoanScheduleList,
  LoadingSpinner,
  EmptyState,
  ScreenContainer,
} from '../../../../../src/components';
import { theme } from '../../../../../src/theme';

// Read-only loan detail: header card + derived repayment schedule (§2.3). Both
// [id] and [loanId] come from the URL (2-param web-param discipline). There is no
// by-id loan endpoint — the loan is resolved from the list query.
export default function LoanDetailScreen() {
  const { id, loanId } = useLocalSearchParams<{ id: string; loanId: string }>();
  const gid = id ?? '';
  const lid = loanId ?? '';
  const { user } = useAuth();

  const loansQuery = useLoans(gid);
  const groupQuery = useGroup(gid);

  const group = groupQuery.data;
  const currency = group?.currency ?? 'INR';
  const loan = (loansQuery.data ?? []).find(l => l.id === lid);
  const memberName = group?.members.find(m => m.user_id === loan?.user_id)?.name;
  const isHead = group?.members.find(m => m.user_id === user?.id)?.role === 'admin';

  if (loansQuery.isLoading || groupQuery.isLoading) {
    return (
      <>
        <Stack.Screen options={{ title: 'Loan' }} />
        <View style={styles.centered}><LoadingSpinner /></View>
      </>
    );
  }

  if (!loan) {
    return (
      <>
        <Stack.Screen options={{ title: 'Loan' }} />
        <ScreenContainer style={styles.screen} testID="loan-detail-root">
          <EmptyState
            icon="card-outline"
            title="Loan not found"
            subtitle="This loan may have been removed."
          />
        </ScreenContainer>
      </>
    );
  }

  return (
    <>
      <Stack.Screen options={{ title: 'Loan' }} />
      <ScreenContainer style={styles.screen} testID="loan-detail-root">
        <ScrollView contentContainerStyle={styles.content}>
          <LoanCard
            loan={loan}
            memberName={isHead ? memberName : undefined}
            currency={currency}
          />
          <LoanScheduleList loan={loan} currency={currency} />
        </ScrollView>
      </ScreenContainer>
    </>
  );
}

const styles = StyleSheet.create({
  screen: {
    flex: 1,
    backgroundColor: theme.color.background,
  },
  centered: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
  },
  content: {
    padding: theme.spacing.lg,
  },
});
