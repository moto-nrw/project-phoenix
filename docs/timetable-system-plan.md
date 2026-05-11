# Timetable & Activity Planning System — Implementation Plan

**Status:** Draft for team discussion (consolidated from 4 iterations)
**Target branch:** `development`
**Author:** Yannick, with input from Christian and Flo
**Epic:** Timetables (Stundenpläne & Betreuungsplanung)

## Changelog

- **2026-04-13 (RFC):** Three-layer model (template → instance → live), class timetable, 5-phase migration, 6 settings.
- **2026-04-13 (Iteration 1):** Confirmed `is_spontaneous` flag (kept for spontaneous-to-template linking).
- **2026-04-13 (Iteration 2):** Devil's advocate review (E1–E10). Eliminated three-mode system. Nullable GroupID. Multi-room override. Missing children as core feature. Substitute endpoint. Conflict detection. Dual materialization. GDPR retention. Partial UNIQUE index.
- **2026-04-14 (Iteration 3):** Architecture session (E11–E14). Replaced class_timetable with per-student arrival schedules. Clean milestone vs. activity separation. Three independent data systems (arrival, timetable, pickup). Implicit care contract. Industry research validating design against 6 open-source SIS + 5 commercial products.
- **2026-04-15 (Iteration 4):** Team feedback from Christian and Flo (E15–E22). Calendar periods. A/B weeks. Enrollment validity. Three-field attendance model. Auto-start levels. Gap detection. Mensa rotation solved. Holiday care compatibility.
- **2026-04-21 (Iteration 5):** Post-B10 review. Open questions §4.1/§4.7/§4.8 resolved. B10 gap identified: `Complete()` must mark remaining `expected` students as `absent` (plan §6.2) — follow-up bundled with WP-B11.
- **2026-04-23 (Iteration 6):** E2E-Sweep-Findings nachgezogen. §6.1 praezisiert die Evaluation-Reihenfolge (Exception schlaegt existing-row-Dedupe, E2E-B1). §10 ergaenzt Query-Budget-Semantik (Handler-Ebene vs. End-to-End, E2E-C2). Siehe `backend/test/e2e/timetable/E2E_FINDINGS.md`.
- **2026-04-24 (Iteration 7):** F-Track gestartet. WP-F1 (Arrival Schedule Editor) shipped via #1306, gebuendelt mit Master-Detail-Refactor der Database-Studenten-Page und B1/B2-Polish (Listen-Enrichment via `include_arrival_times`, neue SSE-Events `student_updated` + `arrival_schedule_changed`, LocationBadge-Status "Kommt heute nicht", Bulk-DTO-Fix `arrival_time` → `expected_arrival`). Naechster Einstieg: F2/F3/F4 parallelisierbar.
- **2026-04-24 (Iteration 7, Klarstellung):** SSE-Backend-Emission fuer `instance_started/completed/cancelled/overdue` wurde bereits in B9 (#1294) mitgeliefert (in `instance_service.go` und `scheduler.go::runOverdueForTenant`). F7 reduziert sich damit auf reines Frontend-Wiring (sse-types + use-sse-Handler + Badge-Bindung) — kein neuer Backend-Code noetig.
- **2026-04-30 (Iteration 8):** F2-Realitaetscheck nach erstem Klicktest. Admin planner ist funktional weitgehend da (Monat/Woche, Perioden, Vorlagen, Termine, Start/Complete/Cancel, Konflikte), braucht aber UI-Polish vor Abschluss. Operativer Betreuer-Wert wird als Folge-WP geschnitten: geplante Instanzen in `/active-supervisions`, "Jetzt starten" im ±15-Minuten-Fenster, erwartete Kinder in der Aufsicht und SSE-Refresh ohne manuellen Reload.
- **2026-05-06 (Iteration 9):** F2 als Admin-Planungsoberflaeche abgeschlossen. Operatives Zielbild geschaerft: Timetable wird nicht nur Kalender, sondern verbindet Buero-Planung, Staff-Ansichten, PyrePortal/Geraete, spontane Aktivitaeten, Self-Check-in und Zeiterfassung. Neue Frontend-/Integration-WPs F13-F17 schneiden diese Arbeit explizit.

---

## 1. Goal

Replace paper-based daily planning in OGS after-school care with a digital system that bridges the gap between **planned activities** (templates) and **real-time attendance** (active sessions). Staff see "my day" before children arrive, check in children against a plan, and admins get plan-vs-reality reporting — all without losing the flexibility of spontaneous activities.

The system connects three independent scheduling domains — **Arrival** (when does the child come?), **Timetable** (what does the child do?), and **Pickup** (when does the child leave?) — into a unified student day view, while keeping each domain independently deployable.

The product goal after the admin planner is operational integration:

- The office plans the day or week: homework, Mensa, activities, free-play blocks, staff assignments, rooms, and expected children.
- Staff and devices see the right next action at the right time: "Start your planned activity now" when a matching instance exists, or "create a spontaneous activity" only when the school allows that in the current time block.
- Planned and spontaneous work both end up in the same timetable history, so the school can answer: what was planned, what actually ran, who supervised it, which children attended, and where the time was booked.
- Every role gets a calendar-shaped view of their day: staff see "what am I doing now/next/this week", admins see what is running, and student views can combine arrival, activities, and pickup.

## 2. Requirements Summary

**Timetable Core:**
- Weekly activity templates with automated materialization into daily instances
- Spontaneous activities (staff creates ad-hoc, no template required) — always available, no mode switch
- Instance lifecycle: `planned` → `active` → `completed` (or `cancelled`)
- Bridge to existing `active.*` live system: starting an instance creates an `active.group`
- Three-field attendance model: core status (system) + substatus (human context) + note (freetext)
- Missing children display as core v1 feature (expected vs. present vs. absent)
- Multi-room support via override pattern (primary room on instance, optional overrides on staff/students)

**Planning & Scheduling:**
- Calendar periods (school year, semester, holiday, custom) with A/B week support
- Enrollment validity (`valid_from`/`valid_until`) enabling semester rollover without data loss
- Supervisor validity (same pattern) — who is assigned to which activity in which period
- Materialization: automated scheduler + manual admin button, with merge strategy protecting active instances
- Activity exceptions (cancellation, time/room changes) consumed during materialization

**Arrival Schedules:**
- Per-student expected arrival times per weekday (mirrors existing pickup schedule pattern)
- Bulk endpoint for class-based entry using `students.school_class` as filter
- Date-specific exceptions (Wandertag, Hitzefrei, schedule change)
- Independent from timetable — deployable first (WP-B1 / WP-B2, shipped in #1280)

**Staff Management:**
- One-click substitute assignment across all instances for a day
- Gap detection: API shows instances with zero or insufficient staff
- Staff absence entity deferred to V2 (no admin UI yet)

**Operations:**
- Conflict detection with soft warnings (room overlap, staff/student double-booking)
- Auto-start in three levels: passive (UI indicators), active (SSE events), automatic (scheduler)
- Device-aware start flow: PyrePortal and other staff devices can start the planned instance that matches staff, time, room, and tenant context
- Time-block rules for spontaneous activities: schools can define windows where ad-hoc activities are allowed, suggested, or blocked
- Timetable visibility outside the planner: active supervision, staff day/week, student day/week, and reporting all read from the same plan-vs-reality model
- Time tracking integration: started/completed instances can feed working-time context without making timetable the canonical time-clock system
- GDPR retention setting for completed timetable data
- Per-tenant configurable via 7 settings in the existing registry system

## 3. Key Decisions (confirmed)

22 design decisions made across 4 review iterations. All confirmed by Yannick.

| # | Topic | Decision |
|---|-------|----------|
| **E1** | **Modes** | One unified system. No `flexible`/`planned`/`hybrid` mode switch. Materialization is optionally activatable, spontaneous activities are always available. |
| **E2** | **`active.groups.group_id`** | Becomes nullable. Spontaneous instances have no template, so the live group has no `group_id`. Go model: `int64` → `*int64`. |
| **E3** | **Multi-room** | `room_id` stays on instance as primary room (NOT NULL). `instance_staff` and `instance_students` get optional `room_id` override. NULL = primary room. Covers the Lernzeit split case (~10% of instances) without a junction table. |
| **E4** | **Attendance sync** | Application code updates `instance_students.status` at check-in/check-out time. No DB triggers. Check-in handler gains instance-awareness via `activity_instances.active_group_id`. |
| **E5** | **Missing children** | Core v1 feature, not nice-to-have. Instance student list shows expected/present/missing counts with names. |
| **E6** | **Substitution** | One-click endpoint: replace staff member across all instances for a date. Original entry kept with `is_absent=true` for reporting. Substitute entry created with `is_substitute=true`. |
| **E7** | **Conflict detection** | Soft warnings at instance create/edit time. Room overlap, staff double-booking, student double-booking. Warnings in API response, no hard blocks. |
| **E8** | **Materialization** | Both scheduler (automated, setting-gated) and manual admin button. Merge strategy: override `planned`, protect `active`/`completed`/`cancelled`. |
| **E9** | **GDPR retention** | Own setting `gdpr.timetable_retention_days` (default 365). Completed/cancelled instances cleaned independently from existing attendance cleanup. |
| **E10** | **UNIQUE constraint** | Partial index only for template-based instances (`WHERE activity_group_id IS NOT NULL`). PostgreSQL NULL != NULL means spontaneous instances cannot have table-level uniqueness. |
| **E11** | **Arrival schedules** | Replaces `class_timetable` entirely. Per-student, per-weekday — exact mirror of existing pickup schedule system. No new `school_classes` entity needed. Bulk endpoint uses `students.school_class` string as filter. |
| **E12** | **Milestones vs. activities** | Arrival and pickup are point-in-time milestones (no lifecycle, no supervisor). Activities are time-span instances (start/end, lifecycle, staff). Milestones are NOT modeled as activity instances. |
| **E13** | **Three independent systems** | Arrival, timetable, and pickup have no foreign keys between them. Integration is read-side only (student day view aggregates all three). Different lifecycles, granularities, and owners. |
| **E14** | **Care contract** | Implicitly defined by arrival + pickup schedules. "Is Max expected today?" = has arrival entry for this weekday AND no cancellation exception. No separate entity. |
| **E15** | **Calendar periods** | `schedule.calendar_periods` table created now, but nullable FK everywhere. Auto-generated default school-year period per tenant. No admin UI until holiday care. Period types: `school_year`, `semester`, `holiday`, `custom`. |
| **E16** | **A/B weeks** | `week_pattern` on `activities.schedules` (0=every, 1=A, 2=B). Cycle config on calendar period (`week_cycle_length`, `week_cycle_anchor`). Day-based difference calculation for correct year-boundary handling. |
| **E17** | **Enrollment validity** | `valid_from`/`valid_until` on enrollments and supervisors. Enables semester rollover without losing history. Partial UNIQUE: only unbounded enrollments must be unique. Bulk rollover endpoint. |
| **E18** | **Three-field attendance** | `status` (expected/present/absent — system-controlled), `substatus` (late/excused/sick/field_trip/other — human-controlled), `note` (freetext, max 500). TEXT with CHECK constraint, not DB ENUM. Auto-detection of `late` substatus at check-in. |
| **E19** | **Auto-start** | Three independently configurable levels: Passive (UI indicators, MVP — WP-F3), Active (SSE events to assigned staff — WP-F7), Automatic (scheduler creates `active.group`, per setting — WP-F10). |
| **E20** | **Substitute detection** | Gap-detection query (MVP) — API endpoint shows instances with zero/insufficient staff. Staff-absence entity (`schedule.staff_absences`) deferred until admin UI exists. |
| **E21** | **Mensa rotation** | Already solved by existing templates + A/B weeks. Daily rotation via different weekday schedules, weekly rotation via `week_pattern`. No additional model needed. |
| **E22** | **Holiday care** | All schema decisions keep holiday care possible without schema changes: nullable `calendar_period_id`, `activities.schedules.weekday` allows 1–7 (not restricted to 1–5), templates not bound to school weeks, enrollment validity scoping. Note: arrival schedules stay 1–5 (school days only) — holiday arrivals use a different pattern. |

## 4. Open Questions for Team Review

1. ~~**Spontaneous visitors** — child checks into an instance but is not in `instance_students`. Create a new entry with `status=present`? Or track only in `active.visits`?~~ **Resolved (2026-04-21):** No schema change. Spontaneous check-ins are accepted as valid. `instance_students` stays unchanged (only planned students). The B11 student-day aggregator joins `instance_students` with `active.visits` via `active_group_id`; students in `active.visits` but not in `instance_students` surface in the response with `is_unplanned: true`. Counting "who was here?" happens read-side, not write-side. Nice side effect: `Complete()` mark-as-absent logic stays trivial (`UPDATE instance_students SET status='absent' WHERE status='expected'`) — no need to distinguish planned-missing from spontaneous-attending.
2. **"Re-plan week" UI** — confirmation dialog with diff showing what will be overwritten? Or simpler "are you sure?" dialog?
3. **Arrival bulk: overwrite or merge?** — when bulk-setting arrival times for class "3a" and Max already has individual times, overwrite silently or warn?
4. **Arrival notes** — do we need `student_arrival_notes` analog to `student_pickup_notes`? Or does the `reason` field on exceptions suffice?
5. **`early_pickup` substatus** — needed? Or covered by the existing pickup exception system?
6. **Period overlap rules** — can active periods overlap (e.g., "Schuljahr 2026/27" + "Projektwoche Mai" both active)? If yes, materialization must pick the more specific period's templates.
7. ~~**`minimum_staff` field** — does `activity_instances` need a `minimum_staff SMALLINT DEFAULT 1` for gap detection? Or is `COUNT(staff) < 1` sufficient for MVP?~~ **Resolved (2026-04-21):** No field for MVP. WP-B12 uses `COUNT(instance_staff WHERE NOT is_absent) < 1` as the gap-detection predicate. An optional `minimum_staff` column can be added later if the planning use case appears, without breaking existing data.
8. ~~**Cross-dependency with enrollment plan** — Chris's enrollment plan introduces `platform.school_years` and `users.student_year_assignments`. Care offering selections on approved enrollments may need to map to `activities.student_enrollments`. Should we plan this interface now? (See §7.2)~~ **Resolved (2026-04-21):** Proceed independently on both sides. If the interface shape between enrollment care-offerings and `activities.student_enrollments` needs reconciliation later, the migration cost is accepted. No blocking dependency.

---

## 5. Architecture

### 5.1 Three-Layer Model

```
┌─────────────────────────────────────────────────────┐
│  TEMPLATE LAYER (planning — what happens weekly?)   │
│                                                     │
│  activities.groups        → Activity definition     │
│  activities.schedules     → Weekday + time window   │
│  activities.student_enrollments → Enrolled students │
│  activities.supervisors   → Assigned staff          │
│  schedule.calendar_periods → A/B weeks, semesters   │
└──────────────────────┬──────────────────────────────┘
                       │ Materialization
                       │ (scheduler, weekly)
                       ▼
┌─────────────────────────────────────────────────────┐
│  INSTANCE LAYER (concrete day — what happens today?)│
│                                                     │
│  schedule.activity_instances  → Entry for date X    │
│  schedule.instance_staff      → Assigned staff      │
│  schedule.instance_students   → Expected children   │
└──────────────────────┬──────────────────────────────┘
                       │ Start (staff taps button)
                       ▼
┌─────────────────────────────────────────────────────┐
│  LIVE LAYER (realtime — who is where now?)          │
│  *** ALREADY EXISTS ***                             │
│                                                     │
│  active.groups            → Running session         │
│  active.visits            → Child checked in/out    │
│  active.group_supervisors → Staff in session        │
└─────────────────────────────────────────────────────┘
```

**Independent from the timetable, three point-in-time systems frame the day:**

```
ARRIVAL                     TIMETABLE                    PICKUP
(when does the child come?) (what does the child do?)    (when does the child leave?)

student_arrival_schedules   activity_instances            student_pickup_schedules
student_arrival_exceptions  instance_staff/students       student_pickup_exceptions

Per student, per weekday    Per activity, per date        Per student, per weekday
Point-in-time               Time span                     Point-in-time
No lifecycle                planned→active→completed      No lifecycle
No FK to each other         No FK to arrival/pickup       No FK to each other
```

### 5.2 Domain model

**New schema: `schedule` additions (7 tables)**

```
schedule.calendar_periods
  id, tenant_id, name, period_type ('school_year'|'semester'|'holiday'|'custom'),
  start_date, end_date,
  week_cycle_length SMALLINT DEFAULT 1,  -- 1=no alternation, 2=A/B
  week_cycle_anchor DATE,                -- reference date for "this is week A"
  is_active BOOLEAN DEFAULT false,
  created_at, updated_at
  UNIQUE(tenant_id, name)

schedule.activity_instances
  id, tenant_id, date,
  activity_group_id (nullable FK → activities.groups),  -- NULL = spontaneous
  calendar_period_id (nullable FK → calendar_periods),
  title, description, start_time TIME, end_time TIME,
  room_id (NOT NULL FK → facilities.rooms),             -- primary room, always set
  status ('planned'|'active'|'completed'|'cancelled') DEFAULT 'planned',
  active_group_id (nullable FK → active.groups),        -- bridge to live layer
  is_spontaneous BOOLEAN DEFAULT false,
  notes, created_by, started_by, started_at, completed_at, created_at, updated_at
  PARTIAL UNIQUE (tenant_id, date, activity_group_id, start_time)
    WHERE activity_group_id IS NOT NULL

schedule.instance_staff
  id, tenant_id, instance_id (FK CASCADE), staff_id (FK → users.staff),
  room_id (nullable FK → facilities.rooms),  -- NULL = primary room of instance
  is_primary BOOLEAN DEFAULT false,
  is_substitute BOOLEAN DEFAULT false,
  is_absent BOOLEAN DEFAULT false,
  created_at
  UNIQUE(instance_id, staff_id)

schedule.instance_students
  id, tenant_id, instance_id (FK CASCADE), student_id (FK → users.students),
  room_id (nullable FK → facilities.rooms),  -- NULL = primary room
  status ('expected'|'present'|'absent') DEFAULT 'expected',      -- system-controlled
  substatus ('late'|'excused'|'sick'|'field_trip'|'other'|NULL),  -- human/auto-set
  note TEXT (max 500, nullable),                                   -- freetext
  checked_in_at TIMESTAMPTZ,
  created_at, updated_at
  UNIQUE(instance_id, student_id)

schedule.activity_exceptions
  id, tenant_id, activity_group_id (FK), exception_date,
  exception_type ('cancelled'|'modified'),
  start_time, end_time, room_id (all nullable — only for 'modified'),
  reason, created_by, created_at
  UNIQUE(tenant_id, activity_group_id, exception_date)

schedule.student_arrival_schedules
  id, tenant_id, student_id (FK), weekday (1–5),  -- Mon–Fri only (OGS school days)
  expected_arrival TIME,
  notes VARCHAR(500), created_by (FK → users.staff),
  created_at, updated_at
  UNIQUE(tenant_id, student_id, weekday)

schedule.student_arrival_exceptions
  id, tenant_id, student_id (FK), exception_date,
  expected_arrival TIME (NULL = child not coming today),
  reason VARCHAR(255), created_by (FK → users.staff),
  created_at, updated_at
  UNIQUE(tenant_id, student_id, exception_date)
```

All new tables get RLS policies matching the existing tenant-isolation pattern.

**Modifications to existing tables**

```
activities.groups
  + type TEXT NOT NULL DEFAULT 'activity'
    -- 'activity' (AG), 'care' (Mensa, Lernzeit, Freispiel), 'external' (DAZ, Musik)
  + education_group_id BIGINT (nullable FK → education.groups)
  + is_template BOOLEAN NOT NULL DEFAULT true

activities.schedules
  + week_pattern SMALLINT NOT NULL DEFAULT 0    -- 0=every, 1=A, 2=B
  + calendar_period_id BIGINT (nullable FK → calendar_periods)

activities.student_enrollments
  enrollment_date → RENAMED TO valid_from
  + valid_until DATE (nullable — NULL = unbounded)
  + calendar_period_id BIGINT (nullable FK)
  UNIQUE changed → partial index WHERE valid_until IS NULL

activities.supervisors
  + valid_from DATE NOT NULL DEFAULT CURRENT_DATE
  + valid_until DATE (nullable)
  + calendar_period_id BIGINT (nullable FK)
  UNIQUE changed → partial index WHERE valid_until IS NULL

active.groups
  group_id: NOT NULL → NULLABLE (*int64 in Go)
```

**Summary: 7 new tables, 5 modified tables, 0 removed.**

Removed vs. original RFC: `schedule.class_timetable` and `schedule.class_timetable_exceptions` — replaced by `student_arrival_schedules/exceptions` (E11).

### 5.3 Service layer

- **`services/schedule/arrival_service.go`** — CRUD for arrival schedules + exceptions. Bulk upsert by class. Resolution logic (exception → schedule → none). Soft warnings when arrival exceptions conflict with planned instances.
- **`services/schedule/calendar_period_service.go`** — CRUD for periods. Auto-create default school-year period on first access. A/B week resolution.
- **`services/schedule/instance_service.go`** — Instance CRUD. Spontaneous creation. Start/complete/cancel lifecycle. Conflict detection (room, staff, student overlaps). Bridge to `active.*` on start.
- **`services/schedule/materialization_service.go`** — Scheduler job + manual endpoint. Reads templates, applies exceptions, filters by enrollment/supervisor validity and A/B week pattern, creates instances with staff/students. Merge strategy: override `planned`, protect `active`/`completed`/`cancelled`.
- **`services/schedule/attendance_service.go`** — Attendance sync at check-in/check-out. Three-field model updates. Auto-detection of `late` substatus. Instance-end handler marks remaining `expected` as `absent`.
- **`services/schedule/substitute_service.go`** — One-click substitution across all instances for a date. Gap detection query.
- **`services/schedule/conflict_service.go`** — Room overlap, staff double-booking, student double-booking checks. Returns `[]Conflict` with soft warnings.
- **`services/schedule/cleanup_service.go`** — GDPR retention cleanup for completed/cancelled instances older than `gdpr.timetable_retention_days`.

### 5.4 API surface

**Admin / Office:**

```
GET    /api/timetable/instances?week=2026-W38         List instances for a week
POST   /api/timetable/instances                        Create spontaneous instance
PUT    /api/timetable/instances/{id}                   Edit instance (room, time, staff)
DELETE /api/timetable/instances/{id}                   Cancel instance (status → cancelled)
POST   /api/timetable/instances/{id}/start             Start → creates active.group
POST   /api/timetable/instances/{id}/complete          Complete → closes active.group
POST   /api/timetable/materialize                      Manual materialization (week param)
POST   /api/timetable/substitute                       One-click substitute for a day
GET    /api/timetable/gaps?date=...&date_to=...        Gap detection (staffing gaps)
GET    /api/timetable/exceptions                       Activity exceptions
POST   /api/timetable/exceptions                       Create exception (cancel/modify)
GET    /api/timetable/periods                          Calendar periods (MVP: GET only)
POST   /api/timetable/periods                          Create period (needed for holiday care)
PUT    /api/timetable/periods/{id}                     Edit period
DELETE /api/timetable/periods/{id}                     Delete period
```

**Staff (Mobile):**

```
GET    /api/timetable/my-day?date=2026-09-15           "My Day" — instances I'm assigned to
GET    /api/timetable/my-week                          "My Week" overview
GET    /api/timetable/instances/{id}/students           Expected + present + missing children
POST   /api/timetable/instances/{id}/checkin/{student}  Check in child
POST   /api/timetable/instances/{id}/checkout/{student} Check out child
```

**Student Profile:**

```
GET    /api/timetable/student/{id}/day?date=...        Student's day (aggregates arrival + instances + pickup)
GET    /api/timetable/student/{id}/week                Student's week
```

**Arrival Schedules:**

```
GET    /api/students/{id}/arrival-schedules             Per-student arrival schedule
PUT    /api/students/{id}/arrival-schedules             Upsert per-student schedules
POST   /api/students/arrival-schedules/bulk             Bulk upsert by school_class
GET    /api/students/{id}/arrival-exceptions            Arrival exceptions for student
POST   /api/students/{id}/arrival-exceptions            Create arrival exception
```

**Enrollment Management:**

```
POST   /api/activities/enrollments/semester-rollover   Bulk semester rollover
GET    /api/activities/enrollments?valid_at=2026-04-15 Filter enrollments by validity date
```

### 5.5 Frontend pages

| Page | Purpose |
|------|---------|
| `/{tenant}/admin/timetable` | Weekly planner grid (weekdays × time slots, color-coded by type) |
| `/{tenant}/admin/timetable/exceptions` | Activity exception manager |
| `/{tenant}/admin/arrivals` | Arrival schedule editor (bulk by class + individual overrides) |
| `/{tenant}/admin/calendar-periods` | Period management (deferred until holiday care) |
| `/{tenant}/staff/my-day` | Staff's daily view — timeline with instance cards, start/complete buttons |
| `/{tenant}/staff/instances/{id}` | Instance detail with check-in list, expected/present/missing |
| `/{tenant}/students/{id}/day` | Student day view (arrival + instances + pickup aggregated) |

**Admin weekly planner:** Grid view with X-axis = weekdays, Y-axis = time slots. Color-coded by type (care=blue, activity=green, external=orange using MOTO brand colors from `LOCATION_COLORS`). Click on instance → detail panel with staff + children. "Materialize next week" button.

**Staff "My Day":** Vertical timeline. Each instance as card with title, time, room, expected children count. "Start" button with overdue indicators (passive auto-start, E19). Spontaneous activity creation button.

**Student day view:** Aggregated from three sources:

```
12:50  ○ Expected arrival          ← arrival_schedules
13:00  ━━━━ Mensa ━━━━━━ 13:45   ← activity_instance
13:45  ━━━━ Lernzeit ━━━━ 14:30  ← activity_instance
14:30  ━━━━ Freispiel ━━━ 15:15  ← activity_instance
15:30  ○ Pickup                    ← pickup_schedules
```

### 5.6 Settings

7 settings registered in `services/config/defaults/timetable.go`. New "Timetable" category in the operations tab.

**Operations Tab** — WritePermission: `config:update`

| Key | Type | Default | DependsOn |
|-----|------|---------|-----------|
| `timetable.materialization_enabled` | boolean | `false` | — |
| `timetable.materialization_weekday` | select | `5` (Fri) | materialization_enabled eq true |
| `timetable.materialization_weeks_ahead` | number | `1` (min:1, max:4) | materialization_enabled eq true |
| `timetable.auto_start_planned` | boolean | `false` | — |
| `timetable.overdue_threshold_minutes` | number | `5` (min:1, max:30) | — |
| `timetable.show_expected_children_count` | boolean | `true` | — |

**GDPR Tab** — WritePermission: `config:manage`

| Key | Type | Default | DependsOn |
|-----|------|---------|-----------|
| `gdpr.timetable_retention_days` | number | `365` (min:30, max:1825) | data_cleanup_enabled eq true |

**Note:** A/B weeks are NOT a tenant setting — they are configured per calendar period (`week_cycle_length`, `week_cycle_anchor`). Different periods can have different cycle lengths (school year: A/B, holidays: no alternation).

---

## 6. Core Flows & Integration

### 6.1 Materialization

```
Weekly (configurable day, default Friday):

1. For each active tenant:
2.   For each active calendar period:
3.     For each template group (activities.groups WHERE is_template = true):
4.       Skip if template.calendar_period_id IS NOT NULL AND ≠ current period
         (templates without period = apply to ALL active periods)
5.       Load schedules → weekdays + time windows
6.       For each weekday of next week:
7.         Check A/B week pattern against period cycle
8.         Check activity_exceptions → cancel/modify?
9.         Skip if week_pattern doesn't match current week
10.        Create activity_instance (title, room, times from template; overrides from exception)
11.        Copy supervisors → instance_staff (filtered by valid_from/valid_until)
12.        Copy enrollments → instance_students (filtered by validity, status='expected')
```

**Merge strategy on re-materialization:**

| Instance status | Behavior |
|-----------------|----------|
| `planned` | Override — deleted and recreated from template |
| `active` | No change — running session stays untouched |
| `completed` | No change — historical data preserved |
| `cancelled` | No change — stays cancelled |

**Evaluation order per candidate** (wichtig, sonst uneindeutig): fuer jeden materialization-Kandidaten wird zuerst die Exception-Tabelle geprueft, erst danach die Dedupe gegen existierende Instanzen. Folge: eine `cancelled` oder `modified` Exception gewinnt immer ueber eine bereits existierende `planned` Row. Ein re-materialize ueber einen gecancelten Termin fuehrt zu `skipped_exception`, nicht `skipped_existing`. Das schuetzt vor dem Fall, dass ein versehentlicher Re-Materialize eine absichtlich gecancelte Aktivitaet wieder auferweckt. (E2E-B1, Flow B Schritt 3.)

**A/B week resolution:**

```go
func shouldMaterialize(schedule, instanceDate, period) bool {
    if schedule.WeekPattern == 0 { return true }     // every week
    if period.WeekCycleLength <= 1 { return true }    // no alternation

    daysDiff := instanceDate.Sub(period.WeekCycleAnchor).Hours() / 24
    weeksDiff := int(daysDiff) / 7
    currentPattern := ((weeksDiff % period.WeekCycleLength) +
                       period.WeekCycleLength) % period.WeekCycleLength + 1
    return currentPattern == schedule.WeekPattern
}
```

Day-based difference (not ISO week numbers) prevents year-boundary wrapping bugs.

### 6.2 Check-in & attendance sync

When staff taps a child in a running instance:

1. Create `active.visit` (existing NFC/app flow)
2. Find active instance via `activity_instances.active_group_id` matching the `active.group`
3. Update `instance_students SET status='present', substatus=determineSubstatus(), checked_in_at=NOW()`
4. On check-out: `active.visit.exit_time` is set (existing flow). `instance_students.status` stays `present`.

**Substatus auto-detection** (must not overwrite manual entries):

```go
func determineSubstatus(checkinTime, instanceStartTime, existingSubstatus) {
    if existingSubstatus != nil { return existingSubstatus }  // preserve manual entry
    if checkinTime.After(instanceStartTime) { return "late" }
    return nil
}
```

**At instance end:** Remaining `expected` students are marked `absent` (no substatus = unexcused).

**Spontaneous visitor:** Child not in `instance_students` checks in → open question (§4.1).

### 6.3 Substitution & gap detection

**One-click substitute:**

```
POST /api/timetable/substitute
{ "absent_staff_id": 42, "substitute_staff_id": 55, "date": "2026-09-15" }
```

Logic: find all `instance_staff` for `staff_id=42` on that date. For each: mark original as `is_absent=true`, create new entry for substitute with `is_substitute=true` (inheriting `room_id`).

**Gap detection:**

```
GET /api/timetable/gaps?date=2026-04-16&date_to=2026-04-20
```

Returns instances where `COUNT(instance_staff WHERE NOT is_absent) < 1` (or `< minimum_staff` if that field exists). Grouped into `gaps` (zero staff) and `understaffed`.

### 6.4 Three independent data systems

Arrival, timetable, and pickup are architecturally independent. No foreign keys cross between them. Integration happens only on the read side:

**Student day view** (`GET /api/timetable/student/{id}/day`) aggregates:
- Arrival schedule/exception for that student + weekday → expected arrival time
- Activity instances where student is in `instance_students` → time blocks
- Pickup schedule/exception for that student + weekday → pickup time

**Arrival → timetable warnings** (read-side only): when a staff member creates an arrival exception, the service checks if any activity instances start before the new arrival time → returns soft warnings ("Max will be late for Lernzeit by 15 minutes").

**Staff instance view** enriches with arrival data: students with arrival exceptions/schedules after instance `start_time` are shown as `expected_late` in the API response.

**Care contract is implicit:** "Is Max expected today?" = has arrival entry for this weekday AND no cancellation exception. No separate entity needed.

### 6.5 Enrollment system cross-dependencies

Chris's enrollment plan (PR #1270) introduces shared concepts that interact with the timetable system:

| Enrollment Plan Concept | Timetable Interaction |
|------------------------|----------------------|
| `platform.school_years` | Separate from `schedule.calendar_periods` but related. School years = administrative (enrollment, student assignments). Calendar periods = scheduling (when activities repeat, A/B weeks). Can map 1:1 in practice. |
| `users.students.status` (pending/active/inactive) | Materialization must filter for `status='active'` students. Pending students from enrollment should not appear in instance_students. |
| `users.student_year_assignments` (grade, class) | Can replace `students.school_class` string for arrival bulk endpoints in the future. Currently: bulk arrival uses `school_class` string. |
| Care offerings on approved enrollments | **Open question (§4.8):** approved care selections (e.g., "child enrolled in Lernzeit + Mensa") may map to `activities.student_enrollments`. This interface should be planned before either system ships. |

**No blocking dependency:** Both systems can proceed independently. The shared `platform.school_years` table from the enrollment plan does not conflict with `schedule.calendar_periods` — they serve different purposes. However, the care-offering → enrollment mapping should be discussed before WP-B5 ships (instance tables) or before the equivalent point in the enrollment plan.

### 6.6 Existing system integration points

| Existing System | Integration |
|-----------------|-------------|
| **`active.*` (live sessions)** | Bridge: starting an instance creates `active.group` + `active.group_supervisors` from `instance_staff`. `activity_instances.active_group_id` links the two. Existing NFC check-in flow stays unchanged — new attendance sync runs alongside. |
| **`activities.*` (templates)** | Extended, not replaced. `activities.groups` gets `type`, `is_template`, `education_group_id`. `activities.schedules` gets `week_pattern`, `calendar_period_id`. Existing activity management UI continues to work. |
| **Pickup schedules** | Arrival schedules mirror the exact same pattern (`student_*_schedules` + `student_*_exceptions`). Same model, repo, service, API shape. Can share frontend components. |
| **Settings system** | 7 new settings following existing registry pattern. All in `services/config/defaults/timetable.go`. Auto-rendered in settings UI. |
| **Scheduler** | New jobs: materialization (weekly), auto-start (per-minute check), GDPR cleanup (daily). Uses existing `forEachTenantSettings()` iteration pattern. |
| **SSE/realtime** | Auto-start level 2 adds `instance_due` and `instance_overdue` event types to existing realtime package. No new infrastructure. |
| **GDPR cleanup** | New `gdpr.timetable_retention_days` setting. Cleanup job for completed/cancelled instances. `active.groups` + `active.visits` cleaned separately by existing job. |
| **PyrePortal (IoT kiosk)** | No direct impact. IoT check-in handler gets instance-awareness (E4) but the existing NFC flow stays unchanged. No endpoint changes. |

---

## 7. Delivery Plan

Two tracks: **all backend first, frontend after.** Each item is a **Work Package (WP)** — a planning unit, not a GitHub PR number. One GitHub PR may bundle multiple WPs, and one WP may span multiple GitHub PRs.

### Backend Track

#### B1 — Schema foundations (✅ shipped)

- [x] **WP-B1** — Arrival schedules tables + models + repos + service → GitHub #1280
- [x] **WP-B2** — Arrival API endpoints (CRUD + bulk by class) + exception warnings → GitHub #1280
- [x] **WP-B3** — `schedule.calendar_periods` table + model + default-period auto-creation → GitHub #1281
- [x] **WP-B4** — Template extensions: `week_pattern` + `calendar_period_id` on `activities.schedules`, `valid_from`/`valid_until` + `calendar_period_id` on enrollments + supervisors, `enrollment_date` → `valid_from` rename, partial UNIQUE indexes → GitHub #1281

#### B2 — Activity instances + materialization (core)

- [x] **WP-B5** — `activity_instances`, `instance_staff`, `instance_students`, `activity_exceptions` tables + models + repos → GitHub #1283
- [x] **WP-B6** — `active.groups.group_id` NOT NULL → NULLABLE migration + Go model update + NULL-safe consumer updates → GitHub #1286
- [x] **WP-B7** — Timetable settings (7 entries) registered in config system → GitHub #1286
- [x] **WP-B8** — Materialization service + scheduler job (A/B week + validity filtering) + manual endpoint → GitHub #1293
- [x] **WP-B9** — Instance lifecycle: start (→ `active.group` bridge), complete, cancel + conflict detection service → GitHub #1294
  - **Bundled SSE-Emission (Backend-Teil von F7):** B9 emittiert die vier Instance-Events `EventInstanceStarted` / `EventInstanceCompleted` / `EventInstanceCancelled` (aus `instance_service.go`) und `EventInstanceOverdue` (aus `scheduler.go::runOverdueForTenant`, einmal pro planned-Instance nach Ueberschreiten von `timetable.overdue_threshold_minutes`, deduped pro Instance/Tag). Frontend-Wiring fehlt noch (siehe F7).
- [x] **WP-B10** — Attendance sync (E4) + three-field attendance model (E18: status / substatus / note) → GitHub #1295

#### B3 — Aggregation + Ops

- [x] **WP-B11** — Student day API (aggregates arrival + instances + pickup) → GitHub #1300
  - **Bundled B10 fix (plan §6.2):** `Complete()` now flips remaining `expected` students to `absent` inside the tenant tx. `Cancel()` intentionally untouched, since a cancelled instance never ran, so "absent" would be a false claim.
  - **Spontaneous-visitor handling (§4.1):** `active.visits` joined to `instance_students` via `active_group_id`; unplanned attendance surfaces as `is_unplanned: true` for active/completed instances only.
  - **Follow-ups solved in the same PR:** /week batched into single-range queries (no per-day N+1), `ScheduledInstanceRow` migrated to `models/schedule/`, enrolled+visit dedup test added, DST-safe inclusive day count, `CanReadStudent` extracted to `auth/authorize/`.
- [x] **WP-B12**: Gap detection endpoint + substitute endpoint → GitHub #1303
  - **Gaps:** `GET /api/timetable/gaps?date=...&date_to=...` (max 14 Tage, heute plus Zukunft). Instanzen mit `status IN (planned, active)` und `COUNT(instance_staff WHERE is_absent=false) = 0`. Bulk-Query mit GROUP BY, kein N+1.
  - **Substitute:** `POST /api/timetable/substitute { absent_staff_id, substitute_staff_id, date }`. Für jede instance_staff-Row am Datum: Original auf is_absent=true, neue Row mit is_substitute=true (room_id geerbt, is_primary=false). Für active Instanzen zusätzlich `active.group_supervisors` synchronisiert.
  - **Atomarität:** Dry-Run-First-Pattern (Phase A klassifiziert ohne Writes, Phase B schreibt erst wenn alle Checks bestanden). Löst den 409-bei-4xx-Tx-Middleware-Fall, bei dem partielle Writes sonst committed würden.
  - **Idempotent:** drei stabile Action-Strings: `substituted`, `already_substituted`, `already_on_instance` (Substitute war bereits Co-Betreuer, existing Row bleibt unverändert).
  - **Soft-Warnings:** `substitute_time_conflict` bei Zeit-Überschneidung mit anderen Einsätzen des Substitutes am selben Tag. Blockiert nicht.
  - **Coverage-Note:** SonarQube meldete 79.3% auf New Code (knapp unter 80%-Gate). Kein Merge-Block, aber werterhöhende Tests (besonders für den Active-Path in substitute.go) wären ein guter Nachzieher.
- [x] **WP-B13**: Exception conflict warnings (arrival ↔ timetable) → GitHub #1304
  - **Endpoint:** `GET /api/timetable/exception-conflicts?date=...&date_to=...` (max 14 Tage, heute plus Zukunft, SchedulesRead).
  - **Warning-Typen:** `cancelled_instance_with_scheduled_arrivals` (Admin sieht welche Kinder zu einer gecancelten Aktivitaet kommen), `modified_instance_time_mismatch` (Aktivitaet verschoben, Schueler wuerde den Anfang verpassen).
  - **Modified-Semantik:** Warning nur wenn `exception.start_time` != NULL und die resolved Arrival-Zeit des Schuelers VOR der neuen Activity-start_time liegt. Room-only-Aenderungen werden nicht geflaggt.
  - **Daten-Inkonsistenz:** Wenn arrival_exception mit `expected_arrival=nil` (Kind kommt nicht) existiert, wird der Schueler fuer beide Warning-Typen skipped. Admin bekommt keine False-Positives.
  - **Query-Strategie:** Fixed anzahl Bulk-Queries (FindByDateRange + Instance-Range + FindExpectedByInstanceIDs + arrival batches + template-time batch). Kein N+1.
  - **Bundled Refactor:** `inclusiveDayCount` nach `internal/timezone/` extrahiert, wird jetzt von /week (B11), /gaps (B12) und /exception-conflicts (B13) geteilt.
- [x] **WP-B14** — GDPR cleanup job for timetable data → GitHub #1298

#### B4 — Backend roadmap (after MVP)

- [ ] **WP-B15** — Staff absence entity + substitution plan backend
- [ ] **WP-B16** — Holiday-specific templates + enrollments backend

### Frontend Track (starts only after backend MVP is stable)

#### F1 — Arrival admin

- [x] **WP-F1** — Arrival schedule editor (class-based bulk + individual override) → GitHub #1306
  - **Per-Student-Manager:** `ArrivalScheduleManager` + `ArrivalDayEditModal` + `ArrivalScheduleFormModal` (Mo–Fr-Wochengrid mit Wochennavigation, Day-Edit fuer Exception/Notes, Bulk-Wochen-Edit). Eingebettet im Studenten-Detail-Tab "Betreuungszeiten" zusammen mit `PickupScheduleManager`. Read-only-Mode wenn `has_full_access=false`.
  - **Class Bulk:** `ClassBulkArrivalModal` ueber Kebab-Menue auf Klassen-Gruppen-Rows (statt prominentem Button). Holt echte Per-Student-Schedules beim Open, um die Kollisionszahl korrekt zu berechnen — die alte v1-Annahme "alle Schueler haben bereits Arrival" produzierte falsche Warnungen.
  - **Proxy + Client:** Next.js-Proxy-Routen unter `/api/students/[id]/arrival-{schedules,exceptions,notes}` und `/api/students/arrival-{schedules,times}/bulk`. API-Client + Helpers in `lib/student-arrival-api.ts` und `lib/arrival-schedule-helpers.ts` mit deutschen Fehler-Mappings.
  - **Begleitende B1/B2-Polish (kein neues WP):** `GET /students?include_arrival_times=true` reichert Listen mit heutiger Arrival-Time + Exception-Flag + zusammengefuehrten Notes an, sodass OGS-Groups, Active-Supervisions, Search und Detail keinen parallelen Bulk-Fetch mehr brauchen. Bulk-Endpoint-DTO `arrival_time` → `expected_arrival` korrigiert (Handler/Service/Test). Neue SSE-Events `student_updated` und `arrival_schedule_changed` aus dem Students-Resource. `date_params.go`-Helper formatiert Berlin-Kalendertage und paart mit `?::date` in Pickup/Arrival-Queries (loest TZ-Drift unter DB-Session-Timezone).
  - **LocationBadge:** Neuer State "Kommt heute nicht" ersetzt das Home-Badge wenn `arrival_exception` mit `expected_arrival=NULL` fuer heute existiert. Sortierung nach Ankunftszeit in OGS-Groups + Student-Search ergaenzt.
  - **Out-of-scope-Bundled (nicht im F-Plan, aber im selben PR):** Master-Detail-Refactor der Database-Studenten-Page (`MasterDetailLayout`, `GroupedList`, `GroupHeader`, `DetailPanel`, `EmptyDetailState`) als wiederverwendbare Primitiven fuer kommende Database-Refactors (Groups, Rooms). Ersetzt das alte Modal-Editor-Muster durch tabbed Detail-Panel mit URL-Sync (`?student=ID&groupBy=class|group|none`). Tab-Strip horizontal scrollbar fuer Viewports < 640px. Redundantes "Klasse "-Prefix in Listen entfernt (Backend liefert bereits "Klasse 1a").

#### F2 — Admin planner (current PR)

- [x] **WP-F2** — Admin weekly planner UI (current branch)
  - **Shipped scope:** month view, week view, calendar-period CRUD, recurring template CRUD, manual materialization, one-off instance CRUD, instance complete/cancel, conflict warnings, staff gaps, substitute action, re-plan-week.
  - **Scope decision:** The admin planner is a planning and correction surface. Starting planned instances from the planner is intentionally deferred; the operational start flow belongs to `/active-supervisions` (F3-F4), where staff already work during live operations.
  - **Deferred follow-up:** Recurring template archive remains a later planner-lifecycle polish item (see F12), not a blocker for F2.
  - **Non-goal for WP-F2:** staff-facing operational flow. That belongs to F3-F6 plus F8 below.

#### F3 — Staff operations from the plan

**Implementation cut:** F3 and F4 should ship together in one PR. F3 alone only shows planned work; F4 turns that card into the operational action that starts the planned instance and prevents duplicate unlinked live groups.

- [x] **WP-F3** — Planned items in `/active-supervisions` ("Jetzt geplant") → current PR
  - Show planned instances for today in an operational window: `now - 15min` through `now + 15min`, plus overdue planned instances.
  - Cards show title, time, room, assigned staff, expected child count, conflict/gap status and a clear **Jetzt starten** action.
  - Prefer instances assigned to the current staff member; admins may see all when supervision overview is enabled.
- [x] **WP-F4** — Start planned instance from `/active-supervisions` → current PR
  - **Jetzt starten** calls `POST /api/timetable/instances/{id}/start`.
  - The resulting live `active.group` appears in the current supervision UI without requiring navigation back to the admin timetable.
  - Prevent confusing duplicate starts: when a matching planned instance exists, the staff flow should start that instance instead of creating an unlinked live group.
  - **Bundled operational polish:** active-supervision dashboard now bridges timetable instances into the live roster flow, supports completion from the live view, avoids loading-state flashes, and handles timetable SSE invalidation for started/completed/cancelled/overdue events.
- [x] **WP-F5** — Instance detail/check-in list in active supervision
  - After start, show expected children from `instance_students` alongside live visits.
  - Support quick status changes: expected, present, absent, substatus/note where needed.
  - Surface unplanned children separately instead of hiding them.
- [x] **WP-F6** — Spontaneous activity creation (staff-facing)
  - Staff can create and immediately start an ad-hoc activity from `/active-supervisions`.
  - The flow creates a spontaneous timetable instance when useful for reporting, then bridges it to `active.groups`.
  - **Shipped scope:** compact mobile-first start banner with plus action, existing-activity search/chips, custom title fallback, room selection, optional additional staff, immediate create+start, empty roster first, then F5 search/check-in flow for children.
  - **SSE behavior:** start/complete still flows through the timetable instance lifecycle, so `instance_started`/`instance_completed` plus active-supervision invalidation keep other assigned staff in sync.

#### F4 — Rollover + live updates

- [ ] **WP-F7** — Semester rollover UI + enrollment validity management
- [x] **WP-F8** — Timetable/active-supervision SSE wiring (E19 level 2)
  - Backend emission is already shipped via WP-B9: `EventInstanceOverdue` from the scheduler plus `EventInstanceStarted/Completed/Cancelled` from `instance_service.go`.
  - Frontend work: extend `frontend/src/lib/sse-types.ts`, update global SSE handlers, invalidate timetable and active-supervision SWR keys, and bind overdue/started state to the operational cards.
  - Acceptance: starting a planned instance in the timetable updates `/active-supervisions` without browser refresh; starting/completing in active supervision updates the timetable card.

#### F5 — Frontend roadmap

- [ ] **WP-F9** — Calendar period admin UI beyond the planner header
- [ ] **WP-F10** — Student day view in frontend (currently API-only)
- [ ] **WP-F11** — Automatic auto-start (E19 level 3)
- [ ] **WP-F12** — Recurring template archive action
  - Add archive action to the series UI with confirmation, success/error toast, and SWR refresh for the affected `timetable-templates-*` cache.
  - Backend and proxy support already exist via `DELETE /api/timetable/templates/{id}`; this is frontend wiring only.
- [ ] **WP-F13** — Device-aware planned instance start flow
  - PyrePortal and future staff devices should ask the timetable before starting an activity session: "Is there a planned instance for this staff member, room, and current time window?"
  - If exactly one planned instance matches, the device starts that instance via the existing lifecycle path instead of creating an unlinked `active.group`.
  - If multiple candidates match, the device shows a short chooser with title, time, room, and expected count.
  - If no candidate matches, the device falls back to the spontaneous flow only when the current time block allows ad-hoc activities.
  - Keep the existing IoT auth and response contract stable for old PyrePortal builds; add new optional fields/endpoints rather than changing hardcoded error messages.
- [ ] **WP-F14** — Spontaneous activity windows and rules
  - Add a tenant-configurable rule model for spontaneous activity windows: allowed, suggested, or blocked by weekday/time range and optionally room/activity type.
  - Use the rules in `/active-supervisions`, PyrePortal, and future mobile flows so spontaneous sessions are not just "always possible" in the UI.
  - When staff creates a spontaneous session inside an allowed block, create a spontaneous timetable instance first, then bridge it to `active.groups` for live visits and reporting.
  - Outside allowed blocks, show the matching planned instance or a clear reason why a spontaneous session cannot be started.
- [ ] **WP-F15** — Staff calendar views ("my day" / "my week")
  - Build staff-facing calendar views from timetable instances, arrival context, active sessions, and pickup context.
  - Show "now", "next", and "this week" states: planned, active, completed, cancelled, overdue, and spontaneous.
  - Let staff jump from a calendar item into the active supervision/check-in view when an instance is running or ready to start.
  - Keep admins able to inspect "who is doing what right now" without turning the admin planner into the operational UI.
- [ ] **WP-F16** — Timetable + time tracking integration
  - Define how started/completed timetable instances inform staff time tracking: supervision time, substitutions, late starts, early endings, and spontaneous coverage.
  - Do not make timetable the canonical time-clock system. It supplies context and reporting links; the time tracking domain remains authoritative for working time.
  - Add reporting hooks so plan-vs-reality can show planned staff vs. actual staff and planned time vs. actual active duration.
- [ ] **WP-F17** — Mobile self-check-in scope decision
  - Decide whether "self-check-in" means staff mobile check-in, child device self-service, parent app, or multiple flows.
  - Define auth, permissions, room/instance selection, abuse prevention, and offline/error behavior before building UI.
  - If the first scope is staff mobile check-in, reuse the F5 instance detail/check-in list and attendance sync model.

### Backend dependency map

| Work Package | Blocks |
|---|---|
| WP-B5 (instance tables) | B8, B9, B10, B11, B12, B13 |
| WP-B6 (nullable group_id) | B9 |
| WP-B7 (settings) | — (parallelizable, no blockers) |
| WP-B10 (attendance sync) | B13 |
| WP-B14 (cleanup) | — |

### Recommended next

**Backend-MVP ist geschlossen.** B1 bis B14 sind shipped. B15 (Staff-Absence-Entity) und B16 (Ferienbetreuung) bleiben als post-MVP Roadmap-Items, nicht blockierend.

**F-Track ist gestartet.** WP-F1 (Arrival Schedule Editor) ist via #1306 shipped. Damit ist der erste sichtbare Admin-Mehrwert live und das Master-Detail-Layout-Pattern als wiederverwendbare Primitive etabliert.

**Aktueller Stand:** WP-F2 ist abgeschlossen. Der Admin-Stundenplan ist jetzt als Planungswerkzeug nutzbar: Buero/Admin kann Perioden, Serien, konkrete Termine, Personal-Luecken, Ersatz und Neuplanung pflegen.

**Naechster Produktwert:** WP-F3-F6 plus F8 sind im operativen `/active-supervisions`-Schnitt geschlossen: geplante Aktivitaeten erscheinen, koennen gestartet werden, laufende Instanzen zeigen erwartete/ungeplante Kinder, spontane Aktivitaeten koennen direkt gestartet werden, und SSE haelt die Ansichten ohne manuellen Refresh frisch.

**Danach muss Timetable in die echten Startpunkte rein:** F13/F14 verbinden geplante Instanzen und spontane Aktivitaeten mit PyrePortal/Geraeten. Das Ziel ist: Ein Geraet startet nicht blind eine neue Live-Gruppe, sondern erkennt zuerst den passenden geplanten Termin. Spontane Aktivitaeten bleiben moeglich, aber nur in expliziten Zeitfenstern und mit Timetable-Instance fuer Reporting.

**Staff-Kalender und Zeiterfassung folgen darauf:** F15/F16 machen aus den Instanzen eine persoenliche Tages-/Wochenansicht und verbinden geplante/gelaufene Zeiten mit der Zeiterfassung, ohne die Zeiterfassung als eigene Domain zu ersetzen.

**F8 ist ein Quick-Win nach F3:** Die Backend-SSE-Emission fuer `instance_started/completed/cancelled/overdue` ist via B9 bereits live. Sobald F3 die operational cards hat, ist F8 nur noch Frontend-Wiring (sse-types + globaler SSE-Handler + SWR-Invalidierung + Badge-Bindung).

**F11 (Automatic Auto-Start)** kommt zuletzt, weil es die Scheduler-Erweiterung um automatisches `active.group`-Anlegen voraussetzt (E19 Level 3) und auf der UI-Indikator-Schicht aus F3 aufbaut.

**Alternative Backend-Arbeit** falls jemand auf dem Backend bleiben will: B15 ist die logische Fortsetzung von B12 (Staff-Absence als persistente Entity statt Ad-hoc-Flag). B16 kann parallel zu F-Track laufen, wenn die Schule Ferienbetreuung konkret anfragt.

---

## 8. Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| **`active.groups.group_id` nullable breaks existing code** | All queries/services using `GroupID` must become NULL-safe. Existing NFC flow always sets GroupID — no behavioral change for current features. |
| **Template changes after materialization** | "Re-plan week" overwrites only `planned` instances. Active/completed instances are protected. UI shows warning before re-materialization. |
| **Attendance dual source of truth** (`instance_students` vs. `active.visits`) | Application code syncs at check-in/check-out time (E4). `active.visits` remains canonical for real-time. `instance_students` is the plan-vs-reality view. |
| **A/B week boundary bugs** | Day-based difference calculation (not ISO weeks) with double-modulo for negative differences. Tested explicitly in materialization tests. |
| **Enrollment rollover data loss** | `valid_from`/`valid_until` preserves all history. Old enrollments are never deleted — they expire. Partial UNIQUE index ensures only active enrollments are deduplicated. |
| **Period overlap during materialization** | Each template has an optional `calendar_period_id`. Templates without a period match all active periods. Templates with a period only materialize within that period's date range. |
| **Care offering → enrollment mapping unclear** | Flagged as open question (§4.8). No blocking dependency — systems can proceed independently and the mapping can be defined later. |
| **PyrePortal impact** | Existing IoT endpoints and error messages stay stable for old kiosk builds. New planned-instance behavior must be additive: device can discover/start a planned instance, but old NFC payloads still work. |
| **Device creates duplicate live group instead of starting plan** | F13 adds a pre-start matching step. If a planned instance matches staff/room/time, the device starts that instance. Only unmatched allowed windows create spontaneous instances. |
| **Spontaneous activity rules become hidden business logic** | F14 makes time-block rules explicit in tenant settings/UI and reuses the same rule check in active supervision, PyrePortal, and mobile flows. |
| **Time tracking source-of-truth drift** | F16 keeps time tracking authoritative for working time. Timetable links planned/actual supervision context but does not overwrite clock records silently. |
| **GDPR: student names in timetable data** | `instance_students` stores only `student_id`. GDPR cleanup removes completed instances after retention period. slog uses student IDs only at info level. |
| **Large-scale materialization (many templates × many students)** | Materialization runs as background scheduler job, not in request path. Idempotent via partial UNIQUE index — re-running creates only missing instances. |
| **Multi-room complexity in UI** | ~90% of instances use single room (override is NULL). UI defaults to simple view, shows room assignments only when overrides exist. |

---

## 9. Out of Scope (v1)

- iServ / WebUntis integration (data import from school timetable systems)
- Automatic shift plan generation (data foundation is laid, generation deferred)
- Parent app integration (separate epic)
- Push notifications ("your instance starts in 10 minutes")
- Drag-and-drop in weekly planner (click-to-edit for v1)
- Staff absence entity + full substitution plan UI (gap detection query for MVP)
- Holiday care admin UI (schema is prepared, E22)
- RRULE-based recurrence (A/B weeks via simple pattern field is sufficient)
- Capacity checks at materialization time (room max capacity vs. enrolled students)
- Retrospective materialization (backfilling past weeks)

---

## 10. Sharp Edges for Implementation

- **`active.groups.GroupID` nullable** — every consumer of `GroupID` must handle `nil`. Search for `.GroupID` across the codebase before merging.
- **Partial UNIQUE indexes** — `WHERE activity_group_id IS NOT NULL` on instances, `WHERE valid_until IS NULL` on enrollments/supervisors. BUN ORM must use `bun.Safe()` for raw WHERE clauses in index definitions.
- **Attendance sync ordering** — check-in handler must create `active.visit` FIRST (existing flow), then update `instance_students`. If instance lookup fails, the visit still works (graceful degradation).
- **Hermetic tests** — all new tests must use `testpkg.CreateTestStudent`, `testpkg.CreateTestStaff`, etc. No hardcoded IDs. Run `TestHermeticTestPatterns` before pushing.
- **Settings consumers** — follow the `HasTenantOverride` → `Resolve*` → env var → fallback pattern from `.claude/rules/settings-system.md`.
- **Migration version uniqueness** — check `MigrationRegistry` for collisions. Multiple migrations in this system will be registered.
- **German UI** — all setting labels/descriptions in German. Instance titles from templates are user-entered (already German).
- **`enrollment_date` → `valid_from` rename** — existing code using `enrollment_date` must be updated. Search for the field name across backend.
- **Calendar period auto-creation** — the default period is created lazily on first timetable access. Tests must account for this (or pre-create a period in fixtures).
- **No production requests from Claude** — local dev / staging only.
- **Query-Budget-Semantik** — PR-Beschreibungen wie "≤ 2 queries", "≤ 22 queries" beziehen sich auf Handler-Queries, nicht End-to-End. Die `TenantTxMiddleware` addiert 4 bis 5 Queries pro Request (`SET LOCAL ROLE`, `SELECT set_config(...)`, BEGIN/COMMIT). Beim Messen in Isolation muss dieser Overhead abgezogen werden; beim Setzen von SLO-Targets ist die End-to-End-Zahl relevant. (E2E-C2.)

---

## 11. Industry Validation

Design validated against 6 open-source SIS (Gibbon, OpenSIS, Frappe Education, OpenEduCat, Fedena, SchoolTool) and 5+ commercial products (Amilia, Famly, OGS-Connect, Ganztagsplaner, Aurora Ganztag):

| Design aspect | Industry standard | Our approach |
|---------------|-------------------|-------------|
| Activity-centric model | Universal (all 6 OSS) | Correct — students connect via enrollment/junction tables |
| Template → Instance | Amilia, Google Calendar | Correct — bounded materialization for attendance-tracking |
| Template + Exception | iCal RFC 5545 (EXDATE/RECURRENCE-ID) | Correct — `activity_exceptions` mirror this pattern |
| Attendance stored per instance | OpenEduCat, Amilia | Correct — deliberate denormalization for performance (E4) |
| Spontaneous activities | Amilia (drop-in with `DropInOccurrenceId`) | Correct — `is_spontaneous` flag preserved (E1/Iter 1) |
| Conflict detection | Aurora Ganztag | Correct — soft warnings, no hard blocks (E7) |
| Live-layer bridge | No comparable system | Unique to Project Phoenix — `active.groups` integration |
| Multi-room override | No system this clean | Better than industry — avoids junction table overhead (E3) |
| Care contract as implicit | OGS-Connect, Ganztagsplaner | Correct — arrival + pickup = care window (E14) |

Reference repos: **Gibbon** (`github.com/GibbonEdu/core`) for schema patterns, **Amilia** REST API docs for the closest Template→Instance design, **teambition/rrule-go** for future RRULE support if needed.
