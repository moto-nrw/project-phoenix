# Parent-Facing Student Enrollment System — Implementation Plan

**Status:** Draft for team discussion (revision 2)
**Target branch:** `development`
**Author:** Drafted with Claude Code, 2026-04-13 (revised 2026-04-15 based on team feedback)

## Changelog

- **2026-04-15 (rev 2):** Added structured Betreuungsangebote (care offerings), email outbox/delivery-failure visibility, waitlist status + parent status page, per-child approval decisions, refined withdrawal rules. Updated domain model, API, settings, phasing, and risks sections accordingly.
- **2026-04-15 (rev 2.1):** Added per-offering application windows (`application_window_start`/`_end`) and service date fields on `care_offerings` so parents can't apply for offerings whose intake period has closed (e.g. holiday care after holidays begin).
- **2026-04-16 (rev 2.2):** Aligned with Yannick's Timetable RFC (`docs/timetable-system-plan.md`). Dropped `platform.school_years` in favor of `schedule.calendar_periods` (filter by `period_type='school_year'`). Dropped `users.student_year_assignments` in favor of the validity-scoped pattern (`valid_from`/`valid_until`). Approved care offering selections now create `activities.student_enrollments` rows instead of a new `users.student_care_enrollments` table. Added §11 Cross-dependencies.
- **2026-04-16 (rev 2.3):** Resolved team questions. Q11: outbox is **shared platform-level** (`platform.email_outbox`), not enrollment-scoped. Q12: status token TTL default 1 year, **operator-editable only** per tenant (new `platform:config:update` write permission for this setting). Q13: Yannick will ship `schedule.calendar_periods` — emergency fallback plan removed from Phase 1 PR 1 / PR 6.

---

## 1. Goal

Enable parents to submit a sign-up request for a child (or multiple children) for a future or current OGS school year via a public form on the tenant subdomain. School admins review requests, approve or reject them, and on approval the system creates the student records and invites the parent to create an account — all without manual data entry from the admin.

## 2. Requirements Summary

- Public submission form per tenant (`/{tenant}/enroll`), no login required
- If submitter is logged in as a guardian, prefill their data (optional convenience)
- One submission can include multiple children
- **Structured Betreuungsangebote (care offerings)** — modules, days, holiday care, lunch — selected by parents as first-class typed data, not freeform custom fields
- **Per-child admin decisions** — admin can approve, waitlist, or reject each child independently; the parent is still treated as one unit (one account, one status link)
- Admin review UI: see all requests, edit fields before approval, approve/waitlist/reject with per-child reason notes
- On approval: create student records (initially in `pending` status), link guardian, send email invitation to create account
- Guardians without an account get an invitation email with a token; existing accounts get a "new child linked" confirmation
- **Waitlist support** with reason captured and visible to the parent
- **Parent status page** accessible via the same tokenized link in the confirmation email — stays accessible after decisions to let the parent track status per child
- **Email outbox with retry + admin visibility** — approval writes email jobs atomically with DB changes; delivery failures surface in admin UI with a resend action
- Admins can edit the enrollment form per tenant (add/remove/reorder custom fields) — core fields are fixed
- Optional behaviors (which fields to collect, activation timing, duplicate handling, waitlist on/off) go into the settings system
- XLSX export of requests (Phase 2)
- Ability for parents to view/edit/withdraw their submission via a tokenized link in the confirmation email (withdrawal limited to non-terminal statuses; approved children must go through admin)
- File attachments deferred to Phase 2
- Re-enrollment of existing students deferred (not in scope for v1)

## 3. Key Decisions (confirmed)

