# Plan: Show children currently in a room (issue #1323)

> Iterating across sessions. Top section is the contract — change it deliberately.
> Everything below the contract is implementation detail and may shift.

---

## Scope

Add a "Kinder im Raum" view to the room detail page so staff can quickly see
who is in a room right now (e.g. for pickups). Provide a one-click jump to
Kindersuche pre-filtered to the same room.

**In scope**
- Room detail page (`/rooms/{id}`) lists children currently checked in to
  any active group taking place in that room.
- Button "In Kindersuche öffnen" → `/students/search?room_id={id}`.
- Kindersuche supports `?room_id={id}` filter end-to-end (URL → frontend
  state → backend list endpoint → filtered response).
- Permissions and PII redaction follow the existing
  `gdpr.student_data_scope` setting (already registered in
  `services/config/defaults/gdpr.go`) and the centralised
  `api/common/student_access.go` helper
  (`DetermineStudentAccess` + `HasFullAccessByGroupID`).
  No new permission, no new setting, no new policy code.
- Live data — no client cache, no polling. SSE push + SWR
  `revalidateOnFocus` only.

**Out of scope (for v1)**
- New "all_staff" UX modes or admin overrides.
- Historical "who was in this room earlier today" view (the existing
  history block on the room detail page already covers that).
- Mobile-specific layout changes beyond what `StudentCard` already handles.
- Read paths for the deprecated `users.students` location flags.

## What the feature does (user-visible)

