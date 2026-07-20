import { Tabs } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';

export default function TabsLayout() {
  return (
    // headerShown:false → these Tabs are the SOLE nav chrome (bottom bar on
    // native / top nav on web) with NO tab-level header. Dashboard/Profile own
    // their one meaningful header in-body. Only the two primary destinations live
    // here; group detail and notifications are push-in Stack screens on the parent
    // (app) navigator (see ../_layout.tsx), so entering a group is a genuine Stack
    // push/pop rather than a Tab focus/blur — the latter never hid the inactive
    // screen on the web build, which leaked stale group content across visits.
    <Tabs screenOptions={{ headerShown: false }}>
      <Tabs.Screen
        name="index"
        options={{
          title: 'Dashboard',
          tabBarIcon: ({ color, size }) => (
            <Ionicons name="home" size={size} color={color} />
          ),
        }}
      />
      <Tabs.Screen
        name="profile"
        options={{
          title: 'Profile',
          tabBarIcon: ({ color, size }) => (
            <Ionicons name="person" size={size} color={color} />
          ),
        }}
      />
    </Tabs>
  );
}