| Topic | Decision |
|-------|----------|
| **Parent role** | No new role. Reuse existing `guardian` base role, `guardian_profiles`, `guardian_invitations`, and `students_guardians`. The orphaned `auth.accounts_parents` table is NOT wired in — leave it alone or remove in a separate cleanup task. |
| **Student creation timing** | On approval, create `users.students` rows with `status='pending'`. A daily scheduler job flips them to `active` when `enrolled_from <= today`. Schools can choose `immediate` or `scheduled` activation via settings. |
| **School year model** | **Reuse `schedule.calendar_periods`** from Yannick's Timetable RFC (filter by `period_type='school_year'`). Do NOT introduce a separate `platform.school_years` table. Year-scoped class progression uses the validity pattern (`valid_from`/`valid_until`) on the relevant student/enrollment rows, matching the Timetable RFC's E17 decision. |
| **Grade level vs class** | Collect **grade level** (integer 1–13) on the form, not specific class ("1a" vs "1b"). Whether the form asks for it is toggled by setting `enrollment.collect_grade_level`. The concrete class (e.g., "1a") is assigned by the admin post-approval. |
| **Year picker** | Parent chooses the target school year from available open years (tenant has ~4 open years max). No setting needed. |
| **Edit/withdraw** | Tokenized link in the confirmation email lets the parent view/edit/withdraw their pending submission. |
| **Form schema editor** | Simple table (add/edit/delete/reorder fields), reusing the settings system's field-type vocabulary. No drag-and-drop layout builder. Versioned — submitted requests pin to their schema version. |
| **Attachments** | Deferred to Phase 2. |
| **Re-enrollment** | Deferred — this form is first-time signup only for v1. |
| **Betreuungsangebote (care offerings)** | First-class structured model. Admin defines a catalog per school year (`enrollment.care_offerings`); parent selects; selection stored on `request_child_offerings`. Not treated as custom form fields — structure is required for capacity, reporting, and export. |
| **Approval granularity** | Per-child. Admin decides approve/waitlist/reject for each child independently. The parent is still one unit: one guardian profile, one account (if created), one status token, one status page. |
| **Waitlist** | Supported as first-class status alongside approved/rejected. Setting `enrollment.waitlist_enabled` toggles visibility in admin UI. Admin can promote waitlisted children to approved (same code path as initial approval). |
| **Parent status visibility** | Tokenized status page accessible from the confirmation email stays live throughout the lifecycle. Parent sees per-child status + optional reason. Same token also powers edit/withdraw (gated by rules in §6.3). |
| **Email reliability** | Outbox pattern (`platform.email_outbox`) — all approval-related emails written atomically with DB changes, dispatched by a worker with exponential backoff retry. Admin UI shows per-request delivery status with resend button. No more silent send failures. |
| **Decision notification style** | Configurable per tenant via `enrollment.notify_per_decision` — default `digest` (one email summarizing all children's decisions after admin saves), alternative `immediate` (one email per child decision). |
| **Care-offering ↔ activity-group mapping** | Each `enrollment.care_offerings` row optionally references an `activities.groups` row (care-type). Approving a child's care selection creates an `activities.student_enrollments` row linking student to that activity group, with `valid_from`/`valid_until` derived from the selected `calendar_period`. The offering row stays in the `enrollment` schema as the intake-side catalog; the activity group stays in the `activities` schema as the schedule-side entity. Clean separation. |

## 4. Open Questions for Team Review

1. **GDPR / consent checkboxes** — what fixed consent items are legally required on a first-time signup form? Proposed defaults: (a) AGB/terms acceptance, (b) data processing, (c) contact by email. Are there more (photo, emergency contact sharing)?
2. **Rejected-request retention** — how long should rejected submissions stay in the DB before automatic deletion? Proposed default: 90 days, settings-configurable.
3. **Calendar period management ownership** — who creates `calendar_periods` entries (with `period_type='school_year'`) in the admin UI? Operator-level (moto) or tenant-level (school admin)? Since the Timetable RFC auto-generates a default school-year period per tenant (E15), the practical answer may be "automatically by the system; admins only add extra periods (holiday, custom)." Confirm with Yannick.
4. **Existing `school_class` string field** — Yannick's RFC (E11) keeps `students.school_class` as the canonical source for class identity and uses it as the filter for arrival-schedule bulk endpoints. Our plan no longer needs a year-scoped assignment table, so we can leave `school_class` untouched. Grade-level progression across years becomes the Timetable RFC's concern.
5. **Spam protection** — is Turnstile/hCaptcha acceptable, or do we need a different provider? Gated behind `enrollment.require_captcha` setting.
6. **Email sender identity** — emails sent from a tenant-specific from-address, or a global `enrollment@moto-app.de`? DNS/DKIM implications.
7. **Grade levels beyond OGS** — OGS is typically grades 1–4, but the migration allows 1–13. Keep permissive or restrict?
8. **Care offerings — downstream representation** — **RESOLVED (rev 2.2):** Approved care selections create `activities.student_enrollments` rows scoped by `valid_from`/`valid_until` from the calendar period, following the Timetable RFC's E17 pattern. Offerings without a linked `activity_group_id` stay as catalog-only records.
9. **Care offering capacity overflow behavior** — when a parent submits but the chosen offering is full, should we: (a) reject the submission with an error, (b) auto-place the child on waitlist and let the admin decide, or (c) allow submission and surface the overflow in admin UI? (b) feels most parent-friendly.
10. **Digest vs. immediate notification default** — default proposed as `digest` (one email after admin saves all decisions). Is that right, or do schools expect immediate per-child emails?
11. **Outbox scope** — **RESOLVED (rev 2.3):** build `platform.email_outbox` (shared across features), not an enrollment-scoped table.
12. **Status token lifetime** — **RESOLVED (rev 2.3):** default 1 year. Operator-only editable per tenant (new permission pattern — see §11 coordination notes).
13. **Coordination with Timetable RFC delivery** — **RESOLVED (rev 2.3):** Yannick will ship `schedule.calendar_periods`. Enrollment Phase 1 no longer needs its own calendar_periods PR; we consume his.
14. **Auto-generated default school-year period** — still open: does Yannick's delivery auto-create a `school_year` period per tenant? If not, the enrollment admin UI (PR 6 or earlier) needs a setup step. Ask him.
15. **Care offering ↔ activity group wiring UX** — since `care_offerings.activity_group_id` is nullable, admins can create offerings before the matching `activities.groups` row exists. When a care offering has no linked activity group and a child is approved, what should happen? Proposed: log a warning, still approve the enrollment request, but skip the `student_enrollments` insert — admin can wire it later via an "attach activity group" action.

---

## 5. Architecture

### 5.1 Domain model additions

**New schema: `enrollment`**

```
enrollment.form_schemas
  id, tenant_id, version, fields JSONB, is_active, created_by, created_at

enrollment.care_offerings
  id, tenant_id,
  calendar_period_id bigint NOT NULL FK → schedule.calendar_periods,  -- replaces school_year_id; period_type is typically 'school_year' or 'holiday'
  activity_group_id bigint (nullable FK → activities.groups),          -- care-type group this offering maps to (may be NULL for pure catalog entries until the admin wires a group)
  name, description,
  days_of_week_mode ('fixed'|'parent_choice'),   -- fixed = Mon–Fri always, choice = parent picks
  available_days JSONB,                            -- e.g. ["mon","tue","wed","thu","fri"]
  includes_holiday_care boolean,
  includes_lunch boolean,
  capacity int (nullable),                         -- null = unlimited
  price_cents int (nullable),                      -- informational only; not billing-grade
  application_window_start timestamptz (nullable), -- parents can apply from this moment
  application_window_end timestamptz (nullable),   -- parents can no longer apply after this
  service_start_date date (nullable),              -- informational: when the offering actually begins (e.g. first day of holiday care)
  service_end_date date (nullable),                -- informational: when it ends
  is_active boolean,
  sort_order, created_at, updated_at
  CHECK (application_window_end IS NULL OR application_window_start IS NULL
         OR application_window_end > application_window_start)
  CHECK (service_end_date IS NULL OR service_start_date IS NULL
         OR service_end_date >= service_start_date)

enrollment.requests
  id, tenant_id, schema_id (version snapshot),
  calendar_period_id bigint NOT NULL FK → schedule.calendar_periods,  -- which school-year period the request targets
  -- request-level status is DERIVED from children: submitted | under_review | partial | finalized | withdrawn
  guardian_first_name, guardian_last_name, guardian_email, guardian_phone,
  guardian_account_id (nullable — set if submitter was logged in),
  consent_flags JSONB,
  custom_data JSONB,
  status_token (unique, used for edit + status page),
  status_token_expires (nullable — extended on state transitions so the link keeps working)
  submitted_at, updated_at, withdrawn_at

enrollment.request_children
  id, request_id, first_name, last_name, date_of_birth,
  target_grade_level (nullable, 1–13),
  custom_data JSONB,
  status ('submitted'|'under_review'|'approved'|'waitlisted'|'rejected'|'withdrawn'),
  status_reason TEXT (nullable — admin's explanation, shown to parent if setting enabled),
  activation_mode ('immediate'|'scheduled'),
  activate_on DATE (nullable),
  reviewed_at, reviewed_by,
  created_student_id (nullable — filled on approval),
  sort_order

enrollment.request_child_offerings
  id, request_child_id, care_offering_id,
  selected_days JSONB (nullable — if offering uses parent_choice mode),
  notes TEXT (nullable)
  UNIQUE(request_child_id, care_offering_id)

platform.email_outbox                          -- SHARED ACROSS FEATURES, not enrollment-only
  id, tenant_id, kind (text, extensible — initial kinds listed below),
  related_entity_type (text, nullable — e.g. 'enrollment_request', 'invitation'),
  related_entity_id (bigint, nullable — polymorphic reference, no FK constraint),
  payload JSONB (rendered template context),
  status ('pending'|'sending'|'sent'|'failed'),
  attempts int default 0,
  last_error TEXT (nullable),
  next_retry_at timestamptz,
  created_at, sent_at (nullable)
  Indexes: (status, next_retry_at) for the worker pickup query
```

**Why `email_outbox` over a pure async dispatcher:** the approval flow writes student + guardian + outbox rows in ONE transaction. The worker reads outbox rows independently. A dispatcher crash or SMTP outage no longer leaves the database in a state where records exist but the invitation was never sent. Admin UI reads from `email_outbox` to show delivery status; resend re-enqueues a new outbox row.

**Why `platform.email_outbox` (shared) instead of `enrollment.email_outbox`:** per-feature outboxes would duplicate worker logic, retry policy, monitoring, and admin UI. A single platform-level table keyed by `kind` + `related_entity_type` is reused for guardian invitations (enrollment), staff invitations, audit email notifications, and any future feature. The enrollment feature only adds new `kind` values and a small wrapper on the admin UI to filter by request-related rows. Initial `kind` values registered by enrollment: `guardian_invitation`, `enrollment_submitted`, `enrollment_admin_notification`, `enrollment_decision_digest`, `enrollment_approved`, `enrollment_waitlisted`, `enrollment_rejected`.

**No `platform.school_years` / `users.student_year_assignments` tables.** Earlier revisions of this plan proposed new tables for year tracking and year-scoped class assignment. The Timetable RFC (`docs/timetable-system-plan.md`) introduces `schedule.calendar_periods` with `period_type IN ('school_year'|'semester'|'holiday'|'custom')` — a superset of what we need — and establishes the `valid_from`/`valid_until` pattern on existing rows (`activities.student_enrollments`, `activities.supervisors`) for year-scoped progression. We defer to that model:

- **Year identity** → `schedule.calendar_periods` rows with `period_type='school_year'`
- **Year-scoped care enrollment** → `activities.student_enrollments` rows with `valid_from`/`valid_until` scoped by the selected calendar period
- **Class progression across years** → the Timetable RFC's enrollment-validity pattern (bulk rollover endpoint from the RFC's §5.4); we don't model this in the enrollment plan
- **Grade level at time of enrollment** → still collected on `request_children.target_grade_level`, but recorded on the created student via existing student fields (not a new assignment table)