1. Pflegekraft opens `/rooms/{id}`.
2. Below the existing room/occupancy header, a new section
   **"Kinder im Raum"** renders:
   - One `StudentCard` per child currently in an active group in this room.
   - **Sorted alphabetically** by last name, then first name.
   - Header shows live count using **the true total** (`total_count`)
     so it matches what a colleague at the kiosk sees.
   - When `visible_count < total_count`, a subtle hint appears next to
     the count: "X von Y sichtbar · Du siehst nur Kinder aus deinen
     Gruppen." This is the explicit anti-irritation cue.
   - Live-update indicator (green/yellow/red dot, German labels) reuses
     the standard SSE connection indicator pattern already in use on
     MyRoom and OGS Groups (see `frontend/CLAUDE.md` → "Connection
     Indicator Pattern").
   - Empty state: "Aktuell keine Kinder im Raum."
3. Button **"In Kindersuche öffnen"** above the list links to
   `/students/search?room_id={id}`. Kindersuche opens with the room filter
   active and a removable filter chip.
4. Cards link through to the existing student detail page (current
   `StudentCard` default behaviour).

## Permissions contract

**Route gating** — `/api/rooms/{id}/students-present` is protected by
`authorize.RequiresPermission(permissions.RoomsRead)`, matching every
other read on `/api/rooms/*`. Without `rooms:read` the request gets
403 and the section is hidden in the UI.

**Per-row redaction** — applied inside the handler/service via the
existing `api/common/student_access.go` helper:

```go
access := common.DetermineStudentAccess(r, userContextSvc, settingsSvc, logger)
hasFullAccess := access.HasFullAccessByGroupID(student.GroupID)
```

This already encapsulates the three-way decision (admin / all_staff
scope / supervised group), keyed off the `gdpr.student_data_scope`
tenant setting.

| Caller | `gdpr.student_data_scope` | What they see on /rooms/{id} |
|---|---|---|
| Admin (`admin:*` / `*:*`) | any | All children, full names |
| Staff supervising ≥1 group in room | `group_supervisors_only` | All children in groups they supervise; for other groups, redacted row (id only, names null) — still counted |
| Staff supervising ≥1 group in room | `all_staff` | All children, full names |
| Staff supervising no group in room | `group_supervisors_only` | All redacted rows (count visible, no names) |
| Staff supervising no group in room | `all_staff` | All children, full names |
| Anyone without `rooms:read` | any | 403, section hidden in UI |

We always return one row per child (with names redacted to `null` when
not authorised) rather than collapsing to a count, so `total_count` and
`visible_count` stay consistent and the SSE-driven re-render logic
can diff by `student_id` regardless of access.

## Data freshness

The codebase pattern for live student/visit data is **SSE, not polling**
(see `frontend/CLAUDE.md` → "Real-Time Updates (SSE)"). Reuse it:

- Subscribe via the existing `useSSE("/api/sse/events")` hook.
- **Correction (2026-05-04):** `student_checkin` / `student_checkout`
  events do **not** carry `room_id` — only `active_group_id`
  (see `backend/services/active/visit_helpers.go::broadcastVisitCreated`
  and the EndVisit broadcast in `active_service.go`). The page therefore
  derives the set of active-group ids from the latest REST payload (via
  `uniqueActiveGroupIdsInRoom` in `lib/students-in-room-helpers.ts`) and
  triggers `mutate()` when an inbound event's `active_group_id` is in
  that set. New active groups starting in this room arrive as
  `activity_start` events, which **do** carry `event.data.room_id`, so
  we additionally `mutate()` when that room id matches the page's room
  — symmetric for `activity_end`. This bounds the chicken-and-egg
  problem to "session that started in this room AFTER the most recent
  refetch": the activity_start event closes that gap.
- Initial fetch via `useSWRAuth` with `revalidateOnFocus: true` (same
  default Kindersuche relies on for staleness on tab return).
- No `refreshInterval`. SSE provides the push; SWR provides the pull on
  focus. Polling would be inconsistent with how MyRoom and OGS Groups
  already do this.
- Backend SSE hub already broadcasts the relevant events
  (`backend/services/active/active_service.go`). No new event types
  required.

**Subscription scope caveat** — `useSSE` auto-subscribes the client to
its supervised active groups (see backend SSE handler). A staff member
who supervises no group in this room will not receive checkin/checkout
events for that room over SSE; they fall back to the
`revalidateOnFocus` refresh. That's acceptable for v1 — they only see
counts anyway, and the count refresh on tab return is enough.

## GDPR notes

- No localStorage cache. SWR's in-memory cache is fine and gets cleared
  on logout via NextAuth.
- Logging: never log student IDs or names at Info level from the new
  endpoint. Tenant ID + room ID + result count only.
- Response payload: redact names server-side for non-authorized students
  (return `null` first/last name, keep ID for count) — same shape as the
  existing visit-display redaction so the frontend handles it uniformly.
- Audit: no new audit events. Read paths are not audited today, no reason
  to start with this one.

---

## Implementation outline (subject to change)

### Backend

**New endpoint**

```
GET /api/rooms/{id}/students-present
Auth: rooms:read (route)  + per-row redaction via DetermineStudentAccess
Tenant-scoped via TenantTxMiddleware
```

Response shape — defined fresh as
`StudentsPresentInRoomResponse` in `backend/api/rooms/types.go`. There
is no existing `StudentWithVisitDisplay` to mirror; the per-row shape
borrows fields from `StudentResponse` (`api/students/types.go`) for
consistency, and the envelope adds `total_count` / `visible_count`.

```json
{
  "room_id": 42,
  "as_of": "2026-05-04T14:33:12Z",
  "total_count": 5,
  "visible_count": 3,
  "students": [
    {
      "student_id": 1234,
      "first_name": "Anna",
      "last_name": "Schmidt",
      "active_group_id": 88,
      "group_name": "Lesegruppe",
      "entry_time": "2026-05-04T14:01:55Z",
      "has_full_access": true
    },
    {
      "student_id": 1235,
      "first_name": null,
      "last_name": null,
      "active_group_id": null,
      "group_name": null,
      "entry_time": null,
      "has_full_access": false
    }
  ]
}
```

Redaction rules (applied server-side, never trust the client):
- `has_full_access = false` → `first_name`, `last_name`, `group_name`,
  `entry_time`, `active_group_id` all set to `null`. Only the stable
  `student_id` survives so the frontend can diff rows on SSE updates.
- `total_count` is the true count; `visible_count` is rows where
  `has_full_access = true`. Identical to the kiosk's view, which is
  the anti-irritation property we want.

Implementation path:
1. Handler in `backend/api/rooms/students_present_handlers.go`.
   - Calls `common.DetermineStudentAccess(r, userContextSvc,
     settingsSvc, logger)` once.
   - Calls the service.
   - Maps service rows → response rows, applying redaction via
     `access.HasFullAccessByGroupID(student.GroupID)`.
2. Service method on `services/active/Service`:
   `ListStudentsPresentInRoom(ctx, roomID int64) ([]StudentVisitView, error)`.
   - Returns the raw join (visit + group + student + person), no
     redaction. Redaction is the handler's job because that's where the
     request-scoped access context lives (matches how
     `api/students/api.go` does it today).
   - **Must NOT construct queries in the service** (backend rule 11).
3. New repository method on
   `backend/database/repositories/active/visits.go`:
   `ListActiveByRoomID(ctx context.Context, roomID int64) ([]VisitWithStudent, error)`.
   - Naming matches the existing sibling `CountActiveByRoomID` in the
     same file. Reuses the same `active.visits → active.groups →
     facilities.rooms` join already used by `FindByStudentAndTimeRange`.
   - Tenant scoping flows through the existing TenantTxMiddleware /
     RLS — no manual tenant filter in the SQL.
4. Wire route in `backend/api/rooms/api.go` with
   `authorize.RequiresPermission(permissions.RoomsRead)` (matching the
   sibling `/{id}/history` route).
5. Logging — slog only, GDPR-safe. Info: `room_id`, `tenant_id`,
   `total_count`, `visible_count`. Never student names or IDs at Info.
6. Tests: hermetic — fixtures for room, two active groups, one
   supervised by caller, one not. Assert redaction behaviour for both
   `gdpr.student_data_scope` values and for admin callers.

**Kindersuche `room_id` filter**

1. Extend `parseStudentListParams` in `backend/api/students/list_helpers.go`
   to read `room_id` (alongside the existing `group_id`, `school_class`,
   `location`, `search`, etc.).
2. `room_id` AND-combines with the other filters. A student can have at
   most one open visit at a time, so `room_id` + `group_id` is
   self-consistent (it just narrows further).
3. Push the filter into the existing students list query via the
   repository — students whose current open visit is in the given room.
4. The existing `listStudents` handler already runs all rows through
   `DetermineStudentAccess` + redaction. No new auth code.
5. Tests: list with `?room_id=X` returns only students currently in X,
   and respects `gdpr.student_data_scope`.

### Frontend

**Room detail page** (`frontend/src/app/[tenant]/(protected)/rooms/[id]/page.tsx`)

1. Add `<StudentsInRoomSection roomId={id} />` below the existing
   header, above the history block.
2. New component
   `frontend/src/components/rooms/students-in-room-section.tsx`:
   - `useSWRAuth` against `/api/rooms/{id}/students-present` for the
     initial paint and focus revalidation. No `refreshInterval`,
     no `localStorage` cache.
   - `useSSE("/api/sse/events")` subscribed alongside; `onMessage`
     triggers `mutate()` of the SWR key when the event is
     `student_checkin` / `student_checkout` AND the event's `room_id`
     matches the current page's room. The event payload already carries
     `room_id` directly — no client-side derivation of the active-group
     set required.
   - Renders `StudentCard` per visible (non-redacted) student,
     alphabetically sorted by last name, then first name. Pass
     `checkinMode={false}` and an explicit `onClick` that pushes to
     `/{tenant}/students/{id}` (the component does not navigate on its
     own).
   - Redacted rows render as a generic placeholder card ("Kind ohne
     Sichtbarkeit") so the count matches the kiosk view without
     leaking names.
   - SSE connection indicator in the header (reuse the documented
     pattern: green/yellow/red dot + German labels).
   - Empty / permission-limited / error states as in the contract.
3. "In Kindersuche öffnen" button:
   `Link href={`/{tenant}/students/search?room_id=${id}`}`.
4. Hide the entire section if the caller's session lacks `rooms:read`.

**Kindersuche** (`frontend/src/app/[tenant]/(protected)/students/search/page.tsx`)

1. Read `room_id` from `useSearchParams()` on mount, hydrate into
   filter state.
2. Pass `room_id` to `studentService.listStudents({ room_id })`.
3. Render a removable filter chip "Raum: {roomName}" — fetch room
   name via the existing `/api/rooms/{id}` endpoint or pass via query
   param `room_name` for instant render.
4. Removing the chip clears the param and re-fetches.

**Next.js proxy** (`frontend/src/app/api/rooms/[id]/students-present/route.ts`)

- Standard JWT-forwarding proxy following the existing pattern in
  `frontend/src/app/api/rooms/[id]/route.ts`.

### Type mapping

- Backend `int64` student/group/room IDs → frontend strings via the
  existing helpers in `lib/students-helpers.ts` /
  `lib/rooms-helpers.ts`.
- snake_case → camelCase mapping in the same helpers.
