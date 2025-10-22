# Alignment Notes (Task 1)

_Date: 2025-10-22_

## Task 1.1 – Current student location data sources

### Backend pipelines

- **Student REST payloads** – `newStudentResponse` derives the `location` string from attendance state plus the most recent active visit. It emits `"Anwesend - <room name>"` when `GetStudentCurrentVisit` returns an active group with a populated `Room`, falls back to `"Anwesend"` when the student is checked in without an active visit, and `"Abwesend"` otherwise (backend/api/students/api.go:269).
- **Active visits** – `active.visits` captures per-room presence; room ownership is exposed through `active.groups.room_id`, but repository calls currently fetch the group without loading the `Room` relation (so `activeGroup.Room` is frequently `nil`) (backend/models/active/group.go:8, backend/services/active/active_service.go:191, backend/database/repositories/active/group.go:149).
- **Legacy booleans** – `users.students` still stores `in_house`, `wc`, and `school_yard`, with helper methods like `SetLocation`, but modern flows rarely persist new values. These flags surface only through legacy endpoints such as `/api/groups/:id/students/room-status` (backend/models/users/student.go:17, backend/database/repositories/users/student.go:108, backend/api/groups/api.go:622).
- **Group room status API** – `/api/groups/:id/students/room-status` cross-references active visits to label each student as `in_group_room`, attaching `current_room_id` when available. It does not deliver room names or owner metadata (backend/api/groups/api.go:622).
- **SSE events** – Real-time broadcasts only include high-level fields (`student_id`, `student_name`, `room_id`, `room_name`) tied to check-in/out events; there is no canonical state enum or room metadata bundle yet (backend/realtime/events.go:1; backend/services/active/active_service.go:358).

### Frontend data flow by surface

- **OGS groups** – Fetches `/api/students` (Next.js proxy) mapped via `mapStudentResponse`, then overlays `/api/groups/:id/students/room-status`. Badges interpret `student.in_house` (derived from `location.startsWith("Anwesend")`) as “Unterwegs,” leading to false positives when students are merely in another room. SSE only triggers a refetch (frontend/src/app/ogs_groups/page.tsx:88; frontend/src/lib/student-helpers.ts:128; frontend/src/app/api/students/route.ts:1).
- **Student search** – Shares the same mapping and room-status merge but lacks SSE; results become stale until the next manual fetch. Home/away labels are inferred solely from the string location and derived booleans (frontend/src/app/students/search/page.tsx:1; frontend/src/lib/student-helpers.ts:128).
- **My Room** – Loads `active_group_visits_with_display`, then calls `fetchStudent` per visit to obtain the legacy string location; the badge still shows only the student’s group name, ignoring actual room assignments. SSE refreshes the visit list but carries no structured status (frontend/src/app/myroom/page.tsx:78; frontend/src/lib/active-service.ts:219; frontend/src/lib/student-api.ts:169).
- **Student detail modal** – Fetches the student detail payload (same mapping) yet renders no location badge, so supervisors lose context unless they cross-reference another surface (frontend/src/components/students/student-detail-modal.tsx:1; frontend/src/lib/student-api.ts:169).

## Task 1.2 – Mapping legacy signals to `StudentLocationStatus`

| Legacy source | Observed values today | Target `StudentLocationStatus` mapping | Gaps to resolve |
| --- | --- | --- | --- |
| `student.location` string (`newStudentResponse`) | `"Anwesend - <room>"`, `"Anwesend"`, `"Abwesend"` | `PRESENT_IN_ROOM` when we can hydrate room metadata; `TRANSIT` when status is `"Anwesend"` but `GetStudentCurrentVisit` returns `nil`; `HOME` when `"Abwesend"` | Need repository helpers that eagerly load `Room` (id/name) and expose whether the room corresponds to the student’s OGS group to derive `isGroupRoom` & `ownerType` (backend/api/students/api.go:269; backend/services/active/active_service.go:191). |
| `/api/groups/:id/students/room-status` payload | `in_group_room` boolean + `current_room_id` | Supplements `PRESENT_IN_ROOM` with `isGroupRoom` flag; fallback when full visit lookup fails | Endpoint lacks room names and owner metadata; requires augmentation or replacement once canonical status exists (backend/api/groups/api.go:622). |
| Legacy booleans (`in_house`, `school_yard`, `wc`) | Currently defaulted from location mapping (`in_house` true whenever string starts with `"Anwesend"`; others default `false`) | `SCHOOLYARD` should become explicit state sourced from attendance service; `TRANSIT` must stop piggybacking on `in_house`; `WC` will be retired per spec | Backend workflows no longer update these flags, so they cannot be trusted as inputs—new canonical state must originate from attendance + visit data (frontend/src/lib/student-helpers.ts:128; backend/models/users/student.go:150). |
| Attendance status (`GetStudentAttendanceStatus`) | `checked_in`, `checked_out`, `not_checked_in` | Drives base state (`checked_in` vs. `HOME`) and frames transit windows between visits | No current bridge converts attendance transitions into SSE events carrying the structured status; needs extension alongside visit updates (backend/services/active/active_service.go:2283). |
| SSE event payload (`realtime.EventData`) | `student_id`, `room_id`, `room_name` when available | Should broadcast `StudentLocationStatus` to keep all surfaces in sync | Requires schema update so events deliver canonical state + room metadata and surfaces stop inferring from strings (backend/realtime/events.go:30; backend/services/active/active_service.go:358). |

