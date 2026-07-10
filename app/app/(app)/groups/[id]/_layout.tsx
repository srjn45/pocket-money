import { Tabs, useLocalSearchParams } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { View } from 'react-native';
import { LoadingSpinner } from '../../../../src/components';
import { useGroup } from '../../../../src/hooks/useGroup';
import { useAuth } from '../../../../src/auth-context';

export default function GroupDetailLayout() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { user } = useAuth();
  const { data: group, isLoading } = useGroup(id ?? '');

  if (isLoading || !group) {
    return (
      <View style={{ flex: 1, justifyContent: 'center', alignItems: 'center' }}>
        <LoadingSpinner />
      </View>
    );
  }

  const isHead = group.members.find(m => m.user_id === user?.id)?.role === 'admin';
  const overviewTitle = isHead ? group.name : (group.name + ' — Overview');

  return (
    <Tabs screenOptions={{ headerShown: true }}>
      <Tabs.Screen
        name="index"
        options={{
          title: overviewTitle,
          tabBarIcon: ({ color, size }) => (
            <Ionicons name="people-outline" size={size} color={color} />
          ),
        }}
      />
      <Tabs.Screen
        name="chores"
        options={{
          title: 'Chores',
          tabBarIcon: ({ color, size }) => (
            <Ionicons name="list-outline" size={size} color={color} />
          ),
        }}
      />
      <Tabs.Screen
        name="loans"
        options={{
          title: 'Loans',
          tabBarIcon: ({ color, size }) => (
            <Ionicons name="card-outline" size={size} color={color} />
          ),
        }}
      />
      <Tabs.Screen
        name="members/[userId]"
        options={{
          href: null,
        }}
      />
    </Tabs>
  );
}