This removes duplicate concepts and avoids competing schemas. If the Timetable RFC's `calendar_periods` table has not landed by the time this plan starts, Phase 1 PR 1 becomes a thin helper migration that introduces just `schedule.calendar_periods` (subset of Yannick's spec — `period_type`, `start_date`, `end_date`, `is_active`) so this plan can proceed without blocking on the full timetable system.

**New in `platform` schema**

```
platform.school_years
  id, tenant_id, label ("2026/2027"), start_date, end_date, is_current,
  UNIQUE(tenant_id, label), UNIQUE(tenant_id) WHERE is_current
```

**Additions to existing tables**

```
users.students
  + status text ('pending'|'active'|'inactive'|'alumnus'), default 'active'
  + enrolled_from DATE, enrolled_until DATE
  Indexes on (tenant_id, status) and (tenant_id, enrolled_from) WHERE status='pending'

users.student_year_assignments  (new table)
  id, tenant_id, student_id, school_year_id,
  grade_level (int 1–13), class_name (text, nullable),
  group_id (FK education.groups, nullable),
  UNIQUE(student_id, school_year_id)
```

All new tables get RLS policies matching the existing tenant-isolation pattern.

### 5.2 Service layer

- **`services/enrollment/form_schema_service.go`** — get active schema, create versions, validate submissions
- **`services/enrollment/care_offering_service.go`** — admin CRUD over the care offerings catalog, capacity checks, per-year cloning for next year's setup, **application-window enforcement** (offerings outside their `application_window_start..application_window_end` are hidden from the public form and rejected server-side even if a stale client submits them)
- **`services/enrollment/request_service.go`** — public submit/withdraw/edit (token-auth), admin list/get/patch; derives request-level status from child statuses; enforces withdrawal rules (only non-terminal children)
- **`services/enrollment/decision_service.go`** — per-child approve/waitlist/reject. On `approved`: creates student (status=pending), guardian_profile, students_guardians, and — for each selected care offering with a non-null `activity_group_id` — an `activities.student_enrollments` row with `valid_from`/`valid_until` derived from the calendar period. Writes outbox rows atomically. Idempotent per child — promoting a waitlisted child to approved works via the same entry point. Depends on `schedule.calendar_periods` (from Timetable RFC) and `activities.student_enrollments` (existing).
- **`services/enrollment/outbox_worker.go`** — scheduler-driven worker. Picks up `pending` outbox rows where `next_retry_at <= now()`, renders template, dispatches via existing mailer. Exponential backoff on failure (1min, 5min, 30min, 2h, 12h, then `failed` at 6 attempts). Marks `sent` on success.
- **`services/auth/guardian_invitation_service.go`** — complete the half-built service that mirrors the staff `invitation_service`. Issue token, render email via outbox (not direct dispatch), accept-and-create-account, cleanup expired, resend.
- **`services/scheduler/activate_students.go`** — daily job: `UPDATE users.students SET status='active' WHERE status='pending' AND enrolled_from <= CURRENT_DATE`, and the inverse for `enrolled_until`

