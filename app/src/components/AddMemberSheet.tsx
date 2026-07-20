import { useEffect, useRef, useState } from 'react';
import { StyleSheet, View } from 'react-native';
import { useAddMember } from '../hooks/useGroups';
import type { CurrencyCode } from '../api';
import { parseMoneyToMinorUnits, currencySymbol } from '../money';
import { Sheet } from './Sheet';
import { Button } from './Button';
import { TextField } from './TextField';
import { useToast } from './Toast';
import { theme } from '../theme';

interface AddMemberSheetProps {
  visible: boolean;
  onClose: () => void;
  groupId: string;
  /** Group currency — stamped onto the optional base-pay amount (QA batch 1, Item 4). */
  currency: CurrencyCode;
}

// Same regex the auth screens (register.tsx / login.tsx) validate email with.
const isValidEmail = (email: string) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);

export function AddMemberSheet({ visible, onClose, groupId, currency }: AddMemberSheetProps) {
  const { show: showToast } = useToast();
  const addMember = useAddMember(groupId);

  const [email, setEmail] = useState('');
  const [name, setName] = useState('');
  const [basePayStr, setBasePayStr] = useState('');
  const [emailError, setEmailError] = useState('');
  const [nameError, setNameError] = useState('');
  const [basePayError, setBasePayError] = useState('');

  // Reset on the closed→open edge so a re-open starts clean (mirrors AllowanceSheet).
  const prevVisible = useRef(visible);
  useEffect(() => {
    if (visible && !prevVisible.current) {
      setEmail('');
      setName('');
      setBasePayStr('');
      setEmailError('');
      setNameError('');
      setBasePayError('');
    }
    prevVisible.current = visible;
  }, [visible]);

  function handleClose() {
    setEmail('');
    setName('');
    setBasePayStr('');
    setEmailError('');
    setNameError('');
    setBasePayError('');
    onClose();
  }

  async function handleAdd() {
    setEmailError('');
    setNameError('');
    setBasePayError('');
    if (!isValidEmail(email.trim())) {
      setEmailError('Enter a valid email address.');
      return;
    }
    if (!name.trim()) {
      setNameError('Enter a display name.');
      return;
    }
    // Base pay is optional; only validate/send when the field is non-empty.
    let basePay: { currency: CurrencyCode; value: number } | undefined;
    if (basePayStr.trim()) {
      const value = parseMoneyToMinorUnits(basePayStr);
      if (value === null) {
        setBasePayError('Enter a valid amount (e.g. 500).');
        return;
      }
      basePay = { currency, value };
    }
    try {
      await addMember.mutateAsync({ email: email.trim(), name: name.trim(), base_pay: basePay });
      showToast({ tone: 'success', message: 'Member added' });
      handleClose();
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'Failed to add member';
      showToast({
        tone: 'danger',
        message: msg.toLowerCase().includes('already')
          ? 'This person is already a member of this group.'
          : msg,
      });
    }
  }

  return (
    <Sheet
      visible={visible}
      onClose={handleClose}
      title="Add member"
      footer={
        <View style={styles.footerRow}>
          <Button title="Cancel" variant="ghost" onPress={handleClose} fullWidth />
          <Button
            title="Add member"
            variant="primary"
            onPress={handleAdd}
            loading={addMember.isPending}
            disabled={addMember.isPending}
            fullWidth
            testID="add-member-submit"
          />
        </View>
      }
    >
      <TextField
        label="Email"
        keyboardType="email-address"
        autoCapitalize="none"
        autoCorrect={false}
        value={email}
        onChangeText={(v) => { setEmail(v); setEmailError(''); }}
        error={emailError}
        placeholder="name@example.com"
        testID="add-member-email"
      />
      <TextField
        label="Display name"
        value={name}
        onChangeText={(v) => { setName(v); setNameError(''); }}
        error={nameError}
        placeholder="e.g. Aanya"
        testID="add-member-name"
      />
      <TextField
        label={`Base pay (optional, ${currencySymbol(currency)})`}
        keyboardType="decimal-pad"
        value={basePayStr}
        onChangeText={(v) => { setBasePayStr(v); setBasePayError(''); }}
        error={basePayError}
        placeholder="e.g. 500"
        testID="add-member-base-pay"
      />
    </Sheet>
  );
}

const styles = StyleSheet.create({
  footerRow: {
    flexDirection: 'row',
    gap: theme.spacing.sm,
  },
});
