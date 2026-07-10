import { useEffect, useRef, useState } from 'react';
import { StyleSheet, View } from 'react-native';
import { useAddMember } from '../hooks/useGroups';
import { Sheet } from './Sheet';
import { Button } from './Button';
import { TextField } from './TextField';
import { useToast } from './Toast';
import { theme } from '../theme';

interface AddMemberSheetProps {
  visible: boolean;
  onClose: () => void;
  groupId: string;
}

// Same regex the auth screens (register.tsx / login.tsx) validate email with.
const isValidEmail = (email: string) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);

export function AddMemberSheet({ visible, onClose, groupId }: AddMemberSheetProps) {
  const { show: showToast } = useToast();
  const addMember = useAddMember(groupId);

  const [email, setEmail] = useState('');
  const [name, setName] = useState('');
  const [emailError, setEmailError] = useState('');
  const [nameError, setNameError] = useState('');

  // Reset on the closed→open edge so a re-open starts clean (mirrors AllowanceSheet).
  const prevVisible = useRef(visible);
  useEffect(() => {
    if (visible && !prevVisible.current) {
      setEmail('');
      setName('');
      setEmailError('');
      setNameError('');
    }
    prevVisible.current = visible;
  }, [visible]);

  function handleClose() {
    setEmail('');
    setName('');
    setEmailError('');
    setNameError('');
    onClose();
  }

  async function handleAdd() {
    setEmailError('');
    setNameError('');
    if (!isValidEmail(email.trim())) {
      setEmailError('Enter a valid email address.');
      return;
    }
    if (!name.trim()) {
      setNameError('Enter a display name.');
      return;
    }
    try {
      await addMember.mutateAsync({ email: email.trim(), name: name.trim() });
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
    </Sheet>
  );
}

const styles = StyleSheet.create({
  footerRow: {
    flexDirection: 'row',
    gap: theme.spacing.sm,
  },
});
