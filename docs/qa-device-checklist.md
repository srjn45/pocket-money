# Device QA Checklist — Pocket Money (Android + iOS golden path)

This is the **native acceptance gate**. There is no emulator in CI, so the golden path is
signed off by a human walking this checklist on a real device (or a documented simulator run).
Run it after installing a `preview`-profile build (see the root README ➜ **Native App (Android
APK)**). Tick every box; note any failure in the sign-off footer.

**Golden path under test:** register ➜ create an INR group **and** a EUR group ➜ add a member by
email (shadow) ➜ chores / base / loan ➜ month statement ➜ record payment ➜ notifications bell ➜
member claims their account. Cover it on **both** Android and iOS.

> Legend: tick a box once verified on the device. Where a check is platform-specific it says so;
> otherwise verify it on **each** platform you are signing off.

---

## A. Build & install

- [ ] **Android — sideload APK:** transfer the `preview` `.apk` to the device, allow "install
      unknown apps" for the transferring app, and the APK installs and launches.
- [ ] **Android — EAS install page:** *or* open the EAS internal-distribution install page / scan
      its QR on the device and install from there.
- [ ] **iOS — internal distribution:** installs via EAS internal distribution onto a registered
      ad-hoc device, **or** runs via `npx expo run:ios` (simulator) / `npx expo run:ios --device`
      (provisioned device).
- [ ] App launches straight to the **auth screen** with no crash and no red-box / LogBox error.

## B. Icons & splash

- [ ] **Android launcher** shows the **adaptive icon** (foreground on the `#4F46E5` background) —
      not the default Expo icon.
- [ ] **iOS home screen** shows the app icon (`assets/icon.png`), correct on both light and dark
      home screens.
- [ ] The **splash screen** shows `splash-icon.png` centered (`contain`) on the configured
      background during cold start, then hands off to the app with no flash of a wrong color.

## C. Safe areas & platform chrome

- [ ] **Status bar / notch:** no content is drawn under the notch or status bar; headers clear it.
- [ ] **Home indicator / gesture bar:** the bottom tab bar and scroll content clear the iOS home
      indicator and the Android gesture bar (safe-area insets applied; verify on a notched device).
- [ ] **Orientation:** the app stays **portrait-locked** when the device is rotated.
- [ ] **Android back:** the hardware / gesture **back** navigates sensibly — no dead-ends, no
      accidental app exit mid-flow.
- [ ] **Keyboard:** keyboard-avoiding forms don't cover the focused input (e.g. add-entry amount,
      login password).
- [ ] **Touch targets:** buttons, tab-bar items, and list rows are ≥ 44 pt and comfortably tappable.

## D. Auth & dashboard

- [ ] **Register** a new account, then **log in**.
- [ ] The token **persists across an app restart** (kill and relaunch — you stay logged in; secure
      store).
- [ ] The **dashboard** shows "Groups you manage" / "Groups you're in".
- [ ] Each per-group headline is shown in that group's **own currency** and is **never summed
      across currencies** (D7).

## E. Create groups (multi-currency)

- [ ] Create an **INR** group via the currency picker.
- [ ] Create a **EUR** group via the currency picker.
- [ ] The create-group copy makes clear the currency is **permanent** (cannot be changed later).

## F. Statement flow

- [ ] Open a group **as admin** ➜ **Statement**: the **month switcher** changes the displayed month.
- [ ] Each member row shows `base + chores − EMI = payable · cleared · remaining`.
- [ ] The **group total** is shown on top.
- [ ] Switching to a **past month** shows that month's archived statement.
- [ ] Amounts render in the **group currency** with the correct symbol/format (₹ / €) — no `NaN`,
      no raw minor units (e.g. `12345`).

## G. Add member by email (shadow)

- [ ] Add a member by **email** using an address with **no account yet** ➜ a **shadow member**
      appears in the group.
- [ ] The add-member sheet **closes** and the new member **row shows** in the list.

## H. Add entry / chores / loan

- [ ] The **add-entry picker** offers **chore / adjustment / clearing / new loan**.
- [ ] Add a **chore-based** entry and an **adjustment** ➜ both appear in the member's **passbook**.
- [ ] The **statement numbers move** to reflect the added entries.
- [ ] Create a **loan** ➜ the member detail shows the **loan card** with EMI progress / repayment
      schedule.

## I. Record payment

- [ ] Tap **Record payment** on a member row ➜ the amount is **pre-filled with the remaining**.
- [ ] Recording it **decreases "remaining"** and the sheet **closes** (a real state change, not just
      the absence of an error).

## J. Notifications bell

- [ ] After an action that notifies (e.g. a payment recorded), the header **bell** shows an unread
      **badge**.
- [ ] Opening the **notifications list** shows the item.
- [ ] **Marking it read clears the badge.**
- [ ] (This is the **in-app** bell + badge + list — **not** an OS push notification. Push delivery
      is out of scope.)

## K. Member view (claim)

- [ ] **Register the shadowed member's email** ➜ they **claim** the account.
- [ ] The claimed member sees **their own** statement / passbook — **own rows only, read-only**.
- [ ] They see the headline **"You'll receive €X / ₹X this month."** in the group currency.

---

## Sign-off

Run this checklist on at least **one real Android device** and **one iOS device** (a documented iOS
**simulator** run via `npx expo run:ios` is acceptable where no provisioned device is available).
Signing off here **is** the native-acceptance gate — there is no emulator in CI — and is formally
closed out in **V3-6.3**.

| Field | Android | iOS |
|-------|---------|-----|
| Device model | | |
| OS version | | |
| App version / build | | |
| Build profile | `preview` | `preview` |
| Install path (sideload APK / EAS page / `expo run`) | | |
| Tester | | |
| Date | | |
| **Result (PASS / FAIL)** | | |

Notes / failures observed:

>