### 5.3 API surface

**Public (no auth, token- or tenant-slug-gated):**
- `POST /api/enrollment/{tenantSlug}/submit`
- `GET /api/enrollment/{tenantSlug}/care-offerings` — active offerings for the chosen school year whose application window is currently open (`application_window_start <= now() <= application_window_end`, nulls = unbounded); admin view uses a separate endpoint that returns all regardless of window
- `GET /api/enrollment/requests/{statusToken}` — returns current status for each child + per-child reasons (gated by setting)
- `PATCH /api/enrollment/requests/{statusToken}` — edit payload; allowed only while ALL children are still `submitted`
- `POST /api/enrollment/requests/{statusToken}/withdraw` — optional `child_id` in body; omit to withdraw all non-terminal children

**Admin (tenant JWT, `config:update` or custom `enrollment:manage`):**
- `GET/PATCH /api/enrollment/requests`
- `POST /api/enrollment/requests/{id}/children/{childId}/approve`
- `POST /api/enrollment/requests/{id}/children/{childId}/waitlist`
- `POST /api/enrollment/requests/{id}/children/{childId}/reject`
- `POST /api/enrollment/requests/{id}/decisions` — bulk per-child decisions in one payload (preferred UX path)
- `GET/POST/PUT/DELETE /api/enrollment/care-offerings` — catalog CRUD per school year
- `GET/POST/PUT /api/enrollment/schema` — form schema CRUD (versioned)
- `GET /api/enrollment/requests/{id}/email-status` — per-request outbox rows (status, attempts, last_error)
- `POST /api/enrollment/requests/{id}/resend-invitation` — re-enqueue a `guardian_invitation` outbox row
- `GET /api/enrollment/requests/export.xlsx`

