import { useState } from 'react';
import { Platform, Pressable, StyleSheet, Text, View } from 'react-native';
import DateTimePicker from '@react-native-community/datetimepicker';
import type { DateTimePickerEvent } from '@react-native-community/datetimepicker';
import { theme } from '../theme';
import { Button } from './Button';
import { ymdToDate, dateToYmd } from './DateField.types';
import type { DateFieldProps } from './DateField.types';

// Native (iOS/Android) date field: a tappable control that opens the OS calendar
// picker. The web build resolves DateField.web.tsx instead (the picker has no web
// support), so this module — and the native module it imports — is never bundled
// for web. Value contract is 'YYYY-MM-DD' | '' (see DateField.types).
export function DateField({
  label,
  value,
  onChange,
  placeholder,
  error,
  maximumDate,
  testID,
}: DateFieldProps) {
  const [show, setShow] = useState(false);
  const selected = ymdToDate(value);
  // Seed the picker at the current value, else the max (usually today), else now.
  const pickerValue = selected ?? maximumDate ?? new Date();

  const handleChange = (event: DateTimePickerEvent, date?: Date) => {
    // Android renders a one-shot dialog; close it on any result. iOS renders an
    // inline picker we dismiss via the Done button below (or on explicit cancel).
    if (Platform.OS !== 'ios') setShow(false);
    if (event.type === 'dismissed') {
      setShow(false);
      return;
    }
    if (event.type === 'set' && date) onChange(dateToYmd(date));
  };

  const displayText = selected
    ? selected.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
    : placeholder ?? 'Select a date';

  return (
    <View style={styles.container}>
      {label ? <Text style={styles.label}>{label}</Text> : null}
      <View style={styles.row}>
        <Pressable
          style={[styles.input, error ? styles.inputError : null]}
          onPress={() => setShow(true)}
          testID={testID}
          accessibilityRole="button"
          accessibilityLabel={label ?? 'Select date'}
        >
          <Text style={selected ? styles.valueText : styles.placeholderText}>{displayText}</Text>
        </Pressable>
        {selected ? (
          <Pressable
            style={styles.clearBtn}
            onPress={() => onChange('')}
            testID={testID ? `${testID}-clear` : undefined}
            accessibilityRole="button"
            accessibilityLabel="Clear date"
            hitSlop={8}
          >
            <Text style={styles.clearText}>Clear</Text>
          </Pressable>
        ) : null}
      </View>
      {error ? <Text style={styles.error}>{error}</Text> : null}
      {show ? (
        <>
          <DateTimePicker
            value={pickerValue}
            mode="date"
            display={Platform.OS === 'ios' ? 'inline' : 'default'}
            maximumDate={maximumDate}
            onChange={handleChange}
          />
          {Platform.OS === 'ios' ? (
            <Button title="Done" variant="ghost" onPress={() => setShow(false)} />
          ) : null}
        </>
      ) : null}
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
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: theme.spacing.sm,
  },
  input: {
    flex: 1,
    borderWidth: 1,
    borderColor: theme.color.border,
    borderRadius: theme.radius.sm,
    padding: theme.spacing.md,
    backgroundColor: theme.color.surface,
    justifyContent: 'center',
  },
  inputError: {
    borderColor: theme.color.danger,
  },
  valueText: {
    fontSize: theme.fontSize.md,
    color: theme.color.text,
  },
  placeholderText: {
    fontSize: theme.fontSize.md,
    color: theme.color.textMuted,
  },
  clearBtn: {
    paddingHorizontal: theme.spacing.sm,
    paddingVertical: theme.spacing.xs,
  },
  clearText: {
    fontSize: theme.fontSize.sm,
    fontWeight: theme.fontWeight.medium,
    color: theme.color.primary,
  },
  error: {
    fontSize: theme.fontSize.xs,
    color: theme.color.danger,
    marginTop: theme.spacing.xs,
  },
});
