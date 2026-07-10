import type { ReactNode } from 'react';
import { Platform, StyleSheet, View } from 'react-native';
import type { StyleProp, ViewStyle } from 'react-native';

// Responsive content column: full-bleed on native; centered, max-width column on
// web. No breakpoint / media query — `maxWidth + width:'100%'` yields full width
// below 700px and a centered 700px column above it (intrinsic-max-width idiom),
// so it is SSR / `expo export` safe. On native the `web` style is not applied,
// leaving `flex:1, width:'100%'` full-bleed.
export const CONTENT_MAX_WIDTH = 700;

export function ScreenContainer({
  children,
  style,
  testID,
}: {
  children: ReactNode;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}) {
  return (
    <View
      testID={testID}
      style={[styles.base, Platform.OS === 'web' ? styles.web : null, style]}
    >
      {children}
    </View>
  );
}

const styles = StyleSheet.create({
  base: { flex: 1, width: '100%' },
  web: { maxWidth: CONTENT_MAX_WIDTH, width: '100%', alignSelf: 'center' },
});
