import { Pressable, StyleSheet } from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { router } from 'expo-router';
import { theme } from '../theme';

/**
 * Explicit header back control for the group screens (QA batch 1, Item 3).
 *
 * Root cause it fixes: the group detail screens are the *root* screen of their own
 * nested Stack (`groups/[id]/_layout`). The native-stack default back chevron on a
 * root screen calls that navigator's scoped `goBack()`, which has nothing to pop in
 * its OWN stack — so it renders (history exists at an outer navigator) but does
 * nothing. This control instead uses expo-router's global `router.back()` (which
 * pops the real history across navigators) and falls back to an explicit replace to
 * the dashboard when there is no history to pop (e.g. a deep link / web refresh),
 * so the button always navigates somewhere sensible.
 */
export function HeaderBackButton() {
  return (
    <Pressable
      testID="header-back-button"
      onPress={() => {
        if (router.canGoBack()) {
          router.back();
        } else {
          router.replace('/(app)' as never);
        }
      }}
      style={styles.button}
      accessibilityRole="button"
      accessibilityLabel="Back"
      hitSlop={8}
    >
      <Ionicons name="chevron-back" size={26} color={theme.color.text} />
    </Pressable>
  );
}

const styles = StyleSheet.create({
  button: {
    width: 44,
    height: 44,
    alignItems: 'center',
    justifyContent: 'center',
    // Pull the chevron toward the screen edge so it sits where a native back
    // button would, not indented by the 44pt touch target.
    marginLeft: -8,
  },
});
