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
safe because gates 3–5 exist: every producer routes through consent, consent
starts empty, and an enabled school therefore still delivers nothing until
somebody asks for something in their profile. Off now only means that a
person's own choice is silently ignored. While off, `Notify` returns
`notifications.ErrDisabled`; outside the window, `ErrOutsideActiveWindow`.
Producers treat both as a silent no-op.

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
- **Backfill** — migration 1.15.240 gives every account with an existing push
  registration the consent that device already acted on: `parent_announcement`
  for guardian devices, the four reminder types for staff devices. Without it,
  flipping `dispatch_enabled` on by default would have silenced every
  registered phone in the same minute. It is `ON CONFLICT DO NOTHING` (never
  overwrite a decision) and its `down` is a deliberate no-op.

## Channels

| Channel | Status | Transport |
|---|---|---|
| SSE / in-app | active | `realtime.Broadcaster` → SSE event `notification` → toast |
| Web Push | active (#2003) | `webpush-go` (VAPID) → browser push service → service worker notification |
| E-Mail | future | wrap `email.Mailer` + audience→address resolution as a `Channel` |

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
