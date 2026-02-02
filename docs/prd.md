# Product Requirements Document: Pocket Money MVP

---

## Executive Summary

**Pocket Money** is an MVP product that lets families or small groups track chores, assign monetary values to them, record who completed which chores (ledger), approve or reject entries, and record cash settlements—all from a single app that runs on Android, iOS, and Web. The backend runs on a home/LAN server; all clients connect to it over the network. The goal is to replace manual tracking (spreadsheets, paper, memory) with a simple, shared ledger and approval flow so that “pocket money” is transparent and auditable for both parents (group heads) and kids (members).

---

## Problem Statement

Parents and guardians who give pocket money in exchange for chores face several pain points:

- **No single source of truth:** Chore completion and amounts are tracked in notes, spreadsheets, or memory, leading to disagreements and “I already paid you for that.”
- **Asymmetric visibility:** Kids don’t see what they’ve earned or what’s pending; parents lack a clear view of balances and what’s been settled.
- **Manual approval:** There’s no lightweight way for the “head” to approve or reject self-reported chores before they affect the balance.
- **Settlement tracking:** Cash payouts aren’t recorded in one place, so balances drift from reality.

The product solves this by providing **groups** (e.g. one per family), **chores** with amounts, a **ledger** (entries approved/pending/rejected), **approval workflow** for member-submitted entries, and **settlements** to record payouts—all in one cross-platform app backed by a simple API.

---

## Goals & Success Metrics

| Goal | Success Metric |
|------|----------------|
| Replace manual chore/money tracking | Users create at least one group and add chores + ledger entries within first week. |
| Clear approval flow | Head can approve/reject pending entries; members see status (pending/approved/rejected). |
| Accurate balances | Per-member balance (approved ledger − settlements) is visible and used for settlements. |
| Easy onboarding of new members | Invite link (web or deep link) allows joining without app-store install [Proposed: track join-by-token success rate]. |
| Cross-platform usage | App works on Android, iOS, and Web from one codebase; MVP validated on all three. |

---

## User Personas

| Persona | Description | Primary needs |
|---------|-------------|----------------|
| **Group Head (e.g. Parent)** | Creates the group, defines chores and amounts, approves/rejects member-reported chores, records settlements. | Create group, invite members, manage chores, approve ledger entries, add settlements, view balances. |
| **Group Member (e.g. Child)** | Joins via invite, logs completed chores (pending approval), views own balance and ledger. | Join group, add ledger entries for completed chores, see pending/approved/rejected status and balance. |

[Proposed] **Secondary:** A single user may be head of one group and member of another (e.g. co-parent family); the MVP supports this via group list and role per group.

---

## User Stories

- **As a** parent/guardian, **I want to** create a group and add chores with names and amounts **so that** everyone knows what tasks exist and what they’re worth.
- **As a** parent/guardian, **I want to** share an invite link so my kids can join the group **so that** they can log chores and see balances without manual setup.
- **As a** group member (e.g. child), **I want to** add a ledger entry when I complete a chore **so that** my parent can approve it and my balance updates.
- **As a** parent/guardian, **I want to** see pending ledger entries and approve or reject them **so that** only verified work counts toward balance.
- **As a** parent/guardian, **I want to** record a cash settlement when I pay out **so that** the balance reflects real payouts and we don’t double-pay.
- **As a** group member, **I want to** see my balance and ledger history **so that** I know what I’ve earned and what’s been paid.
- **As a** user, **I want to** use the same app on phone and web **so that** I can check or log from any device on the same network.

---

## Functional Requirements

| ID | Feature | Priority | Description |
|----|---------|----------|-------------|
| F1 | User registration | P0 | Register with email, password, name, DOB, sex; store securely (bcrypt); no email verification in MVP. |
| F2 | User login | P0 | Login with email/password; receive JWT; support GET /auth/me for current user. |
| F3 | Groups – create & list | P0 | Authenticated user can create a group (becomes head) and list groups they belong to. |
| F4 | Group detail | P0 | View group by ID with members and chore count. |
| F5 | Invite link generation | P0 | Head can generate invite link (token); optional expiry; response includes invite_url. |
| F6 | Join group by token | P0 | User can join group via token (web query or deep link); POST /groups/join. |
| F7 | List group members | P0 | List members with roles (head, member). |
| F8 | Chores CRUD | P0 | Head: create, update, delete chores (name, description, amount). All members: list chores. |
| F9 | Ledger – create entry | P0 | Head: create approved entry (specify user_id, chore_id, amount). Member: create pending_approval entry (chore, amount). |
| F10 | Ledger – list & filter | P0 | List ledger entries for group; optional filter by status. |
| F11 | Ledger – approve/reject | P0 | Head: POST approve or reject on ledger entry; GET pending entries for group. |
| F12 | Balance | P0 | Per-member balance = sum(approved ledger) − sum(settlements). |
| F13 | Settlements – list & create | P0 | Head: create settlement (user_id, amount, date, note). All: list settlements for group. |
| F14 | Auth persistence & redirect | P0 | Persist JWT (e.g. SecureStore/AsyncStorage); redirect unauthenticated users to login. |
| F15 | Dashboard & navigation | P1 | Dashboard shows groups; create group; navigate to group detail; tabs/screens for Chores, Ledger, Settlements, Pending. |
| F16 | Invite/join UX | P1 | Generate link + copy/share; invite route reads token from URL (web) or deep link (mobile); join flow redirects to group. |
| F17 | Loading, errors, empty states | P1 | Loading indicators, error messages, empty states for lists and forms. |
| F18 | [Proposed] Pull-to-refresh | P2 | Refresh group/ledger/settlements on pull or screen focus. |

