import { ActivityIndicator, StyleSheet, Text, View } from 'react-native';
import { theme } from '../theme';

interface LoadingSpinnerProps {
  message?: string;
}

export function LoadingSpinner({ message = 'Loading...' }: LoadingSpinnerProps) {
  return (
    <View style={styles.container}>
      <ActivityIndicator size="large" color={theme.color.primary} />
      <Text style={styles.text}>{message}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: theme.color.surface,
  },
  text: {
    marginTop: theme.spacing.lg,
    fontSize: theme.fontSize.md,
    color: theme.color.textSecondary,
  },
});
