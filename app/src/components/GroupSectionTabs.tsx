import { Pressable, StyleSheet, Text, View } from 'react-native';
import { router } from 'expo-router';
import { theme } from '../theme';

export type GroupSection = 'overview' | 'chores' | 'loans';

// Section switch for a group's Overview / Chores / Loans screens. It is NOT a
// navigator — the three sections are sibling Stack routes under one group header,
// so this segmented control replaces the second bottom bar the old inner Tabs
// rendered. Navigation MUST use absolute id-bearing URLs (never relative sibling
// names or {pathname, params}) or the child's useLocalSearchParams().id is empty
// on web and group-scoped queries fetch /groups// … (expo-router web-param
// gotcha — the same reason e2e reaches these routes via page.goto). `replace`
// (not `push`) keeps section switches off the history stack, matching tab
// semantics.
const SECTIONS: { key: GroupSection; label: string; path: (id: string) => string }[] = [
  { key: 'overview', label: 'Overview', path: (id) => `/(app)/groups/${id}` },
  { key: 'chores', label: 'Chores', path: (id) => `/(app)/groups/${id}/chores` },
  { key: 'loans', label: 'Loans', path: (id) => `/(app)/groups/${id}/loans` },
];

export function GroupSectionTabs({
  groupId,
  active,
}: {
  groupId: string;
  active: GroupSection;
}) {
  return (
    <View style={styles.container}>
      {SECTIONS.map((section) => {
        const isActive = section.key === active;
        return (
          <Pressable
            key={section.key}
            style={[styles.segment, isActive && styles.segmentActive]}
            onPress={() => {
              if (!isActive) router.replace(section.path(groupId) as never);
            }}
            accessibilityRole="tab"
            accessibilityState={{ selected: isActive }}
          >
            <Text style={[styles.label, isActive && styles.labelActive]}>
              {section.label}
            </Text>
          </Pressable>
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flexDirection: 'row',
    backgroundColor: theme.color.surface,
    borderBottomWidth: 1,
    borderBottomColor: theme.color.border,
  },
  segment: {
    flex: 1,
    minHeight: 44, // ≥44pt touch target (§5)
    alignItems: 'center',
    justifyContent: 'center',
    paddingVertical: theme.spacing.sm,
    borderBottomWidth: 2,
    borderBottomColor: 'transparent',
  },
  segmentActive: {
    backgroundColor: theme.color.primaryMuted,
    borderBottomColor: theme.color.primary,
  },
  label: {
    fontSize: theme.fontSize.sm,
    fontWeight: theme.fontWeight.medium,
    color: theme.color.textSecondary,
  },
  labelActive: {
    color: theme.color.primary,
    fontWeight: theme.fontWeight.semibold,
  },
});
