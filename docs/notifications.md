# Notification Abstraction (#1624)

A single technical interface through which features trigger user-facing
notifications without knowing the delivery channels. This is deliberately an
abstraction, not a user feature: it is the foundation the "Erinnerungen" page
(#669) and later bell/push features build on.

## Feature flag

Everything is gated by the tenant setting `notifications.dispatch_enabled`
(boolean, **default off**, tab "Betrieb", permission `config:update`). While
off, `Notify` returns `notifications.ErrDisabled` and no channel delivers
anything.

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
| `ScopeGuardian` | one guardian account's own clients (`GuardianAccountID` required) |
| `ScopeGroup` | clients subscribed to one active group (`ActiveGroupID` required) |

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
  (mailto contact). With any of them unset the channel is inert and the
  subscribe endpoints return an error — nothing else changes. Generate a pair
  with `npx web-push generate-vapid-keys`.
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

## First consumer: reminder notifications

The scheduler task `reminder-notifications`
(`services/scheduler/reminder_notifications.go`) is the first real producer.
Every minute, per tenant, it computes the #1457 reminder list (pickups,
activity starts, overdue variants) and dispatches ONE aggregated event
(`Type: "reminders_due"`) for occurrences that newly became due — priority
`high` when something is overdue, deep link `/reminders`. Double gate:
`notifications.dispatch_enabled` AND at least one `reminders.*` type enabled.
The notification carries counts only, never student names (GDPR); a once-per-
day in-memory guard (rotated at midnight, refires once after a restart)
prevents repeats. The producer only builds an Event and calls `Notify` — new
channels apply to it automatically.

## Verifying a tenant's setup

`POST /api/notifications/test` (tenant JWT, `config:update`) dispatches a
fixed display-safe test event to the whole tenant. Returns `409` while the
feature flag is off. The frontend proxies it at `/api/notifications/test`.

## Adding a new channel

Implement `notifications.Channel` (`Name()` + `Deliver(ctx, Event)`), register
it in `services/factory.go` where `notifications.NewService` is wired, and
respect the GDPR contract above for anything that leaves the backend
(especially push payloads: `Title`/`Body`/`DeepLink` only, never `Data`).
