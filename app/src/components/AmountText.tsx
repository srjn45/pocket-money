import { StyleSheet, Text } from 'react-native';
import type { TextStyle } from 'react-native';
import { theme } from '../theme';
import { formatMinorGrouped, currencySymbol } from '../money';
import type { CurrencyCode } from '../money';

// Local copy with mutable fontVariant so it satisfies TextStyle (readonly arrays rejected).
const moneyStyle = StyleSheet.create({
  base: {
    fontVariant: ['tabular-nums'],
    fontWeight: theme.fontWeight.bold,
  },
}).base;

interface AmountTextProps {
  /** Amount in integer minor units (paise/cents). e.g. 1250 renders "₹12.50" (INR). */
  minorUnits: number;
  /**
   * ISO-4217 code of the amount. Required — no default, so tsc surfaces every
   * render site and none silently keeps ₹. Pass the Money's currency, or the
   * group's currency for locally-derived figures.
   */
  currency: CurrencyCode;
  /**
   * credit -> green, prefixed "+"; debit -> red, prefixed "−" (U+2212 minus).
   * neutral -> default text color, no sign (used for a chore's configured rate).
   * Default: 'neutral'.
   */
  variant?: 'credit' | 'debit' | 'neutral';
  /** Override sign display. Default: show +/− for credit/debit, nothing for neutral. */
  showSign?: boolean;
  /** Maps to theme.fontSize. sm=15, md=17 (default), lg=22, xl=28. */
  size?: 'sm' | 'md' | 'lg' | 'xl';
  style?: TextStyle;
  /** Accessibility: overrides the computed label. */
  accessibilityLabel?: string;
}

const variantColor = {
  credit:  theme.color.credit,
  debit:   theme.color.debit,
  neutral: theme.color.text,
} as const;

const variantSign = {
  credit:  '+',
  debit:   '−', // U+2212 minus sign
  neutral: '',
} as const;

const sizeMap = {
  sm: theme.fontSize.sm,
  md: theme.fontSize.md,
  lg: theme.fontSize.lg,
  xl: theme.fontSize.xl,
} as const;

export function AmountText({
  minorUnits,
  currency,
  variant = 'neutral',
  showSign,
  size = 'md',
  style,
  accessibilityLabel,
}: AmountTextProps) {
  const useSign = showSign !== undefined ? showSign : variant !== 'neutral';
  const sign = useSign ? variantSign[variant] : '';
  const label = `${sign}${currencySymbol(currency)}${formatMinorGrouped(minorUnits)}`;

  return (
    <Text
      style={[
        moneyStyle,
        { fontSize: sizeMap[size], color: variantColor[variant] },
        style,
      ]}
      accessibilityLabel={accessibilityLabel ?? label}
    >
      {label}
    </Text>
  );
}