---

## Non-Functional Requirements

| Area | Requirement |
|------|--------------|
| **Security** | Passwords hashed with bcrypt; API auth via JWT in `Authorization: Bearer` header; auth middleware on protected routes; CORS restricted to configured origins (e.g. web app URL). |
| **Data integrity** | Foreign keys and unique constraints (e.g. group_id + user_id in group_members); ledger status enum (approved, pending_approval, rejected). |
| **Performance** | Backend runs on LAN; no strict latency SLA for MVP. [Proposed] Use DB connection pool and indexed lookups (e.g. ledger by group_id + status). |
| **Scalability** | MVP targets single-household or small group usage on one LAN backend; no multi-tenant or horizontal scaling requirement. |
| **Availability** | Backend availability = host machine availability; no redundancy in MVP. |
| **Deployment** | Backend: config via env (PORT, DATABASE_URL, JWT_SECRET, CORS_ORIGINS). Frontend: EXPO_PUBLIC_API_URL for API base (e.g. `http://<LAN-IP>:8080/api/v1`). |

---

## User Flow & UX Requirements

1. **Registration → Login**  
   User registers (email, password, name, DOB, sex) → then logs in (or auto-login [Proposed]) → token stored → redirect to dashboard.

2. **Dashboard**  
   Shows list of groups; actions: “Create group,” “Join via link” (navigate to invite or paste token). Tapping a group opens group detail.

3. **Group detail**  
   Tabs or stack: Overview (e.g. balance summary), Members, Chores, Ledger, Settlements, Pending (head only). Overview/Ledger shows per-member balance.

4. **Invite (head)**  
   “Generate link” → API creates invite token → show URL + copy button; user can share (e.g. WhatsApp). Optional expiry.

5. **Join (member)**  
   Open `/invite?token=xxx` (web) or deep link `pocketmoney://invite?token=xxx` (mobile) → app calls POST /groups/join with token → on success, redirect to joined group.

6. **Chores (head)**  
   List chores; add (name, description, amount); edit/delete. Members see list only.

7. **Ledger**  
   List entries (approved, pending, rejected). Head: “Add entry” → select member, chore, amount → created as approved. Member: “Add entry” → select chore, amount → created as pending_approval. Head: from Pending tab or ledger list, “Approve” or “Reject.”

8. **Settlements (head)**  
   List settlements; “Add settlement” → select member, amount, date, note.

9. **Errors & empty states**  
   Validation errors (e.g. 400) shown inline or toast; 401 → logout/redirect to login; empty lists show clear “No chores yet” / “No ledger entries” style messaging.

---

## Assumptions & Constraints

| Type | Item |
|------|------|
| **Scope** | Single backend instance on LAN; no cloud or multi-region. |
| **Auth** | No email verification, password reset, or OAuth in MVP. |
| **Security** | HTTPS not required for MVP; HTTP on LAN acceptable. |
| **Platform** | One frontend codebase (React Native + Expo + React Native Web) for Android, iOS, Web. |
| **Deployment** | No app store / Play Store submission; local/dev builds only for MVP. |
| **Data** | No offline support or conflict resolution; all data on server; clients refetch (e.g. on focus or pull-to-refresh). |
| **Multi-group** | User can be in multiple groups (head in one, member in another); “current group” is implicit (selected group in UI). |
| **Tech** | Backend: Go, Gin, pgx, golang-migrate, bcrypt, JWT. Frontend: Expo SDK 52+, TypeScript, expo-router (or React Navigation), env-based API URL. |

---

## Out of Scope (MVP)

- Email verification; password reset.
- OAuth / social login.
- Rate limiting; advanced request validation beyond basic checks.
- HTTPS (LAN HTTP acceptable).
- Offline support; conflict resolution.
- App store / Play Store distribution.
- Multi-group “current group” switching beyond list/detail (simple selection is in scope).

---

*This PRD is derived from the Pocket Money MVP implementation plan and is intended for engineering and stakeholder alignment. Marked [Proposed] items are recommendations where the plan did not specify.*
