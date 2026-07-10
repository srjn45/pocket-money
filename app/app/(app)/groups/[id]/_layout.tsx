import { Stack, useLocalSearchParams } from 'expo-router';
import { View } from 'react-native';
import { LoadingSpinner } from '../../../../src/components';
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
  return (
    <Stack screenOptions={{ headerShown: true }}>
      <Stack.Screen name="index" options={{ title: group.name }} />
      <Stack.Screen name="chores" options={{ title: group.name }} />
      <Stack.Screen name="loans" options={{ title: group.name }} />
      <Stack.Screen name="members/[userId]" options={{ headerShown: true }} />
    </Stack>
  );
}
