# Parent Enrollment — Implementation Execution Plan

**Companion to:** `docs/parent-enrollment-plan.md`
**Audience:** Claude (across multiple sessions) + human reviewer
**Status:** Drafted 2026-04-16

This document is the step-by-step execution plan for implementing the parent enrollment feature. The main plan describes **what** we're building. This document describes **how** to build it PR-by-PR, how to start each session cleanly, and how to hand off state between sessions.

---

## 0. How to Run This Across Multiple Claude Sessions

### Recommendation: one session per PR

Running one session per PR is the right default. Reasons:

- Each PR is a discrete shippable unit; scope fits cleanly in a single session's context
- Earlier PRs land on `development` and become part of the readable repo — Claude doesn't need the full history of how they were built, only what they now do
- Long sessions accumulate noise: failed commands, stale file reads, old tool outputs. A fresh session is faster, cheaper, and more focused
- Context is preserved **through artifacts** (the plan docs, the memory system, CLAUDE.md), not through chat history

### When NOT to start a new session

Stay in the same session if:

- The current PR is still in progress (obvious)
- You're actively iterating with a reviewer in the same sitting
- A tightly-coupled follow-up PR is being prepared and recent context (file layouts, naming choices, test helpers) is still load-bearing

### Context-feeding protocol per new session

At the start of each PR's session, paste this bootstrap to Claude:

```
We are implementing the parent enrollment feature. Read:
1. docs/parent-enrollment-plan.md (the plan — what we're building)
2. docs/parent-enrollment-implementation-plan.md (this doc — how)

Today we're working on PR <N> — <short title>. Start by reading the
"PR N" section of the implementation plan, then the existing code it
depends on. Check memory for prior-PR outcomes. Ask me any clarifying
questions before editing code.
```

Claude will:
1. Read both docs
2. Read any relevant CLAUDE.md rules (auto-loaded)
3. Read the existing files it will modify
4. Check memory for post-PR handoff notes from earlier PRs
5. Confirm scope before writing code

### End-of-session handoff protocol

Before ending a session, Claude MUST write a single memory entry summarizing:

- Which PR number + title was completed
- Final commit SHA and PR URL
- Any deviations from the implementation plan and why
- Anything surprising for future PRs (unexpected refactors, convention discoveries, test helper additions)
- Anything deliberately deferred that a later PR must pick up

Memory entry name: `enrollment_pr_<N>_handoff.md`, type: `project`. Keep it under 200 words.

### What lives where

