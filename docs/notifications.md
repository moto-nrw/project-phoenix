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
        Scope:    notifications.ScopeTenant,  // or ScopeGuardian / ScopeGroup
    },
    Priority: notifications.PriorityNormal,   // low | normal | high
    Title:    "Abholung in 10 Minuten",       // display-safe, German
    Body:     "Ein Kind wird bald abgeholt.", // display-safe, German
    DeepLink: "/reminders",                   // app-relative path only
})
```

The producer never references a channel. `Notify` validates the event, checks
the feature flag, and fans out to all registered channels; a failing channel
is logged and never blocks the caller (fire-and-forget, like SSE
broadcasting).

Audience scopes map to the existing SSE routing:

| Scope | Recipients |
|---|---|
| `ScopeTenant` | every connected staff client of the tenant |
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

## Channels

| Channel | Status | Transport |
|---|---|---|
| SSE / in-app | active | `realtime.Broadcaster` → SSE event `notification` → toast |
| Web Push | prepared stub | see activation plan in `services/notifications/webpush_channel.go` |
| E-Mail | future | wrap `email.Mailer` + audience→address resolution as a `Channel` |

The existing SSE cache-invalidation events are untouched: the `notification`
event type is additive, and SSE remains the open-app channel while Web Push
is planned for closed/locked devices. Web Push subscription persistence
(multiple devices per account/tenant) is intentionally deferred until the
channel is activated, per the issue.

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

## Verifying a tenant's setup

`POST /api/notifications/test` (tenant JWT, `config:update`) dispatches a
fixed display-safe test event to the whole tenant. Returns `409` while the
feature flag is off. The frontend proxies it at `/api/notifications/test`.

## Adding a new channel

Implement `notifications.Channel` (`Name()` + `Deliver(ctx, Event)`), register
it in `services/factory.go` where `notifications.NewService` is wired, and
respect the GDPR contract above for anything that leaves the backend
(especially push payloads: `Title`/`Body`/`DeepLink` only, never `Data`).
