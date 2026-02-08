import { Tabs, useLocalSearchParams, useRouter, Redirect } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { useState, useEffect } from 'react';
import { View, ActivityIndicator, StyleSheet } from 'react-native';
import { groupsApi, GroupDetail, Member } from '../../../../src/api';
import { useAuth } from '../../../../src/auth-context';

export default function GroupDetailLayout() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { user } = useAuth();
  const [group, setGroup] = useState<GroupDetail | null>(null);
  const [isHead, setIsHead] = useState<boolean | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const loadGroup = async () => {
      if (!id || !user?.id) {
        return;
      }
      setIsLoading(true);
      try {
        const data = await groupsApi.get(id);
        setGroup(data);
        const currentMember = data.members.find((m: Member) => m.user_id === user?.id);
        const headStatus = currentMember?.role === 'head';
        setIsHead(headStatus);
      } catch (error) {
        console.error('Failed to load group:', error);
      } finally {
        setIsLoading(false);
      }
    };
    loadGroup();
  }, [id, user?.id]);

  // Show loading while fetching group data or if role hasn't been determined yet
  if (isLoading || isHead === null) {
    return (
      <View style={styles.loading}>
        <ActivityIndicator size="large" color="#007AFF" />
      </View>
    );
  }

  // Member user: Ledger (primary) + Chores (read-only)
  if (isHead === false) {
    return (
      <Tabs screenOptions={{ headerShown: true }}>
        <Tabs.Screen
          name="ledger"
          options={{
            title: group?.name || 'Ledger',
            tabBarIcon: ({ color, size }) => (
              <Ionicons name="wallet-outline" size={size} color={color} />
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
          name="index"
          options={{
            href: null, // Hide from navigation
          }}
        />
        <Tabs.Screen
          name="settlements"
          options={{
            href: null, // Hide from navigation
          }}
        />
        <Tabs.Screen
          name="pending"
          options={{
            href: null, // Hide from navigation
          }}
        />
      </Tabs>
    );
  }

  // Head user: Overview + Chores tabs
  return (
    <Tabs screenOptions={{ headerShown: true }}>
      <Tabs.Screen
        name="index"
        options={{
          title: group?.name || 'Overview',
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
        name="ledger"
        options={{
          title: 'Ledger',
          href: null, // Hide from tab bar but still accessible via navigation
        }}
      />
      <Tabs.Screen
        name="settlements"
        options={{
          href: null, // Hide from navigation
        }}
      />
      <Tabs.Screen
        name="pending"
        options={{
          href: null, // Hide from navigation
        }}
      />
    </Tabs>
  );
}

const styles = StyleSheet.create({
  loading: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: '#f5f5f5',
  },
});