| State | Lives in |
|-------|----------|
| What we're building | `docs/parent-enrollment-plan.md` |
| How we're building it | This doc |
| What's been finished | Git history + per-PR memory handoff notes |
| Session-only state | Stays in the session (don't persist) |
| Open questions awaiting team answers | §4 of the plan doc |

---

## 1. Pre-flight checks (once, before PR 2)

Before starting the first implementation PR, Claude should verify:

- [ ] `docs/student-enrollment-plan-v2` branch merged to `development` (or the plan doc is otherwise accessible on the working branch)
- [ ] The `guardian_invitation` table exists (migration `001006016`)
- [ ] The `accounts_parents` table exists but is unwired (we are deliberately NOT touching it)
- [ ] Recent CLAUDE.md rules loaded: hermetic tests, settings system, no-fallbacks, env-docker-sync
- [ ] `cd backend && go test ./...` passes on a clean `development` checkout
- [ ] `cd frontend && pnpm run check` passes on a clean `development` checkout

If any check fails, STOP and surface it to the user before proceeding.

---

## 2. PR-by-PR Execution Plan

Each PR section is self-contained: read it, do it, ship it. PRs 2–5 can be worked in parallel by separate sessions if needed (see dependency table at the end).

Feature flag: every user-visible addition is gated by `enrollment.enabled` (default `false`) — set up in PR 4. Until PR 4 lands, PRs 2/3 ship silent scaffolding only.

---

### PR 2 — Student lifecycle + activation scheduler

**Goal:** Introduce `pending`/`active`/`inactive`/`alumnus` status on students, plus `enrolled_from`/`enrolled_until` date fields, and a daily scheduler job that flips `pending → active` when the date arrives.

**Why first:** zero external dependencies (no calendar_periods, no guardian service). Pure backend change. Enables later PRs to create students in `pending` state.

**Scope:**
- Migration: add `status`, `enrolled_from`, `enrolled_until` to `users.students`
  - Default `status='active'` for existing rows (no behavior change)
  - Indexes: `(tenant_id, status)` and `(tenant_id, enrolled_from) WHERE status='pending'`
- Model: extend `users/student.go` with the new fields + `StudentStatus` typed string constants
- Repository: helper `UpdateStatus(ctx, studentID, newStatus)` + a query method `FindPendingDueForActivation(ctx, tenantID, asOf time.Time)`
- Service: `services/scheduler/activate_students.go` — per-tenant iteration following `forEachTenantSettings()` pattern. Idempotent — safe to run multiple times per day.
- Wire into the scheduler bootstrap in `services/scheduler/scheduler.go`
- Setting: `students.activation_scheduler_interval_minutes` (default 60)

**Files touched (estimate):**
- `backend/database/migrations/<next>_student_lifecycle.up.sql` + `.down.sql`
- `backend/models/users/student.go`
- `backend/database/repositories/users/student_repo.go`
- `backend/services/scheduler/activate_students.go` (new)
- `backend/services/scheduler/scheduler.go` (wire in)
- `backend/services/config/defaults/operations.go` (register setting)
- `backend/models/config/keys.go` (key constant)
- Tests for all of the above

**Tests:**
- Model: status enum round-trip, field marshaling
- Repo: `FindPendingDueForActivation` returns only pending students with `enrolled_from <= asOf` and respects tenant scope
- Scheduler: job flips exactly the due rows, leaves others, respects tenant RLS
- Hermetic: use `testpkg.CreateTestStudent`; no hardcoded IDs

**Acceptance criteria:**
- Existing students keep working (status defaults to `active`)
- A student created with `status='pending'` and `enrolled_from=yesterday` is flipped to `active` on the next scheduler tick
- A student with `enrolled_until=today` is flipped to `inactive` at the next tick
- Slog at info level: `"student status transition" student_id=X from=Y to=Z` (no names)

**Before merging:**
- `go test ./backend/...` green
- `go test ./backend/test/ -run TestHermeticTestPatterns` green
- Docker rebuild + manual smoke check that scheduler starts without error

**Session handoff note template:**
```
PR 2 merged as <SHA>. Student lifecycle fields live. Scheduler job named
ActivateStudents runs every X minutes. Setting key is <keyname>. No
known deviations from plan.
```

---

### PR 3 — Complete `guardian_invitation_service`

**Goal:** Fill in the missing service layer behind the existing `auth.guardian_invitations` table and email template. Mirror `invitation_service.go` (staff) patterns. Ship the accept-invite frontend page.

**Why it can be PR 3 in parallel:** zero dependency on PR 2. Unblocks enrollment approval (PR 8) AND any other feature that wants to invite guardians.

**Scope:**
- Backend service: `backend/services/auth/guardian_invitation_service.go`
  - `Create(ctx, guardianProfileID, email, createdBy) → *GuardianInvitation`
  - `SendEmail(ctx, invitation)` — dispatches via existing mailer using `guardian-invitation.html`
  - `Accept(ctx, token, password) → (*Account, error)` — creates `auth.accounts` row with the `guardian` base role, marks invitation `accepted_at`
  - `Resend(ctx, invitationID)` — re-issues token if expired, re-sends email
  - `CleanupExpired(ctx)` — scheduler-callable
- API handlers: `backend/api/auth/guardian_invitation_handlers.go`
  - `POST /api/auth/guardian-invitations/accept`
  - `POST /api/auth/guardian-invitations/{id}/resend`
- Frontend: `/accept-guardian-invite/[token]` — mirror the existing `/accept-invite` page. Collect password, confirm password, show guardian's name + email pre-filled.
- Email template: `backend/templates/email/guardian-invitation.html` exists — verify rendering with test context
- Settings: `invitations.guardian_token_expiry_hours` (default 48)

**Files touched (estimate):**
- `backend/services/auth/guardian_invitation_service.go` (new)
- `backend/services/auth/guardian_invitation_service_test.go` (new)
- `backend/api/auth/guardian_invitation_handlers.go` (new)
- `backend/api/auth/guardian_invitation_handlers_test.go` (new)
- `frontend/src/app/accept-guardian-invite/[token]/page.tsx` (new)
- `frontend/src/app/api/auth/guardian-invitations/**` (proxy routes)
- `backend/services/config/defaults/invitations.go` (setting if not already present)

**Tests:**
- Create → token unique, expiry set, email outbox row written (or direct dispatch if we're not doing outbox yet — coordinate with PR 5)
- Accept valid token → account created with guardian role, invitation marked accepted
- Accept expired token → 410 Gone
- Accept reused token → 409 Conflict
- Resend → new token issued, old invalidated
- Hermetic: use `testpkg.CreateTestAccount` etc.

**Acceptance criteria:**
- A row in `auth.guardian_invitations` + a valid token produces a working accept-invite page
- Accepted invitation creates a real `auth.accounts` row with `guardian` role and the correct tenant link
- Expired tokens return a clear German error ("Einladung abgelaufen")
- Existing staff invitation flow (separate table) is unaffected

**Notes / gotchas:**
- If PR 5 (email outbox) hasn't merged yet, dispatch directly via the existing mailer. Leave a TODO comment and a memory note to migrate to outbox when available.
- The existing `services/auth/invitation_service.go` is the reference — stay close to its structure for easier reviews.
- `auth.accounts_parents` exists but is orphaned — DO NOT write to it. Accounts go into `auth.accounts`.

---

### PR 4 — Enrollment settings registry + tab

**Goal:** Register all settings the enrollment feature will consume, create the "Enrollment" tab in the admin settings UI, and land the master feature flag `enrollment.enabled` (default `false`). No behavior changes yet — this is plumbing.

**Why it's PR 4 in parallel:** pure additive work on the settings system. Independent of PR 2/3. Lets later PRs ship behind the flag.

**Scope:**
- Register definitions in `backend/services/config/defaults/enrollment.go` (new file)
- Register all keys in `backend/models/config/keys.go`
- Add `enrollment` tab to the schema builder if not already present (tab definition, German label "Anmeldung")
- Frontend: add the new tab to the settings page. Since the settings page is auto-generated, this is just a registry-side change plus any tab label translation
- Acceptance permissions: `config:update` for operational toggles, `config:manage` for sensitive settings (captcha, retention)

**Settings to register (from §5.6 of the plan):**

All of these with German labels + descriptions. See §5.6 for the full table. Don't forget the rev-2.1 and rev-2.2 additions (care offerings, application windows, outbox, per-decision, etc.).

**Files touched:**
- `backend/services/config/defaults/enrollment.go` (new)
- `backend/models/config/keys.go` (many new constants)
- `backend/services/config/schema_builder.go` (maybe — if tab registry is declarative, likely not)
- Tests in `backend/services/config/defaults/defaults_test.go`

**Tests:**
- All new keys are registered (count, names)
- Each definition has a label, description, valid type, and correct permission
- Select-type settings have `Options.Static` filled
- Boolean dependencies (e.g., `waitlist_enabled`) referenced by other settings are valid keys

**Acceptance criteria:**
- `GET /api/settings/schema` returns the new tab with all enrollment settings
- Admin can toggle `enrollment.enabled` in the UI (even though it controls nothing yet)
- Settings with `depends_on` conditions render correctly (child fields hide when parent is off)

**Gotcha:** Labels and descriptions MUST be in German. See `.claude/rules/settings-system.md`.

---

### PR 5 — Enrollment core tables + form_schema service + email outbox + form editor UI

**Goal:** Ship the `enrollment` schema tables (except care offerings), the form_schemas service with versioning, the email outbox worker, and the admin form editor UI. No public submission endpoint yet — that's PR 7.

**Why this PR is bigger:** all three pieces are tightly coupled. Tables reference each other; form editor consumes the schema service; outbox will be filled by PR 8 but needs to exist so PR 7 can write confirmation emails to it.

**Depends on:** PR 4 (settings keys exist for outbox retries/token TTL). Does NOT depend on PR 1 (calendar_periods — added in a later column-add migration by whichever plan ships it first).

**Scope:**
- Migration: `enrollment.form_schemas`, `enrollment.requests` (calendar_period_id column nullable for now — see emergency plan §3 below), `enrollment.request_children`, `enrollment.email_outbox`. RLS policies on all four.
- Models + repositories for each
- `services/enrollment/form_schema_service.go` — active schema getter, create-version, validate-submission-against-schema
- `services/enrollment/outbox_worker.go` — picks up `pending` rows with `FOR UPDATE SKIP LOCKED`, renders + dispatches, handles retry. Wire into scheduler.
- Admin form editor UI at `/{tenant}/admin/enrollment-form` — simple table: add/edit/delete/reorder fields
- Admin API: schema CRUD endpoints

**Tests:**
- Schema versioning: creating a new version marks the old inactive; submissions pin to their version_id; editing an active schema doesn't break old submissions
- Outbox worker: picks up pending rows, retries on failure, marks failed after max attempts, handles concurrent workers (two goroutines don't double-process the same row)
- Form editor backend: validates field types, rejects duplicate keys, enforces required core fields
- Hermetic: new test file using real tables + tenants

**Acceptance criteria:**
- Admin can create / edit the active schema via UI
- Outbox worker runs and processes test-dispatched emails (use mock mailer in test env)
- RLS verified: tenant A cannot read tenant B's form_schemas or email_outbox rows

**Gotchas:**
- `email_outbox.kind` enum must be comprehensive from day one (avoid migration pain later)
- Form schema version pinning: when the admin edits, the running form should not suddenly change underneath submitters — cache the version ID in the submission flow per PR 7

---

### PR 6 — Care offerings catalog + admin UI

**Goal:** Ship `enrollment.care_offerings` table + admin editor UI for managing the catalog (add/edit/delete/clone for next year). Parent-facing form doesn't consume it yet (PR 7).

**Depends on:** Something has to own `schedule.calendar_periods` before this merges. See §3 "Calendar Periods Emergency Plan" below — if Yannick hasn't shipped the table by the time we reach this PR, ship the minimal subset as part of this PR (not a separate PR).

**Scope:**
- Migration: `enrollment.care_offerings` + `enrollment.request_child_offerings` (the latter is empty until PR 7 writes to it). RLS.
- Model + repo + service per offering entity
- Admin UI at `/{tenant}/admin/care-offerings` — list by calendar period, create/edit/delete/clone form
- API: offering CRUD + parent-facing open-window endpoint (returns offerings filterable by school year + window open)

**Tests:**
- Capacity check logic (null = unlimited)
- Application window filter: offerings with window in the past / future excluded from parent-facing endpoint
- Clone-to-next-year helper: new offerings created with shifted dates but identical module structure
- Admin can see closed offerings; parent endpoint cannot

**Acceptance criteria:**
- Admin UI lets them build a realistic catalog (e.g., "Regelbetreuung", "Ferienbetreuung Ostern", "Ferienbetreuung Sommer")
- Calendar period FK works: deleting a period with offerings is blocked (`ON DELETE RESTRICT`)

---

### PR 7 — Public enrollment form + submission + confirmation email + status/edit page

**Goal:** The public-facing flow. Parents can submit; they get a confirmation email with a status/edit token link; they can view/edit/withdraw from the status page (subject to rules from §6.3 of the plan).

**Depends on:** PR 4 (settings), PR 5 (schemas + outbox), PR 6 (care offerings), PR 3 (so the guardian invite outbox kind is valid — even though we don't invite yet).

**Scope:**
- Public endpoints per plan §5.3
- Public form at `/{tenant}/enroll` — dynamic renderer consuming form_schema + care_offerings
- Confirmation page `/{tenant}/enroll/submitted`
- Status/edit page `/{tenant}/enroll/status/[token]` — displays per-child status (everything will show `submitted` until PR 8 decisions land), withdraw button
- Captcha integration gated by setting `enrollment.require_captcha`
- Rate limiting on the public submit endpoint (per-IP, per-email)
- Outbox row written on submission: `enrollment-submitted` to parent, `enrollment-admin-notification` to the configured admins

**Tests:**
- Submission validates against pinned schema version
- Care offering with closed application window is rejected server-side even if client sends it (defense-in-depth)
- Capacity overflow behavior per setting (blocked/warned/waitlisted)
- Status token auth: parent with token can view/edit/withdraw; without token returns 404
- Edit disabled once any child has a non-`submitted` status (rule from §6.3)
- Withdrawal rules from §6.3 enforced
- Hermetic

**Acceptance criteria:**
- End-to-end manual test: fresh browser → open `/{tenantSlug}/enroll` → fill form → submit → receive email (mock mailer log) → click status link → see submitted state → withdraw → status updated

---

### PR 8 — Admin review UI + per-child decision service + delivery status panel + resend

**Goal:** The big finale. Admin can approve/waitlist/reject per child, decisions write to DB atomically with outbox, status page reflects decisions, guardian invitation triggers on first approval.

**Depends on:** all prior PRs.

**Scope:**
- Admin pages: `/{tenant}/admin/enrollments` list + `/{tenant}/admin/enrollments/[id]` detail
- `services/enrollment/decision_service.go` — the core transactional logic from plan §6.1
- API endpoints per plan §5.3 (admin section)
- Email delivery status panel on the detail page (reads `email_outbox`)
- Resend button → re-enqueues outbox row
- XLSX export endpoint (could be split to a PR 9 if PR 8 gets too big)

**Tests:**
- Decision transaction: student + guardian_profile + students_guardians + activities.student_enrollments + outbox rows all written atomically (rollback test: force a failure mid-transaction, verify no partial state)
- Idempotency: promoting a waitlisted child to approved doesn't duplicate student rows
- Digest vs. immediate notification mode: correct number of outbox rows written per mode
- Guardian invitation only enqueued once per request (even when multiple children are approved)
- RLS: tenant isolation on all admin endpoints

**Acceptance criteria:**
- End-to-end test run by a human: parent submits → admin approves some children / waitlists one / rejects one → parent gets digest email → parent clicks status link → sees correct per-child state → guardian gets invitation email → guardian clicks link → creates account → can log in

---

## 3. Calendar Periods Emergency Plan

**Trigger:** PR 6 or PR 7 is ready to ship and `schedule.calendar_periods` has not been created by Yannick's Timetable RFC work.

**Action:** Ship a minimal version of the table as part of PR 6 (not a separate PR — keep coordination surface small).

### Minimal migration

```sql
-- backend/database/migrations/<next>_calendar_periods_minimal.up.sql

CREATE TABLE IF NOT EXISTS schedule.calendar_periods (
  id            bigserial PRIMARY KEY,
  tenant_id     bigint NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
  name          text NOT NULL,
  period_type   text NOT NULL DEFAULT 'school_year'
    CHECK (period_type IN ('school_year','semester','holiday','custom')),
  start_date    date NOT NULL,
  end_date      date NOT NULL,
  is_active     boolean NOT NULL DEFAULT false,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, name),
  CHECK (end_date > start_date)
);

-- RLS to match existing tenant-scoped tables
ALTER TABLE schedule.calendar_periods ENABLE ROW LEVEL SECURITY;
-- (policy following the existing pattern — copy from a nearby tenant-scoped table)

CREATE INDEX IF NOT EXISTS idx_calendar_periods_tenant_active
  ON schedule.calendar_periods (tenant_id) WHERE is_active;
CREATE INDEX IF NOT EXISTS idx_calendar_periods_tenant_type
  ON schedule.calendar_periods (tenant_id, period_type);
```

### Notes for Yannick's later extension

Yannick's full RFC spec adds:
- `week_cycle_length SMALLINT DEFAULT 1`
- `week_cycle_anchor DATE`

These are additive. His migration just runs `ALTER TABLE schedule.calendar_periods ADD COLUMN ...`. No data migration needed. We should flag this explicitly to him in the PR description so he's not surprised.

### Minimal service

Ship `backend/services/schedule/calendar_period_service.go` with just `Get`, `List`, `Create`, `Update`, `Delete`. Admin UI at `/{tenant}/admin/calendar-periods` — simple list + form. Yannick can extend later.

### Communication

The PR 6 description should include a `### Coordination` section noting:
- We shipped the minimal `schedule.calendar_periods` because the Timetable RFC hadn't landed
- The columns added are a strict subset of the RFC's spec
- Additive extension is safe; no data migration needed
- Link to the Timetable RFC and the enrollment plan's §11

---

## 4. PR Dependency Graph

```
                ┌────────────────────┐
                │ PR 2: student      │
                │   lifecycle        │
                └──────────┬─────────┘
                           │
                ┌──────────▼─────────┐
                │ PR 3: guardian     │
                │   invitation svc   │
                └──────────┬─────────┘
                           │ (can be parallel with PR 2)
                           │
                ┌──────────▼─────────┐
                │ PR 4: enrollment   │
                │   settings + tab   │
                └──────────┬─────────┘
                           │
                ┌──────────▼─────────┐
                │ PR 5: schemas +    │
                │   outbox + editor  │
                └──────────┬─────────┘
                           │
                ┌──────────▼─────────┐
                │ PR 6: care offer.  │  ←── may bundle calendar_periods
                │   catalog + UI     │      emergency migration
                └──────────┬─────────┘
                           │
                ┌──────────▼─────────┐
                │ PR 7: public form  │
                │   + submission     │
                └──────────┬─────────┘
                           │
                ┌──────────▼─────────┐
                │ PR 8: admin review │
                │   + decisions      │
                └────────────────────┘
```

### Parallelizable

- PR 2, PR 3, PR 4 can all start immediately and be worked in parallel — zero cross-dependencies
- PR 5 waits for PR 4 (needs the settings registered)
- PR 6 waits for PR 4 (settings) — can otherwise run parallel with PR 5
- PR 7 waits for PR 5, PR 6, and ideally PR 3 (for invitation kind in outbox)
- PR 8 is the integration PR — needs everything

### If human review capacity is limited

Ship in this strict linear order: 2 → 3 → 4 → 5 → 6 → 7 → 8. Each is shippable behind `enrollment.enabled=false`.

---

## 5. Per-PR Checklist (use every time)

Before opening a PR:

- [ ] Branch targets `development`
- [ ] Commit messages follow `feat:` / `fix:` / `refactor:` / `test:` / `docs:` / `chore:` conventions
- [ ] No "Co-Authored-By: Claude" line
- [ ] `go test ./backend/...` green locally
- [ ] `go test ./backend/test/ -run TestHermeticTestPatterns` green
- [ ] `cd frontend && pnpm run check` green (if frontend touched)
- [ ] Docker rebuild verified if backend changed: `docker compose build server && docker compose up -d server`
- [ ] Migration version numbers don't collide with other in-flight PRs
- [ ] No direct `Resolve*()` calls without `HasTenantOverride` guard (per `.claude/rules/settings-system.md`)
- [ ] All user-facing strings in German
- [ ] No new env vars for per-tenant config — use settings registry
- [ ] All new backend tables have RLS policies
- [ ] All new tests use `testpkg.Create*` helpers, no hardcoded IDs
- [ ] PR description links to the enrollment plan doc + implementation plan
- [ ] For PRs that touch deployed env or frontend env.js: followed `.claude/rules/env-docker-sync.md` checklist

Post-merge:

- [ ] Write handoff memory entry (`enrollment_pr_<N>_handoff.md`)
- [ ] Update this doc if scope/plan deviated meaningfully
- [ ] If the PR introduced new CLAUDE.md-worthy conventions, propose updating CLAUDE.md in a follow-up

---

## 6. Rollback Plan

Every PR must be reversible by a simple revert of its commit(s). Rules:

- DB migrations must have working `.down.sql` files
- Feature flag `enrollment.enabled=false` must fully hide the feature from users — if a merged PR makes anything user-visible even with the flag off, that's a bug to fix before moving on
- Avoid backfills or destructive data changes until the feature has been live and exercised by a real tenant

If a merged PR misbehaves in staging, revert first and debug the revert commit locally.

---

## 7. Known Unknowns (carry forward across sessions)

These are open questions that don't block starting PR 2 but should be resolved before their respective later PR:

| Question | Blocks PR | Source |
|----------|-----------|--------|
| Required consent checkboxes for GDPR (AGB, data processing, contact, photo?) | PR 7 | plan §4 Q1 |
| Default rejection retention in days | PR 5 or PR 8 | plan §4 Q2 (proposed 90) |
| Spam provider choice (Turnstile / hCaptcha / other) | PR 7 | plan §4 Q5 |
| Email sender identity (tenant-specific vs global) | PR 7 | plan §4 Q6 |
| Grade level max (restrict to 4 for OGS or allow 13?) | PR 7 | plan §4 Q7 |
| Care overflow behavior (reject / waitlist / allow) | PR 7 | plan §4 Q9 |
| Digest vs immediate default | PR 8 | plan §4 Q10 |
| Outbox scope (enrollment-only vs platform-shared) | PR 5 | plan §4 Q11 |
| Status token TTL (1 year?) | PR 5 | plan §4 Q12 |
| Calendar period ownership (skip PR 1 / ship minimal in PR 6) | PR 6 | plan §4 Q13 |
| Default school-year period auto-gen | PR 6 | plan §4 Q14 |

---

## 8. Meta: Updating This Document

This plan is a living document for as long as the enrollment feature is under construction. Claude should update it when:

- A PR's actual scope diverged meaningfully from its section above
- A new convention or gotcha emerged that future PRs should inherit
- An open question was resolved (move the answer into the relevant PR section, delete the question from §7)

Do NOT rewrite history — keep a changelog at the top.

### Changelog

- **2026-04-16 (v1):** Initial execution plan drafted by Claude for team review.