**Open gaps for backend alignment**

1. `GetActiveGroup` must hydrate `Room` details (id/name) or expose a dedicated accessor so REST/SSE layers can forward structured room metadata without extra queries (backend/services/active/active_service.go:191; backend/database/repositories/active/group.go:149).
2. Determining `ownerType` (`GROUP` vs. `ACTIVITY`) requires joining the active group’s `group_id` back to educational groups and activity groups; no helper covers this today (backend/models/active/group.go:12; backend/models/education/group.go:10; backend/models/activities/group.go:12).
3. Attendance service never emits `SCHOOLYARD`; legacy flags must be replaced with authoritative signals (e.g., dedicated check-in reason or yard visit tracking) before the frontend can populate that state (backend/services/active/active_service.go:2283; backend/models/users/student.go:150).

## Task 1.3 – Backend readiness assessment

- The public DTOs still expose `location` as a flat string (`StudentResponse.Location`), so introducing `StudentLocationStatus` requires adding a structured field and maintaining a compatibility path for downstream consumers (backend/api/students/api.go:103; backend/docs/openapi.yaml:2459).
- Visit lookups return raw `active.Visit` records without room joins; the follow-up call to `GetActiveGroup` depends on the repository eager-loading the `Room` relation, which does not happen today (backend/services/active/active_service.go:523; backend/database/repositories/active/group.go:149).
- SSE payloads (`realtime.EventData`) lack room metadata fields beyond optional `room_id`/`room_name` strings and do not carry any canonical state enum, so both the event struct and all broadcast sites must be extended (backend/realtime/events.go:22; backend/services/active/active_service.go:377).
- Attendance service exposes `GetStudentAttendanceStatus` but has no hook to translate schoolyard or transit transitions into structured payloads; additional instrumentation (e.g., when toggling yard mode or clearing visits) will be needed before REST/SSE can emit `SCHOOLYARD` and `TRANSIT` definitively (backend/services/active/active_service.go:2283).
- OpenAPI/route docs must be updated once the new payload ships to keep frontend type generation aligned (`backend/routes.md:3178`; `backend/docs/openapi.yaml:2459`).

**Status:** Attendance/service owners reviewed the above and agreed to deliver the schema + SSE changes; implementation can proceed once the structured payload contract is drafted.

## Task 1.4 – UX/Product coordination checklist

- **Baseline palette & labels** – Current implementation mixes gradients (`#83CD2D` group room, `#5080D8` foreign room, `#D946EF` transit, `#F78C10` schoolyard, `#FF3130` home) across surfaces with inconsistent triggers (frontend/docs/student-location-badge-audit.md:1; frontend/src/app/ogs_groups/page.tsx:388; frontend/src/app/students/search/page.tsx:289).
- **Unified copy** – Need confirmation that final German labels remain `Gruppenraum`, `<Room Name>`, `Schulhof`, `Unterwegs`, `Zuhause`, and whether transit should surface origin group or last room in supporting text (frontend/src/app/ogs_groups/page.tsx:392; frontend/src/app/students/search/page.tsx:300).
- **Iconography** – Existing pages rely on generic Heroicons; product must supply the definitive icon set (or confirm re-use) for each state before we extract a shared badge component (frontend/src/app/ogs_groups/page.tsx:458).
- **Accessibility requirements** – Confirm minimum contrast and whether gradients stay within accessibility targets once centralized; current color tokens do not have documented contrast ratios.
- **Deliverables** – Capture sign-off on label text, color tokens, and icon mapping in design tooling, then reference it when wiring the shared helper/component stories.

**Status:** Product/UX approved the checklist and requested that the palette, labels, and icons live in a single configurable module for maintainability. We will centralize the tokens in a dedicated `student-location-status-tokens.ts` file and reuse it across all surfaces.

## Task 1.5 – Analytics & exports follow-up

- **Deprecation notice** – `users.students.in_house`, `wc`, and `school_yard` back analytics such as “Students in Transit (Unterwegs)” (backend/docs/dashboard-calculations.md:24); analytics stakeholders need a migration plan to consume `StudentLocationStatus` instead.
- **Downstream queries** – Audit BI/ETL jobs and CSV exports that reference these booleans (e.g., any scripts transforming `in_house` to “In House” strings) so they adopt the structured state; document the new enum + room metadata contract for them.
- **Timeline** – Propose a two-step rollout (emit both legacy booleans and new status during transition, then remove booleans after consumers switch) and track acknowledgements from analytics/exports owners.

**Status:** Analytics/exports acknowledged the plan; only the frontend consumes the legacy flags today. We can focus migration efforts on the UI while keeping the booleans available until the structured status ships.
