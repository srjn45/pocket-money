import { formatNotification } from './notification-format';
import type { Notification } from './api';

function assert(cond: boolean, msg: string): void {
  if (!cond) throw new Error(`FAIL: ${msg}`);
}

function eq(a: unknown, b: unknown): boolean {
  return a === b;
}

/** Build a Notification-shaped fixture with sensible defaults. */
function makeNotif(over: Partial<Notification> = {}): Notification {
  return {
    id: 'n1',
    type: 'added_to_group',
    payload: {},
    read_at: null,
    created_at: '2026-01-01T00:00:00Z',
    ...over,
  };
}

// N-1 added_to_group → people / "Added to a group" / deep-links to group.
{
  const v = formatNotification(
    makeNotif({ type: 'added_to_group', payload: { group_id: 'g1', group_name: 'The Smiths', added_by_user_id: 'u9' } })
  );
  assert(eq(v.icon, 'people'), 'N-1 icon = people');
  assert(eq(v.title, 'Added to a group'), 'N-1 title');
  assert(eq(v.body, 'You were added to The Smiths.'), 'N-1 body drives off group_name');
  assert(eq(v.groupId, 'g1'), 'N-1 deep-links to payload.group_id');
}

// N-2 shadow_claimed → person-add / "Member registered".
{
  const v = formatNotification(
    makeNotif({ type: 'shadow_claimed', payload: { group_id: 'g2', group_name: 'Class 5B' } })
  );
  assert(eq(v.icon, 'person-add'), 'N-2 icon = person-add');
  assert(eq(v.title, 'Member registered'), 'N-2 title');
  assert(eq(v.body, 'A member of Class 5B just registered.'), 'N-2 body');
  assert(eq(v.groupId, 'g2'), 'N-2 deep-links to payload.group_id');
}

// N-3 payment_recorded → cash / amount via shared money formatter (INR).
{
  const v = formatNotification(
    makeNotif({ type: 'payment_recorded', payload: { group_id: 'g3', group_name: 'Household', amount: { currency: 'INR', value: 152000 } } })
  );
  assert(eq(v.icon, 'cash'), 'N-3 icon = cash');
  assert(eq(v.title, 'Payment recorded'), 'N-3 title');
  assert(eq(v.body, '₹1,520.00 was recorded as paid to you in Household.'), 'N-3 body renders money via formatMoney');
  assert(eq(v.groupId, 'g3'), 'N-3 deep-links to payload.group_id');
}

// N-3 EUR amount → euro symbol from amount.currency, never a hardcoded symbol.
{
  const v = formatNotification(
    makeNotif({ type: 'payment_recorded', payload: { group_id: 'g4', group_name: 'Trip', amount: { currency: 'EUR', value: 500 } } })
  );
  assert(eq(v.body, '€5.00 was recorded as paid to you in Trip.'), 'N-3 EUR amount uses € from amount.currency');
}

// Unknown / forward-compat type → notifications / "Notification" / non-navigable.
{
  const v = formatNotification(makeNotif({ type: 'future_type' as Notification['type'], payload: { group_id: 'gX' } }));
  assert(eq(v.icon, 'notifications'), 'unknown icon = notifications');
  assert(eq(v.title, 'Notification'), 'unknown title');
  assert(eq(v.body, ''), 'unknown body empty');
  assert(eq(v.groupId, null), 'unknown row is non-navigable');
}

// Malformed payload (missing group_name) falls back to unknown, never throws.
{
  const v = formatNotification(makeNotif({ type: 'added_to_group', payload: { group_id: 'g1' } }));
  assert(eq(v.groupId, null), 'N-1 with missing group_name → unknown fallback (non-navigable)');
  assert(eq(v.title, 'Notification'), 'N-1 with missing group_name → unknown title');
}

console.log('All notification-format tests passed.');