**Guardian invitation endpoints** (fill missing pieces):
- `POST /api/auth/guardian-invitations/accept`
- `POST /api/auth/guardian-invitations/{id}/resend`

### 5.4 Frontend pages

- `/{tenant}/enroll` — public form, dynamic from schema, multi-child, includes care-offering selection
- `/{tenant}/enroll/submitted` — confirmation + status/edit link
- `/{tenant}/enroll/status/[token]` — persistent parent status page: per-child status, reasons (if setting allows), per-child withdraw button (only for non-terminal children), edit button (only if request is still fully `submitted`)
- `/{tenant}/admin/enrollments` — list with filters (status, school year, submission date, care offering)
- `/{tenant}/admin/enrollments/[id]` — detail + inline edit + per-child approve/waitlist/reject controls + email delivery status panel with resend button
- `/{tenant}/admin/enrollment-form` — schema editor (add/edit/delete/reorder fields)
- `/{tenant}/admin/care-offerings` — care-offering catalog editor (per school year)
- `/accept-guardian-invite` — mirrors existing `/accept-invite` for staff

Dynamic field rendering reuses the component set built for the settings page.

### 5.5 Email templates

- `enrollment-submitted.html` — confirmation to parent, includes status/edit link
- `enrollment-admin-notification.html` — to admins on new submission
- `enrollment-decision-digest.html` — parent email summarizing per-child decisions (approved / waitlisted / rejected) with status page link and — if any child approved — the guardian invitation link
- `enrollment-waitlisted.html` — optional standalone waitlist notification (used only when `notify_per_decision = immediate`)
- `enrollment-approved.html` — optional standalone approval notification (used only when `notify_per_decision = immediate`)
- `enrollment-rejected.html` — optional standalone rejection notification (used only when `notify_per_decision = immediate`)
- `guardian-invitation.html` — exists, needs the service layer behind it completed; now rendered via outbox rather than dispatched directly

### 5.6 Settings to expose

Registered in `services/config/defaults/enrollment.go`, new "Enrollment" tab. All write-gated by `config:update` (or `config:manage` for sensitive ones):

