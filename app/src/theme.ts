export const theme = {
  color: {
    // brand / semantic (master plan §7.4)
    primary:      '#4F46E5', // indigo — primary actions, links, focus
    primaryText:  '#FFFFFF', // text/icon on a primary surface
    primaryMuted: '#EEF2FF', // tint bg for secondary/ghost pressed, info badges
    success:      '#16A34A', // credits, positive/confirmed
    danger:       '#DC2626', // debits, destructive, errors
    warning:      '#D97706', // pending / needs-attention

    // neutrals
    text:          '#111827', // primary text
    textSecondary: '#6B7280', // subtitles, captions
    textMuted:     '#9CA3AF', // disabled, placeholder, empty-state glyphs
    border:        '#E5E7EB', // hairlines, input borders, card outlines
    surface:       '#FFFFFF', // cards, sheets, inputs
    surfaceMuted:  '#F9FAFB', // system/disabled row bg
    background:    '#F5F5F5', // screen background (matches current screens)
    overlay:       'rgba(0,0,0,0.5)', // sheet/modal scrim

    // money direction (aliases so ledger reads intent, not raw color)
    credit: '#16A34A', // = success
    debit:  '#DC2626', // = danger
  },

  spacing: { xs: 4, sm: 8, md: 12, lg: 16, xl: 24 }, // §7.4: 4/8/12/16/24

  radius: { sm: 8, md: 12, lg: 16, pill: 999 }, // §7.4 default radius = md (12)

  fontSize: { xs: 13, sm: 15, md: 17, lg: 22, xl: 28 }, // §7.4 type scale

  fontWeight: {
    regular:  '400',
    medium:   '500',
    semibold: '600',
    bold:     '700',
  },
} as const;

// Money text style — tabular-nums + bold (§7.4: "money amounts in a tabular-nums bold style").
// Spread into a Text style; `fontVariant` aligns digit columns so amounts line up in lists.
export const moneyTextStyle = {
  fontVariant: ['tabular-nums'] as const,
  fontWeight: theme.fontWeight.bold,
} as const;

export type Theme = typeof theme;
