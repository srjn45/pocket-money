import { StyleSheet, Text, TextInput, View } from 'react-native';
import type { TextInputProps, ViewStyle } from 'react-native';
import { theme } from '../theme';

interface TextFieldProps extends Omit<TextInputProps, 'style'> {
  label?: string;
  /** Inline error text shown below the input; also turns the border danger-colored. */
  error?: string;
  containerStyle?: ViewStyle;
}

export function TextField({ label, error, containerStyle, ...inputProps }: TextFieldProps) {
  return (
    <View style={[styles.container, containerStyle]}>
      {label ? <Text style={styles.label}>{label}</Text> : null}
      <TextInput
        style={[styles.input, error ? styles.inputError : null]}
        placeholderTextColor={theme.color.textMuted}
        {...inputProps}
      />
      {error ? <Text style={styles.error}>{error}</Text> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    marginBottom: theme.spacing.md,
  },
  label: {
    fontSize: theme.fontSize.sm,
    fontWeight: theme.fontWeight.medium,
    color: theme.color.textSecondary,
    marginBottom: theme.spacing.xs,
  },
  input: {
    borderWidth: 1,
    borderColor: theme.color.border,
    borderRadius: theme.radius.sm,
    padding: theme.spacing.md,
    fontSize: theme.fontSize.md,
    color: theme.color.text,
    backgroundColor: theme.color.surface,
  },
  inputError: {
    borderColor: theme.color.danger,
  },
  error: {
    fontSize: theme.fontSize.xs,
    color: theme.color.danger,
    marginTop: theme.spacing.xs,
  },
});
