import type { Notification, Money, CurrencyCode } from './api';
import { formatMoney } from './money';

export type NotificationView = {
  icon: string;          // Ionicons name
  title: string;
  body: string;
  groupId: string | null; // deep-link target (null ⇒ non-navigable)
};

const CURRENCIES: readonly CurrencyCode[] = ['EUR', 'USD', 'INR'];

/** A string payload field, or null if absent/non-string. */
function str(payload: Record<string, unknown>, key: string): string | null {
  const v = payload[key];
  return typeof v === 'string' ? v : null;
}

/** A `Money {currency, value}` payload field, or null if malformed. */
function money(payload: Record<string, unknown>, key: string): Money | null {
  const v = payload[key];
  if (typeof v !== 'object' || v === null) return null;
  const m = v as Record<string, unknown>;
  if (typeof m.value !== 'number') return null;
  if (typeof m.currency !== 'string' || !CURRENCIES.includes(m.currency as CurrencyCode)) return null;
  return { value: m.value, currency: m.currency as CurrencyCode };
}

const UNKNOWN: NotificationView = {
  icon: 'notifications',
  title: 'Notification',
  body: '',
  groupId: null,
};

/**
 * Pure map from a Notification to its display model. Every known `type` has an
 * explicit row; anything else (and any malformed payload) falls back to the
 * forward-compatible UNKNOWN row rather than throwing. N-3 amounts render only
 * through the shared money formatter (D7) — never a bare int or hardcoded symbol.
 */
export function formatNotification(n: Notification): NotificationView {
  const payload = n.payload ?? {};
  const groupId = str(payload, 'group_id');
  const groupName = str(payload, 'group_name');

  switch (n.type) {
    case 'added_to_group':
      if (!groupId || !groupName) return UNKNOWN;
      return {
        icon: 'people',
        title: 'Added to a group',
        body: `You were added to ${groupName}.`,
        groupId,
      };

    case 'shadow_claimed':
      if (!groupId || !groupName) return UNKNOWN;
      return {
        icon: 'person-add',
        title: 'Member registered',
        body: `A member of ${groupName} just registered.`,
        groupId,
      };

    case 'payment_recorded': {
      const amount = money(payload, 'amount');
      if (!groupId || !groupName || !amount) return UNKNOWN;
      return {
        icon: 'cash',
        title: 'Payment recorded',
        body: `${formatMoney(amount.value, amount.currency)} was recorded as paid to you in ${groupName}.`,
        groupId,
      };
    }

    default:
      return UNKNOWN;
  }
}
