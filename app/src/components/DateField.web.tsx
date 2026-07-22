import { TextField } from './TextField';
import type { DateFieldProps } from './DateField.types';

// Web DateField: a typed 'YYYY-MM-DD' field. The native calendar picker
// (@react-native-community/datetimepicker) has no web support, so web keeps the
// typed control — the value contract ('YYYY-MM-DD' | '') is identical, and the
// caller's parseOccurredAt still validates the string. maximumDate is unused here
// (validation rejects future dates on submit).
export function DateField({ label, value, onChange, placeholder, error, testID }: DateFieldProps) {
  return (
    <TextField
      label={label}
      value={value}
      onChangeText={onChange}
      placeholder={placeholder ?? 'YYYY-MM-DD — defaults to today'}
      error={error}
      autoCapitalize="none"
      autoCorrect={false}
      testID={testID}
    />
  );
}