| Key | Type | Default | Purpose |
|-----|------|---------|---------|
| `enrollment.enabled` | boolean | `false` | Master toggle for the public form |
| `enrollment.open_window_start` | date | — | Accept submissions from |
| `enrollment.open_window_end` | date | — | Accept submissions until |
| `enrollment.collect_grade_level` | boolean | `true` | Show grade level field |
| `enrollment.care_offerings_enabled` | boolean | `true` | Show care-offering selection on the form |
| `enrollment.care_offerings_required` | boolean | `false` | Force parents to pick at least one offering |
| `enrollment.default_activation_mode` | select (`immediate`/`scheduled`) | `scheduled` | When approved students become active |
| `enrollment.notification_emails` | text | — | CSV of admin emails to notify on new submission |
| `enrollment.auto_invite_guardian_on_approval` | boolean | `true` | Some schools handle accounts manually |
| `enrollment.duplicate_handling` | select (`block`/`warn`/`ignore`) | `warn` | How to treat repeat submissions |
| `enrollment.allow_submission_edit` | boolean | `true` | Enable parent edit link (while fully `submitted`) |
| `enrollment.require_captcha` | boolean | `true` | Spam protection on public form |
| `enrollment.rejected_retention_days` | number | `90` | Auto-delete rejected submissions after N days |
| `enrollment.waitlist_enabled` | boolean | `true` | Show waitlist as an admin decision option |
| `enrollment.show_status_reason_to_parent` | boolean | `true` | Include admin's per-child reason on the status page + email |
| `enrollment.notify_per_decision` | select (`digest`/`immediate`) | `digest` | One email after admin saves all decisions vs. email per child decision |
| `enrollment.outbox_max_attempts` | number | `6` | Retries before outbox rows are marked `failed` |
| `enrollment.status_token_ttl_days` | number | `365` | How long the parent status/edit token remains valid. **Operator-only writeable** (write permission `platform:config:update`); tenant admins can read but not edit. |

---

## 6. Value Resolution & Approval Flow

### 6.1 Per-child decisions (transactional)

Admin submits decisions for one or more children in a request. For each child in one DB transaction:

**Approve:**
1. Create `guardian_profile` if not already present (matched by email, tenant-scoped)
2. Create `users.students` row, `status='pending'`, `enrolled_from` = `child.activate_on` or the `calendar_period.start_date`; write grade level into the existing student field (no new assignment table)
3. Create `users.students_guardians` link (relationship='parent', is_primary=true if first child for this guardian)
4. For each `request_child_offerings` selection where the referenced `care_offering.activity_group_id IS NOT NULL`: insert an `activities.student_enrollments` row with `valid_from=calendar_period.start_date`, `valid_until=calendar_period.end_date`, `calendar_period_id=...`. Care offerings without a linked activity group are recorded on the request but not materialized into activities.student_enrollments (admin can wire the group later).
5. Set `request_children.status='approved'`, `reviewed_at`, `reviewed_by`, `created_student_id`
6. If this is the FIRST approved child in the request AND no prior guardian invitation outbox row exists: enqueue one `guardian_invitation` outbox row

**Waitlist:**
- Set `request_children.status='waitlisted'`, `status_reason`, `reviewed_at`, `reviewed_by`. No student created.

**Reject:**
- Set `request_children.status='rejected'`, `status_reason`, `reviewed_at`, `reviewed_by`. No student created.

**Notification outbox rows** (written in the same transaction):
- If `notify_per_decision = digest`: enqueue exactly one `enrollment-decision-digest` row addressed to the parent after all children in the payload have been processed.
- If `notify_per_decision = immediate`: enqueue one `enrollment-approved` / `enrollment-waitlisted` / `enrollment-rejected` row per child.

Request-level `status` is derived (not stored): `submitted` while every child is still `submitted`, `under_review` while at least one admin has viewed but no decisions exist, `partial` if some decisions made but others pending, `finalized` once all children are in a terminal status. This is a materialized view or a computed getter on the model — not a column.

Audit log entries are written per child decision (reuse existing audit schema or settings audit pattern).

### 6.2 Outbox worker

Runs every 30–60 seconds via the scheduler. Pseudocode:

```
SELECT id, kind, payload FROM platform.email_outbox
  WHERE status = 'pending' AND next_retry_at <= now()
  ORDER BY created_at
  FOR UPDATE SKIP LOCKED
  LIMIT N;

for each row:
  UPDATE status='sending'
  try: render template, dispatch via mailer
  success: UPDATE status='sent', sent_at=now()
  failure: attempts++, last_error=err,
           if attempts >= enrollment.outbox_max_attempts: status='failed'
           else: status='pending', next_retry_at = now() + backoff(attempts)
```

`FOR UPDATE SKIP LOCKED` allows multiple workers safely (future scale).

### 6.3 Withdrawal rules

- **Before any admin action** (all children `submitted`): parent can withdraw individual children or the whole request; can also edit the payload.
- **After any decision** (any child is not `submitted`):
  - `submitted`, `under_review`, `waitlisted` children → parent can still withdraw (becomes `withdrawn`, no student impact)
  - `approved` children → withdrawal disabled in UI; parent must contact admin (student record exists)
  - `rejected` children → withdrawal unnecessary (terminal)
- Editing the payload is disabled once any child has a non-`submitted` status.

### 6.4 Activation (scheduled)

Daily scheduler job per tenant:
```sql
UPDATE users.students
SET status = 'active', updated_at = now()
WHERE tenant_id = $1
  AND status = 'pending'
  AND enrolled_from <= CURRENT_DATE;
```

