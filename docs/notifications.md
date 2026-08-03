# Notification Abstraction (#1624)

A single technical interface through which features trigger user-facing
notifications without knowing the delivery channels.

It started as an abstraction with no user-visible surface (#1624). With the
personal-notification epic it also became a feature: people choose in their own
profile what they want to hear about, and notifications are addressed to
individuals rather than broadcast to a school. Two things follow from that and
run through this whole document — **no consent, no delivery**, and **the
payload never carries a child's name**.

## Gates

Five gates stand between a producer and a phone. All of them must pass, and
they are checked in this order:

| # | Gate | Where |
|---|---|---|
| 1 | `notifications.dispatch_enabled` (tenant setting, **default on**) | `router.Notify` / `NotifyBatch` |
| 2 | Delivery window `notifications.active_window_start`/`_end` (`test` events exempt) | `router.withinActiveWindow` |
| 3 | The type's school-wide gate, if it has one (`TypeDefinition.TenantGate`) | `PreferenceService.FilterOptedIn` |
| 4 | The recipient's own consent for that type | `PreferenceService.FilterOptedIn` |
| 5 | `notifications.on_duty_only`, for personal staff notifications | the reminder tick / absence producer |

Gate 1 defaults to **on** since the personal-notification epic. That is only
safe because gates 3–5 exist: every producer routes through consent, and an
enabled school therefore still delivers nothing until somebody asks for
something in their profile. Off now only means that a person's own choice is
silently ignored. While off, `Notify` returns `notifications.ErrDisabled`;
outside the window, `ErrOutsideActiveWindow`. Producers treat both as a silent
no-op.

Consent starts empty for everybody except the accounts migration 1.15.240
backfills. That backfill is deliberately narrow: it only writes rows for
schools that had already switched `dispatch_enabled` on themselves, and only
for accounts still active in that school. At those schools the registered
device really was receiving these notifications, so the row records a consent
that already existed in practice rather than inventing one. Everywhere else the
switch starts off.

### E-mail is not routed through these gates

Gates 1 and 2 govern push and in-app hints: they exist so a phone does not ring
at night, and an e-mail does not ring. Announcement e-mails therefore leave
regardless of the dispatch switch and the delivery window, and they apply the
opposite consent rule, `FilterNotOptedOut` instead of `FilterOptedIn`. Only an
explicit "no" removes a recipient; a family that never touched the switch keeps
receiving the mail it received before consent existed. Tying e-mail to
`FilterOptedIn` would have made the reach of a backfill decide whether a
long-standing channel works at all.

What e-mail does share with push is the child-access question. A queued mail
about a child is re-checked when it is sent, not only when it is queued — see
"Queued mail is re-checked when it is sent" below.

## Triggering a notification (backend)

```go
import "github.com/moto-nrw/project-phoenix/services/notifications"

err := factory.Notifications.Notify(ctx, notifications.Event{
    Type:     "pickup_upcoming",              // stable, feature-defined type
    Audience: notifications.Audience{
        TenantID: tenantID,
        Scope:    notifications.ScopeTenant,  // or ScopeAdmin / ScopeGuardian / ScopeGroup
    },
    Priority: notifications.PriorityNormal,   // low | normal | high
    Title:    "Abholung in 10 Minuten",       // display-safe, German
    Body:     "Ein Kind wird bald abgeholt.", // display-safe, German
    DeepLink: "/reminders",                   // app-relative path only
})
```

The producer never references a channel. `Notify` validates the event, checks
the feature flag, and queues fan-out until the surrounding tenant transaction
commits. Without a tenant transaction, delivery remains synchronous. A failing
channel is logged and never blocks the caller (fire-and-forget, like SSE
broadcasting).

Audience scopes map to the existing SSE routing:

| Scope | Recipients |
|---|---|
| `ScopeTenant` | every connected staff client of the tenant |
| `ScopeAdmin` | connected staff clients with effective admin scope in the tenant |
| `ScopeGuardian` | guardian accounts' own clients (`GuardianAccountID` or `GuardianAccountIDs`) |
| `ScopeGroup` | clients subscribed to one active group (`ActiveGroupID` required) |
| `ScopeStaff` | named staff accounts' own clients and devices (`StaffAccountIDs`) |

`ScopeStaff` is the scope of personal notifications: each recipient gets their
own event, so the payload can differ per person. `NotifyBatch(ctx, events)`
dispatches many such events of one tenant in one after-commit hook — it
resolves the feature flag and the window once, and hands the whole batch to
channels that implement `BatchChannel` (Web Push does; it resolves every
recipient's devices in ONE tenant transaction).

## GDPR contract

`Title`, `Body` and `DeepLink` are the only user-visible fields and MUST be
display-safe: no student names or other sensitive child data. Validation
mechanically enforces what it can (deep links must be app-relative, never an
external URL); the semantic half is a producer responsibility. Sensitive
details are loaded authenticated after the user follows the deep link into
the app. `Data` carries opaque IDs only. The SSE channel forwards it as
`notification_data` for client-side routing; channels that leave the app
(especially Web Push and E-Mail) must not include it.

Display safety does not replace recipient authorization. Producers must choose
an audience that matches the visibility of the data used to build the event.
For example, an aggregate derived from an admin-wide view must use
`ScopeAdmin`, not `ScopeTenant`.

## Consent (per account, per type)

Nothing is delivered to a person who did not ask for it. The rule is
mechanical: **no row means off**.

- **Catalogue** — `services/notifications/types.go`. One `TypeDefinition` per
  notification a person can agree to: key, German label and description (the
  profile page renders them verbatim), display group, portal (`staff` /
  `parent`), and the optional `TenantGate` setting a school can use to switch
  the type off for everybody. Registered from `init()` like the settings
  registry, panicking on a duplicate key. Adding a type costs no migration.
- **Storage** — `users.notification_preferences` (migration 1.15.239):
  `UNIQUE (tenant_id, account_id, notification_type)`, RLS-isolated, partial
  index on `(tenant_id, notification_type) WHERE enabled` because the producers'
  hot question is "who in this school agreed to X". A row with
  `enabled = false` is deliberately different from no row: it records a decision
  a later change of defaults must not overwrite. `notification_type` carries no
  CHECK constraint — an unknown type is inert, nothing reads it.
- **Service** — `services/notifications/preferences.go`. `GetForAccount` /
  `SetForAccount` for the staff portal; `GetForParent` / `SetForParent` act
  across every active school of a guardian at once (the parents app has no
  tenant context of its own). A type the school currently blocks can still be
  switched on: the gate is applied at delivery time, so an admin toggling the
  school setting off and on again does not erase anybody's choice.
- **Enforcement** — `FilterOptedIn(ctx, type, accountIDs)` is the single point
  where consent is applied, and it also checks the type's `TenantGate`. The
  order matters: **a producer builds its candidate set from the relation
  (whose group, whose team, whose child) and then narrows it with
  `FilterOptedIn`** — never the other way round. An unknown type fails closed
  and reaches nobody.
- **Endpoints** — staff: `GET /api/notifications/preferences`,
  `PUT /api/notifications/preferences/{type}`,
  `DELETE /api/notifications/preferences` (switch everything off). Parents:
  the same shapes under `/parent/me/notification-preferences`. The UI is
  `components/settings/notification-preferences-section.tsx`, rendered on the
  staff profile page and in the parents dashboard above the per-device push
  card — this one answers "what do I want to hear about", the other "on which
  device".
- **Backfill** — migration 1.15.240 gives an account with an existing push
  registration the consent that device already acted on: `parent_announcement`
  for guardian devices, the four reminder types for staff devices. Without it,
  flipping `dispatch_enabled` on by default would have silenced every phone
  that is receiving notifications today, in the same minute. Two conditions
  keep it from inventing consent instead of recording it: the school must have
  switched `dispatch_enabled` on itself (an explicit override row), because a
  school that never did delivered nothing and therefore has no prior consent to
  record; and the account must still be active in that school. It is
  `ON CONFLICT DO NOTHING` (never overwrite a decision) and its `down` is a
  deliberate no-op, which is exactly why both conditions matter: a wrong row
  written here stays forever.

## Channels

| Channel | Status | Transport |
|---|---|---|
| SSE / in-app | active | `realtime.Broadcaster` → SSE event `notification` → toast |
| Web Push | active (#2003) | `webpush-go` (VAPID) → browser push service → service worker notification |
| E-Mail | future | wrap `email.Mailer` + audience→address resolution as a `Channel` |

E-mail is still not a `Channel`: the features that mail (announcements,
appointments, appointment reminders) enqueue their own outbox rows next to the
`Notify` call. That is why the appointment reminder producer writes to the
outbox AND dispatches a push rather than dispatching once — see the gates
section above for why the two paths also apply different consent rules.

The existing SSE cache-invalidation events are untouched: the `notification`
event type is additive. SSE remains the open-app channel; Web Push covers
closed/locked devices.

### Web Push (#2003)

- **Keys**: `VAPID_PUBLIC_KEY` / `VAPID_PRIVATE_KEY` / `VAPID_SUBSCRIBER`
  (mailto contact). With all three unset the channel is inert and the subscribe
  endpoints return an error. Partial configs, malformed or mismatched P-256
  keys, and invalid contact URIs prevent server startup. Generate a pair with
  `npx web-push generate-vapid-keys`.
- **Subscriptions** live in `iot.push_subscriptions` (RLS, unique per
  `(tenant_id, endpoint)`, multiple devices per account, `portal` =
  `staff`/`parent`). Staff endpoints: `GET /api/notifications/push/public-key`,
  `POST`/`DELETE /api/notifications/push/subscriptions` (DELETE takes
  `?endpoint=…`). Parent endpoints mirror them under `/parent/me/push/*` and
  register one row per active guardian tenant mapping.
- **Audience mapping**: `tenant` → all staff-portal subscriptions of the
  tenant; `admin` → staff-portal subscriptions of accounts with the admin
  role; `guardian` → the one account's parent-portal subscriptions.
  `group` is deliberately NOT delivered over push (no persisted
  device-to-group membership; follow-up if a producer ever needs it).
  Announcement recipients reached only through `pending_enrollment` are also
  deliberately excluded from Web Push until they have an active guardian
  mapping for that school. Their announcement remains available in the parent
  feed and through the announcement's optional e-mail delivery.
- **Child-scoped audiences**: a guardian event that is ABOUT children sets
  `Audience.StudentIDs` — one child for a parent message or a request decision,
  the children an appointment was addressed to for the calendar producers. That
  is an authorization instruction, not payload — the delivery path then requires
  `parent_portal.access` for at least one of them on the recipient's
  `users.students_guardians` row. Producers decide their audience in the
  transaction that produced the event, but delivery runs later, in a
  transaction of its own; a school can revoke access to a child in between,
  and a push is rendered on a lock screen. The check can only narrow the
  audience the producer chose, never widen it: every listed child is one the
  producer already resolved the audience from, so "at least one" cannot admit
  an account the producer excluded. Only events about no child at all
  (announcements) leave the field empty and keep their producer's gate; a
  non-guardian scope with student ids is rejected as malformed, so a producer
  cannot believe it narrowed something it did not.

  A producer addressing many accounts at once splits them by the children they
  were let through by rather than sending one event with the union
  (`calendar.guardianStudentGroups`). "At least one" is only as narrow as the
  set it is asked against: with a union, an account that lost access to the
  child it was invited for would still pass through a child of a **different**
  recipient it happens to be a guardian of — and receive a push for an
  appointment the parents portal then refuses to show it.

  **Both guardian-facing channels ask it**, not only Web Push. The Web Push
  channel folds the predicate into its device lookup
  (`PushSubscriptionRepository.FindForGuardians`); the SSE channel resolves the
  permitted recipients first
  (`StudentGuardianRepository.FilterAccountsWithStudentAccess`) and fans out only
  to those. An in-app event carries the same title, body and deep link a push
  does, so an already-open parent session must not be woken with something a push
  would have been withheld from. A SSE channel wired without that lookup drops
  student-scoped guardian events rather than deliver them unchecked.
- **Payload** is `{title, body, deepLink, type, priority}` — never `Data`
  (GDPR contract above). Priority maps to TTL/urgency: high = 1h/high,
  normal/low = 24h/normal/low.
- **Pruning**: a push-service response of 404/410 deletes the subscription.
  Logout removes the device's registration best-effort; the frontend also
  re-syncs the subscription on opt-in.
- **Frontend**: `public/sw.js` (notification + deep-link click handling),
  `lib/push-api.ts`, opt-in section on the staff profile page and the parents
  dashboard. The permission prompt only ever runs from the button click
  (iOS requirement); on iOS Safari outside an installed home-screen app the
  section explains the install prerequisite instead (help guide chapter
  "App installieren & Benachrichtigungen", #1915).

## Frontend

- `use-global-sse.ts` receives `notification` SSE events and re-dispatches
  them as `phoenix:notification` window events. The parents portal shares the
  same dispatch path through `ParentRealtimeBridge`.
- `components/notifications/notification-bridge.tsx` (mounted once in
  `app/providers.tsx`) renders them as toasts — `high` priority as a warning
  toast, everything else as info. A valid app-relative `DeepLink` adds an
  "Öffnen" action. This is also the seam where a future bell inbox or
  push-permission prompt hooks in.
- Unlike all other SSE events, `notification` events are rendered directly,
  not used as refetch triggers.

## Producers

### Personal reminders (scheduler task `reminder-notifications`)

`services/scheduler/reminder_notifications.go`. Every minute, per tenant:

1. **Candidates** — `StaffRepository.ListAllStaffAccountIDs`: every staff
   member of the school with an active account and an active tenant mapping.
2. **Consent** — `FilterOptedIn` per type (`pickup_upcoming`,
   `pickup_overdue`, `activity_start`, `activity_overdue`,
   `my_activity_starting`), which also applies each type's `reminders.*` gate.
   Nobody opted in → the tick ends **before** any reminder is computed, which
   is the normal case and keeps the per-minute cost at six cheap queries.
3. **On duty** — with `notifications.on_duty_only` (default on) only people
   with an open work session today remain. An empty presence map reaches
   nobody; schools without time tracking must switch this setting off.
4. **Compute** — one `reminders.ComputeBatch` for all recipients: the
   tenant-wide reads happen once, only the per-person slice is derived in
   memory (see the package comment in `services/reminders/batch.go`).
5. **Dispatch** — one event per person and type, aggregated by count, through
   a single `NotifyBatch`. An activity the person is personally planned on is
   reported as `my_activity_starting` (the batch reports the assignment in
   `Result.AssignedActivityInstanceIDs`), everything else in a supervised room
   as `activity_start`; the two switches therefore never overlap and the same
   activity never arrives twice.

Copy is count-only ("2 Kinder aus Ihrer Aufsicht werden bald abgeholt"), never
a name — the reminder rows behind it carry names, the notification must not.
Deep link `/reminders`, priority `high` for the two overdue types. A re-fire
guard keyed by `(tenant, account, reminder identity)` and rotated at midnight
prevents repeats; a failed dispatch leaves the occurrence eligible for the next
tick. The producer lives in the scheduler because it needs both
`services/reminders` and `services/notifications`, and those two deliberately
do not know each other.

### Absences (`services/notifications/absence_producer.go`)

`NotifyAbsenceReported` turns a recorded sick note or excuse for **today** into
a `student_absence_reported` notification for the supervising staff of the
child's group plus the office (effective admins), minus the person who entered
it. Gated additionally by `notifications.absence_reported_enabled` and
`notifications.on_duty_only`; with the latter enabled, only accounts mapped to
staff who are currently checked in remain. Wired into the parent write path and
all three staff-side entry points
(`student_status_day_write.go`, `api/students/status_history.go`,
`excused_request_service.go`). Every call runs after the absence transaction
commits; the producer opens a fresh tenant transaction for its RLS-scoped
recipient and consent reads.

### Parent announcements (`services/announcement/service.go`)

A published announcement notifies its guardian audience as
`parent_announcement`, narrowed by consent. A nil preference service keeps the
pre-consent behaviour (it is always wired in the factory); a wired one has the
final say.

### Parent-facing producers (#1671)

Three more producers address guardians. All of them narrow their audience with
`FilterOptedIn` and none of them names a child — the deep link into the
authenticated parents app is where the subject is shown. Parent deep links carry
no `/parents` prefix: on the parents host that prefix is added by the proxy, and
the service worker opens the link against the browser origin.

| Producer | Type | Fires when | Deep link |
|---|---|---|---|
| `services/messaging` (`notifyGuardianDevice`) | `parent_message` | staff send a message in a child's thread | `/messages` |
| `services/calendar` (`notifyGuardianDevices`) | `parent_appointment` | an appointment is published, changed or cancelled | `/calendar` |
| `services/calendar` (reminder scan) | `parent_appointment_reminder` | shortly before an appointment | `/calendar` |
| `services/parentmessaging` (`notifyRequestDecision`) | `parent_request_decided` | staff decide a care-schedule, master-data or excused-absence request | `/children/{id}` |

Two of these deliberately ride on an existing opt-in rather than introducing a
new one:

- **Appointments** only notify when the appointment carries the organizer's
  per-appointment "Eltern benachrichtigen" flag (`appointments.notify_guardians`)
  — the same flag that governs the e-mails. An organizer who chose not to notify
  does not get to notify anyway through a second channel.
- **Request decisions** hook into the pill emitter, where all three request
  flows already converge, and fire under exactly the condition that fires the
  guardian's SSE wake. So pill, wake and push always agree on who was reached: a
  guardian whose access was revoked gets none of the three, and a school with
  parent messaging switched off gets none either (the pill is that school's
  in-app channel too, and pushing about something the app does not show would be
  a dead end). The push runs in a transaction of its own and therefore re-reads
  the child access there instead of inheriting the pill transaction's verdict —
  a payload rendered on a lock screen answers that question against the row the
  sending transaction sees. The recheck can only narrow the push against the
  pill, never widen it.

Both child-related producers — the message push and the decision push — send
their audience with `Audience.StudentIDs` set, so the child access is answered
once more where the sending happens, in both the device lookup and the in-app
fan-out (see "Child-scoped audiences" above).
The staff message path checks it in the request transaction that stores the
message and again in the delivery transaction; between the two a school can
revoke a guardian's access, and only the second answer decides what leaves the
backend.

The two appointment producers do the same with the children the appointment was
addressed to. Their audience already comes from `parent_portal.access` on the
appointment's recipient rows; carrying those children into the event means the
delivery path asks again, so an access revoked between resolving recipients and
sending drops both the push and the in-app wake instead of putting the event on
a lock screen or into an open session. Their e-mails carry the same scope into
the outbox and are re-checked when the worker sends them.

### Appointment reminders (scheduler task `appointment-reminders`)

`services/scheduler/appointment_reminders.go` — every 5 minutes, per tenant:

1. **Gate** — `calendar.appointment_reminder_enabled` (default **on**) and
   `calendar.appointment_reminder_lead_hours` (default 24, bounded 1–168).
   The default is on because the reminder only ever reaches an appointment whose
   organizer already opted into guardian mail; it is a second notice on an
   existing channel, not a new one.
2. **Window** — the tick keeps the upper bound of the last scanned window, so
   consecutive windows are adjacent and a late or missed tick stretches the
   window instead of skipping the occurrences in between. After downtime it is
   clamped to one hour: a reminder whose moment is long gone is noise. Both
   bounds are shifted by the lead time, so the window stays exactly one tick
   long.
3. **Compute** — `calendar.EnqueueDueAppointmentReminders(from, to)` expands
   occurrences with the same code the calendar and the ICS export use (never a
   second copy of that arithmetic), applies single-occurrence overrides, skips
   cancelled occurrences, and keeps the ones whose start instant lies in the
   half-open window. An all-day appointment is anchored at 08:00 local rather
   than midnight.
4. **Deliver** — one outbox row per reachable guardian
   (`appointment_reminder`, the renderer that already existed for it) plus the
   `parent_appointment_reminder` push. The two are independent: an installation
   without an e-mail outbox still sends the push, and a guardian without an
   e-mail address is still reachable on their device.

The reminder is the one producer that dispatches through
`NotifySynchronously`, because its claim may only be kept once the push service
has answered. Only Web Push is waited for; the in-app channel delivers
fire-and-forget from the same call, exactly as under `Notify`. A parent with the
portal open therefore sees the reminder even when no device is registered, and a
failing SSE hub never costs a delivered push its claim.

The push half is guarded by a durable claim per (appointment, revision,
occurrence, guardian) instead of an outbox key. A claim records that a push went
out, not that one was attempted: whenever nothing was delivered — no registered
device, no VAPID keys, notifications switched off, closed delivery window, or a
failed dispatch — the claim is released again. Only a delivered push keeps its
claim, which is what makes an overlapping scan silent.

Which of those five comes back is decided by the scan window, not by the claim.
A **failed dispatch** fails the tenant's pass, so its upper bound is never
recorded and the next tick offers the same window again — that is where the
released claim earns its keep, together with a lead-time increase, which shifts
occurrences back into an already scanned window. The other four are states
rather than failures: the tick stays quiet, the boundary advances, and that
occurrence's push is dropped while its e-mail still goes out. Quiet hours in
particular behave here exactly as they do for every other producer — the router
drops an event it may not deliver now instead of queueing it, so at the default
24-hour lead an evening appointment reaches parents by mail only. Schools with
many evening appointments raise `notifications.active_window_end`.

Duplicate delivery is impossible by construction: each row carries an
idempotency key of (appointment, occurrence, guardian) and the outbox insert is
`ON CONFLICT DO NOTHING`, so a re-run, an overlapping window or a second
scheduler process cannot produce a second mail.

An **edit between queueing and dispatch** bumps the revision and therefore
invalidates the claim the pending push holds. The claim is not simply dropped
with it: the scan boundary has already moved past that occurrence and no later
tick offers it again, so preparation releases the old claim and re-takes one
under the new revision, provided the occurrence still exists, is not cancelled,
and still starts inside the window it was scanned in. An occurrence the edit
moved out of that window is left alone — its released claim lets the tick that
reaches its new moment deliver it. The e-mail half keeps its documented gap: the
pending reminder mail is cancelled with the other pending mails of the edited
appointment and not re-queued, so the parent gets the "Termin geändert" mail and
the reminder push.

### Queued mail is re-checked when it is sent

An appointment mail carries the account it addresses and the children of that
appointment the account was let through by (`guardian_account_id`,
`student_ids`). The renderer asks `FilterAccountsWithStudentAccess` again inside
the row's tenant transaction immediately before building the message, and
returns `platform.ErrRenderCancelled` when the answer changed — the worker then
retires the row instead of spending its retry budget on something it may never
deliver.

This matters most for the reminder, which waits in the outbox for the whole lead
time (24 hours by default) while naming a title, a time and a place. Cancelling
queued rows from each revocation path instead would have to catch every one of
them to be worth anything; asking at delivery is the same recheck the push
channels do. Rows without the scope — queued before this existed, or addressed
to a recipient with no portal account — render unchanged.

## Verifying a tenant's setup

`POST /api/notifications/test` (tenant JWT, `config:update`) dispatches a
fixed display-safe test event to the whole tenant. Returns `409` while the
feature flag is off. The frontend proxies it at `/api/notifications/test`.

## Adding a new notification type

1. Add the key and a `RegisterType(TypeDefinition{...})` call in
   `services/notifications/types.go` — German label and description (the
   profile page renders them as they are), a group from the existing set, the
   portal, and a `TenantGate` only if a school should be able to switch the
   type off for everybody. No migration.
2. Build the candidate set in the producer from the relation the notification
   is about, then narrow it with `FilterOptedIn(ctx, yourType, candidates)`.
3. Write display-safe copy: counts and roles, never names.
4. Nothing on the frontend: the preference card is generated from the
   catalogue.

## Adding a new channel

Implement `notifications.Channel` (`Name()` + `Deliver(ctx, Event)`), register
it in `services/factory.go` where `notifications.NewService` is wired, and
respect the GDPR contract above for anything that leaves the backend
(especially push payloads: `Title`/`Body`/`DeepLink` only, never `Data`).
