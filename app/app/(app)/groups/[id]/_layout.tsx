import { Tabs, useLocalSearchParams } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { useState, useEffect } from 'react';
import { groupsApi, GroupDetail, Member } from '../../../../src/api';
import { useAuth } from '../../../../src/auth-context';

export default function GroupDetailLayout() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { user } = useAuth();
  const [group, setGroup] = useState<GroupDetail | null>(null);
  const [isHead, setIsHead] = useState(false);

  useEffect(() => {
    const loadGroup = async () => {
      if (!id) return;
      try {
        const data = await groupsApi.get(id);
        setGroup(data);
        const currentMember = data.members.find((m: Member) => m.user_id === user?.id);
        setIsHead(currentMember?.role === 'head');
      } catch (error) {
        console.error('Failed to load group:', error);
      }
    };
    loadGroup();
  }, [id, user?.id]);

  return (
    <Tabs screenOptions={{ headerShown: true }}>
      <Tabs.Screen
        name="index"
        options={{
          title: group?.name || 'Overview',
          tabBarIcon: ({ color, size }) => (
            <Ionicons name="home-outline" size={size} color={color} />
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
        name="ledger"
        options={{
          title: 'Ledger',
          tabBarIcon: ({ color, size }) => (
            <Ionicons name="wallet-outline" size={size} color={color} />
          ),
        }}
      />
      <Tabs.Screen
        name="settlements"
        options={{
          title: 'Settlements',
          tabBarIcon: ({ color, size }) => (
            <Ionicons name="cash-outline" size={size} color={color} />
          ),
        }}
      />
      {isHead && (
        <Tabs.Screen
          name="pending"
          options={{
            title: 'Pending',
            tabBarIcon: ({ color, size }) => (
              <Ionicons name="time-outline" size={size} color={color} />
            ),
          }}
        />
      )}
    </Tabs>
  );
}