Inverse for `active → inactive` when `enrolled_until <= CURRENT_DATE`.

Slog entries use student IDs only (GDPR — no names at info level).

---

## 7. Phased Delivery

### Phase 1: Foundation (no user-visible enrollment feature yet)

- **PR 1** — `schedule.calendar_periods` table + model + repo + admin CRUD API + admin UI. **Coordinate with Yannick** — if the Timetable RFC ships the full table first, this PR becomes a no-op. Otherwise ship a minimal subset (`id, tenant_id, name, period_type, start_date, end_date, is_active`) that the Timetable RFC can extend later with the week-cycle columns.
- **PR 2** — Student lifecycle fields (`status`, `enrolled_from`, `enrolled_until`) + scheduler activation job + tests. No `student_year_assignments` table — year-scoped progression is handled by the Timetable RFC's enrollment-validity pattern on `activities.student_enrollments`.
- **PR 3** — Complete `guardian_invitation_service` + `/accept-guardian-invite` frontend page

### Phase 2: Enrollment MVP

- **PR 4** — Enrollment settings registry entries + new "Enrollment" settings tab
- **PR 5** — Enrollment schema tables (`form_schemas`, `requests`, `request_children`, `email_outbox`) + form_schema service + admin schema editor UI + outbox worker skeleton
- **PR 6** — Care offerings (`care_offerings`, `request_child_offerings`) + admin catalog editor UI
- **PR 7** — Public enrollment form + submission endpoint + confirmation email (via outbox) + status/edit token flow + parent status page
- **PR 8** — Admin review UI + per-child decision service + outbox delivery status panel + resend button (wires Phase 1 + 2 together)

### Phase 3: Polish

- **PR 9** — XLSX export (structured columns for care offerings + custom fields)
- **PR 10** — File attachments (storage + virus scan — separate design)
- **PR 11** — Re-enrollment flow for existing students
- **PR 12** — Waitlist promotion UX refinements (drag-order waitlist, auto-promote on capacity free, etc.) — only if demand appears

Each PR is independently shippable and gated by `enrollment.enabled` (off by default) until Phase 2 lands.

---

## 8. Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| **Legacy `school_class` field consumers break during migration** | Keep the string column populated from current-year assignment via a repository hook; plan a dedicated cleanup PR once all consumers read the new table |
| **Form schema version drift** | Submitted requests pin to `schema_id`; rendering fetches by ID, not "active" |
| **Status token abuse / enumeration** | Token is cryptographically random (256-bit), long TTL for status view but write actions (edit/withdraw) are rule-gated; rate-limited endpoint |
| **Spam submissions on public form** | Turnstile/hCaptcha (setting-gated), per-IP rate limit, per-email rate limit |
| **GDPR — rejected submissions linger** | Auto-delete after `enrollment.rejected_retention_days` (default 90) via scheduler |
| **PyrePortal impact** | None. Enrollment is tenant admin UI + public form only. No IoT endpoint changes. |
| **Cross-tenant leakage** | All new tables get RLS policies matching existing tenant-scoped tables; integration tests verify isolation |
| **Email deliverability for guardian invites** | Outbox pattern + retry with exponential backoff; admin-visible delivery status; verify DKIM/SPF on the from-domain |
| **Outbox worker stalls / duplicates** | `FOR UPDATE SKIP LOCKED` on pickup; idempotent dispatch keyed by outbox ID; monitoring/alert if `failed` count grows |
| **Care offering capacity race** | Capacity check on submit uses `SELECT ... FOR UPDATE` within the submission transaction; over-capacity requests get a friendly error or land on waitlist automatically (setting TBD) |
| **Stale care-offering catalog (parent applies after service started)** | Each offering carries its own `application_window_start`/`_end`; public endpoint filters to offerings with an open window; service validates on submit so a cached browser can't bypass; admin view still lists closed offerings so they can be reopened or cloned |
| **Partial decisions leaving requests in limbo** | Derived request status makes "partial" visible to admin; admin list has a filter "has pending children"; digest emails only fire when a decision batch is complete |
| **Approved child already has a student record if parent withdraws** | Withdrawal of approved children disabled in parent UI; admin must delete/archive the student manually |

---

## 9. Out of Scope (v1)

- Re-enrollment of existing students for the following year
- File attachments (birth certificate, vaccination record)
- Drag-and-drop form layout editor
- Per-child guardian (different guardian for child A vs child B in one submission) — v1 uses one guardian per request
- Payment/fees collection (price field on care offerings is informational only)
- Class capacity enforcement (admin assigns class manually post-approval)
- Auto-promotion of students between school years (admin action only)
- Automatic waitlist promotion when capacity frees up (manual admin move in v1)
- Parent self-service account creation outside the enrollment flow

