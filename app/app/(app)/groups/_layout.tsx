import { Stack } from 'expo-router';

export default function GroupsLayout() {
  return (
    <Stack>
      <Stack.Screen name="index" options={{ title: 'Groups' }} />
      <Stack.Screen name="create" options={{ title: 'Create Group' }} />
      <Stack.Screen name="[id]" options={{ headerShown: false }} />
    </Stack>
  );
}
