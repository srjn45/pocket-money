import { Stack, useLocalSearchParams } from 'expo-router';
import { View } from 'react-native';
import { HeaderBackButton, LoadingSpinner } from '../../../../src/components';
import { useGroup } from '../../../../src/hooks/useGroup';

export default function GroupDetailLayout() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { data: group, isLoading } = useGroup(id ?? '');

  if (isLoading || !group) {
    return (
      <View style={{ flex: 1, justifyContent: 'center', alignItems: 'center' }}>
        <LoadingSpinner />
      </View>
    );
  }

  // Stack (not Tabs) → ONE header (title = group name) shared by all three
  // sections. The second bottom bar is gone; Overview/Chores/Loans are switched
  // by <GroupSectionTabs/> (a segmented control, not a navigator). members/[userId]
  // drills in and sets its own "{name}'s ledger" title.
  //
  // The three SECTION screens (index/chores/loans) are the ROOT of this stack, so
  // the native back chevron there would call this navigator's scoped goBack() with
  // nothing to pop — a visible-but-inert button (QA batch 1, Item 3). We give them
  // an explicit <HeaderBackButton/> that uses router.back()/replace to reliably
  // return to the dashboard. The drill-in screens (loans/[loanId], members/[userId])
  // DO have real history in this stack, so their native back correctly pops to the
  // section and is left untouched.
  return (
    <Stack screenOptions={{ headerShown: true }}>
      <Stack.Screen name="index" options={{ title: group.name, headerLeft: () => <HeaderBackButton /> }} />
      <Stack.Screen name="chores" options={{ title: group.name, headerLeft: () => <HeaderBackButton /> }} />
      <Stack.Screen name="loans/index" options={{ title: group.name, headerLeft: () => <HeaderBackButton /> }} />
      <Stack.Screen name="loans/[loanId]" options={{ headerShown: true }} />
      <Stack.Screen name="members/[userId]" options={{ headerShown: true }} />
    </Stack>
  );
}