---

## 10. Sharp Edges for Implementation

- **Schema versioning** — old submitted requests must still render against their pinned schema after an admin edits the form
- **Hermetic tests** — all new tests MUST use `testpkg.CreateTestStudent` etc.; no hardcoded IDs (see CLAUDE.md)
- **Settings consumers** — anywhere enrollment code reads a setting, follow the `HasTenantOverride` → `Resolve*` → env var → fallback pattern from `.claude/rules/settings-system.md`
- **Migration version uniqueness** — new migration version numbers must not collide in `MigrationRegistry`
- **German UI** — all setting labels/descriptions and email templates in German
- **No production requests from Claude** — local dev / staging only

---

## 11. Cross-Dependencies with Timetable RFC

This plan intentionally defers several concepts to Yannick's Timetable RFC (`docs/timetable-system-plan.md`) rather than building parallel structures. The overlap is substantial and listed here for coordination.

| Concept | Owner | Consumed by this plan |
|---------|-------|-----------------------|
| `schedule.calendar_periods` table | Timetable RFC (E15) | Year picker on submission form; `enrollment.requests.calendar_period_id`; `enrollment.care_offerings.calendar_period_id`; `valid_from`/`valid_until` on created `activities.student_enrollments` rows |
| Auto-generated default `school_year` period per tenant | Timetable RFC (E15) | We assume at least one `school_year` calendar period exists for each active tenant |
| `activities.student_enrollments` with `valid_from`/`valid_until` | Timetable RFC (E17) — extends an existing table | Approval service writes rows here to record approved care offerings |
| `activities.groups` with `type='care'` | Timetable RFC (existing + extension) | `enrollment.care_offerings.activity_group_id` points here |
| Bulk semester rollover endpoint | Timetable RFC (E17, §5.4) | Not consumed directly; relevant for post-v1 re-enrollment flow |
| Student progression across school years | Timetable RFC (enrollment-validity pattern) | Not modeled in this plan; once a year ends, the Timetable RFC's rollover mechanism handles continuation |

### Delivery Scenarios

**Scenario A — Timetable RFC lands first:** This plan's Phase 1 PR 1 is unnecessary. Start from Phase 1 PR 2.

**Scenario B — This plan lands first:** Ship a minimal `schedule.calendar_periods` table in Phase 1 PR 1 (just `id, tenant_id, name, period_type, start_date, end_date, is_active`). Yannick's RFC extends the same table later with the week-cycle columns (additive migration). Similarly ship the additive migration on `activities.student_enrollments` (`valid_from`/`valid_until` nullable, `calendar_period_id` nullable).

**Scenario C — Parallel delivery:** Weekly sync to decide table ownership per PR to avoid migration-number collisions (see Sharp Edges §10).

### Required Coordination Points

- Table ownership for `schedule.calendar_periods` migration — **Yannick ships it (rev 2.3)**; enrollment consumes
- Migration version numbers across both plans must not collide (`MigrationRegistry` is a map — collisions silently overwrite)
- `activities.student_enrollments` column additions — agree who writes them and who runs the data migration
- Confirm `activities.groups.type='care'` is the right mapping for holiday care, daily care, Mensa, etc.
- Error handling when a calendar period is deleted while an enrollment request still references it (FK strategy: `ON DELETE RESTRICT` on both sides)
- Auto-generated default `school_year` period — confirm with Yannick whether his delivery handles this, or enrollment admin UI must create the first period manually

### New Permission Pattern: `platform:config:update`

Rev 2.3 introduces settings that tenant admins can read but only operators (platform scope) can write. This is new — existing settings are tenant-admin-editable. Implementation:

- Add a new write permission `platform:config:update` to the permission catalog
- Grant only to operator role(s)
- Settings declaration uses the new permission in `WritePermission`
- The settings service's permission check already supports this (string compare with wildcard); no service-layer changes needed
- Frontend `writable` flag in the schema response correctly reflects: tenant admin sees the setting as read-only with a "Nur Operator" / "Systemverwaltung" badge; operator UI shows it editable

Apply this to `enrollment.status_token_ttl_days` (first user of the pattern). Other sensitive platform-level knobs should use the same pattern going forward.

---

## 12. Estimated Scope

- Phase 1: ~1 week of focused work (mostly scaffolding, low risk). Reduced if Timetable RFC's PRs land first.
- Phase 2: ~2 weeks (the bulk — form editor, public submission, admin review, approval transaction)
- Phase 3: ~1 week (export, attachments if prioritized)

Total: ~4 engineering weeks for a full v1 + polish, assuming Timetable RFC coordination lands cleanly.
