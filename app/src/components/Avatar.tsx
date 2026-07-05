import { StyleSheet, Text, View } from 'react-native';
import { theme } from '../theme';

const PALETTE = [
  '#4F46E5', // indigo
  '#7C3AED', // violet
  '#DB2777', // pink
  '#D97706', // amber
  '#059669', // emerald
  '#0891B2', // cyan
  '#DC2626', // red
  '#16A34A', // green
] as const;

function hashColor(id: string): string {
  let h = 0;
  for (let i = 0; i < id.length; i++) {
    h = (h * 31 + id.charCodeAt(i)) >>> 0;
  }
  return PALETTE[h % PALETTE.length];
}

interface AvatarProps {
  name: string;
  id: string;
  size?: number;
}

export function Avatar({ name, id, size = 40 }: AvatarProps) {
  const bg = hashColor(id);
  const initials = name.charAt(0).toUpperCase();
  const fontSize = Math.round(size * 0.42);

  return (
    <View
      style={[
        styles.circle,
        { width: size, height: size, borderRadius: size / 2, backgroundColor: bg },
      ]}
    >
      <Text style={[styles.initials, { fontSize }]}>{initials}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  circle: {
    alignItems: 'center',
    justifyContent: 'center',
  },
  initials: {
    color: theme.color.primaryText,
    fontWeight: theme.fontWeight.semibold,
  },
});
