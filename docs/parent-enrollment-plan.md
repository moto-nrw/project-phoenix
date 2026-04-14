# Parent-Facing Student Enrollment System — Implementation Plan

**Status:** Draft for team discussion
**Target branch:** `development`
**Author:** Drafted with Claude Code, 2026-04-13

---

## 1. Goal

Enable parents to submit a sign-up request for a child (or multiple children) for a future or current OGS school year via a public form on the tenant subdomain. School admins review requests, approve or reject them, and on approval the system creates the student records and invites the parent to create an account — all without manual data entry from the admin.

## 2. Requirements Summary

- Public submission form per tenant (`/{tenant}/enroll`), no login required
- If submitter is logged in as a guardian, prefill their data (optional convenience)
- One submission can include multiple children
- Admin review UI: see all requests, edit fields before approval, approve/reject with note
- On approval: create student records (initially in `pending` status), link guardian, send email invitation to create account
- Guardians without an account get an invitation email with a token; existing accounts get a "new child linked" confirmation
- Admins can edit the enrollment form per tenant (add/remove/reorder custom fields) — core fields are fixed
- Optional behaviors (which fields to collect, activation timing, duplicate handling) go into the settings system
- XLSX export of requests (Phase 2)
- Ability for parents to view/edit/withdraw their submission before approval via a tokenized link in the confirmation email
- File attachments deferred to Phase 2
- Re-enrollment of existing students deferred (not in scope for v1)

## 3. Key Decisions (confirmed)

| Topic | Decision |
|-------|----------|
| **Parent role** | No new role. Reuse existing `guardian` base role, `guardian_profiles`, `guardian_invitations`, and `students_guardians`. The orphaned `auth.accounts_parents` table is NOT wired in — leave it alone or remove in a separate cleanup task. |
| **Student creation timing** | On approval, create `users.students` rows with `status='pending'`. A daily scheduler job flips them to `active` when `enrolled_from <= today`. Schools can choose `immediate` or `scheduled` activation via settings. |
| **School year model** | Add a new `platform.school_years` table (tenant-scoped). Student class assignments become year-scoped via a new `users.student_year_assignments` table. The existing `users.students.school_class` field is kept as a legacy "current class" cache during transition. |
| **Grade level vs class** | Collect **grade level** (integer 1–13) on the form, not specific class ("1a" vs "1b"). Whether the form asks for it is toggled by setting `enrollment.collect_grade_level`. The concrete class (e.g., "1a") is assigned by the admin post-approval. |
| **Year picker** | Parent chooses the target school year from available open years (tenant has ~4 open years max). No setting needed. |
| **Edit/withdraw** | Tokenized link in the confirmation email lets the parent view/edit/withdraw their pending submission. |
| **Form schema editor** | Simple table (add/edit/delete/reorder fields), reusing the settings system's field-type vocabulary. No drag-and-drop layout builder. Versioned — submitted requests pin to their schema version. |
| **Attachments** | Deferred to Phase 2. |
| **Re-enrollment** | Deferred — this form is first-time signup only for v1. |

## 4. Open Questions for Team Review

1. **GDPR / consent checkboxes** — what fixed consent items are legally required on a first-time signup form? Proposed defaults: (a) AGB/terms acceptance, (b) data processing, (c) contact by email. Are there more (photo, emergency contact sharing)?
2. **Rejected-request retention** — how long should rejected submissions stay in the DB before automatic deletion? Proposed default: 90 days, settings-configurable.
3. **School-year management ownership** — who creates school year entries in the admin UI? Operator-level (moto) or tenant-level (school admin)? The current proposal is tenant admin, gated by `config:manage`.
4. **Existing `school_class` string field** — acceptable to keep as a legacy cache that mirrors the current-year assignment, or should we migrate callers immediately? A hard cutover risks breaking multiple features.
5. **Spam protection** — is Turnstile/hCaptcha acceptable, or do we need a different provider? Gated behind `enrollment.require_captcha` setting.
6. **Email sender identity** — emails sent from a tenant-specific from-address, or a global `enrollment@moto-app.de`? DNS/DKIM implications.
7. **Grade levels beyond OGS** — OGS is typically grades 1–4, but the migration allows 1–13. Keep permissive or restrict?

---

## 5. Architecture

### 5.1 Domain model additions

**New schema: `enrollment`**

```
enrollment.form_schemas
  id, tenant_id, version, fields JSONB, is_active, created_by, created_at

enrollment.requests
  id, tenant_id, schema_id (version snapshot), school_year_id,
  status ('submitted'|'under_review'|'approved'|'rejected'|'withdrawn'),
  guardian_first_name, guardian_last_name, guardian_email, guardian_phone,
  guardian_account_id (nullable — set if submitter was logged in),
  consent_flags JSONB,
  custom_data JSONB,
  activation_mode ('immediate'|'scheduled'),
  activate_on DATE (nullable),
  review_note TEXT,
  reviewed_at, reviewed_by,
  edit_token (unique, tokenized edit/withdraw link),
  edit_token_expires,
  submitted_at, updated_at

enrollment.request_children
  id, request_id, first_name, last_name, date_of_birth,
  target_grade_level (nullable, 1–13),
  custom_data JSONB,
  created_student_id (nullable — filled on approval),
  sort_order
```

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
- **`services/enrollment/request_service.go`** — public submit/withdraw/edit (token-auth), admin list/get/patch
- **`services/enrollment/approval_service.go`** — orchestrates the full approval transaction: create student(s) (status=pending), guardian_profile, students_guardians link, student_year_assignment, then call guardian_invitation_service to issue the invite
- **`services/auth/guardian_invitation_service.go`** — complete the half-built service that mirrors the staff `invitation_service`. Issue token, send email via existing dispatcher, accept-and-create-account, cleanup expired, resend
- **`services/scheduler/activate_students.go`** — daily job: `UPDATE users.students SET status='active' WHERE status='pending' AND enrolled_from <= CURRENT_DATE`, and the inverse for `enrolled_until`

### 5.3 API surface

**Public (no auth, token- or tenant-slug-gated):**
- `POST /api/enrollment/{tenantSlug}/submit`
- `GET /api/enrollment/requests/{editToken}`
- `PATCH /api/enrollment/requests/{editToken}`
- `POST /api/enrollment/requests/{editToken}/withdraw`

**Admin (tenant JWT, `config:update` or custom `enrollment:manage`):**
- `GET/PATCH /api/enrollment/requests`
- `POST /api/enrollment/requests/{id}/approve`
- `POST /api/enrollment/requests/{id}/reject`
- `GET/POST/PUT /api/enrollment/schema` — form schema CRUD (versioned)
- `GET /api/enrollment/requests/export.xlsx`

**Guardian invitation endpoints** (fill missing pieces):
- `POST /api/auth/guardian-invitations/accept`
- `POST /api/auth/guardian-invitations/{id}/resend`

### 5.4 Frontend pages

- `/{tenant}/enroll` — public form, dynamic from schema, multi-child
- `/{tenant}/enroll/submitted` — confirmation + edit link
- `/{tenant}/enroll/edit/[token]` — token-auth edit/withdraw
- `/{tenant}/admin/enrollments` — list with filters
- `/{tenant}/admin/enrollments/[id]` — detail + inline edit + approve/reject
- `/{tenant}/admin/enrollment-form` — schema editor (add/edit/delete/reorder fields)
- `/accept-guardian-invite` — mirrors existing `/accept-invite` for staff

Dynamic field rendering reuses the component set built for the settings page.

### 5.5 Email templates

- `enrollment-submitted.html` — confirmation to parent, includes edit/withdraw link
- `enrollment-admin-notification.html` — to admins on new submission
- `enrollment-approved.html` — includes guardian invitation link
- `enrollment-rejected.html` — with optional admin note
- `guardian-invitation.html` — exists, needs the service layer behind it completed

### 5.6 Settings to expose

Registered in `services/config/defaults/enrollment.go`, new "Enrollment" tab. All write-gated by `config:update` (or `config:manage` for sensitive ones):

| Key | Type | Default | Purpose |
|-----|------|---------|---------|
| `enrollment.enabled` | boolean | `false` | Master toggle for the public form |
| `enrollment.open_window_start` | date | — | Accept submissions from |
| `enrollment.open_window_end` | date | — | Accept submissions until |
| `enrollment.collect_grade_level` | boolean | `true` | Show grade level field |
| `enrollment.default_activation_mode` | select (`immediate`/`scheduled`) | `scheduled` | When approved students become active |
| `enrollment.notification_emails` | text | — | CSV of admin emails to notify on new submission |
| `enrollment.auto_invite_guardian_on_approval` | boolean | `true` | Some schools handle accounts manually |
| `enrollment.duplicate_handling` | select (`block`/`warn`/`ignore`) | `warn` | How to treat repeat submissions |
| `enrollment.allow_submission_edit` | boolean | `true` | Enable parent edit/withdraw link |
| `enrollment.require_captcha` | boolean | `true` | Spam protection on public form |
| `enrollment.rejected_retention_days` | number | `90` | Auto-delete rejected submissions after N days |

---

## 6. Value Resolution & Approval Flow

### 6.1 Approval (transactional)

On admin approving a request with N children:

1. For each child:
   - Create `guardian_profile` if not already present (matched by email)
   - Create `users.students` row, `status='pending'`, `enrolled_from` = `request.activate_on` or `school_year.start_date`
   - Create `users.students_guardians` link (relationship='parent', is_primary=true)
   - Create `users.student_year_assignments` row with grade level
2. If guardian has no account yet → create `auth.guardian_invitations` row, send invitation email
3. If guardian already has an account → send "new child linked" notification
4. Mark `request.status='approved'`, `reviewed_at=now()`, `reviewed_by=adminID`
5. Set `created_student_id` on each `request_children` row
6. Audit log entry (reuse settings audit pattern or existing audit schema)

All in one DB transaction; email sends happen via the async dispatcher after commit.

### 6.2 Activation (scheduled)

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

- **PR 1** — `platform.school_years` table + model + repo + admin CRUD API + admin UI
- **PR 2** — Student lifecycle fields (`status`, `enrolled_from`, `enrolled_until`) + `student_year_assignments` table + data migration for existing students + scheduler activation job + tests
- **PR 3** — Complete `guardian_invitation_service` + `/accept-guardian-invite` frontend page

### Phase 2: Enrollment MVP

- **PR 4** — Enrollment settings registry entries + new "Enrollment" settings tab
- **PR 5** — Enrollment schema tables (`form_schemas`, `requests`, `request_children`) + form_schema service + admin schema editor UI
- **PR 6** — Public enrollment form + submission endpoint + confirmation email + edit/withdraw token flow
- **PR 7** — Admin review UI + approval service (wiring together Phase 1 + 2)

### Phase 3: Polish

- **PR 8** — XLSX export
- **PR 9** — File attachments (storage + virus scan — separate design)
- **PR 10** — Re-enrollment flow for existing students

Each PR is independently shippable and gated by `enrollment.enabled` (off by default) until Phase 2 lands.

---

## 8. Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| **Legacy `school_class` field consumers break during migration** | Keep the string column populated from current-year assignment via a repository hook; plan a dedicated cleanup PR once all consumers read the new table |
| **Form schema version drift** | Submitted requests pin to `schema_id`; rendering fetches by ID, not "active" |
| **Edit-token abuse / enumeration** | Token is cryptographically random (256-bit), short TTL (7 days), single-purpose, rate-limited endpoint |
| **Spam submissions on public form** | Turnstile/hCaptcha (setting-gated), per-IP rate limit, per-email rate limit |
| **GDPR — rejected submissions linger** | Auto-delete after `enrollment.rejected_retention_days` (default 90) via scheduler |
| **PyrePortal impact** | None. Enrollment is tenant admin UI + public form only. No IoT endpoint changes. |
| **Cross-tenant leakage** | All new tables get RLS policies matching existing tenant-scoped tables; integration tests verify isolation |
| **Email deliverability for guardian invites** | Reuse existing SMTP infra; verify DKIM/SPF on the from-domain; add `enrollment-submitted` to existing email audit trail if one exists |

---

## 9. Out of Scope (v1)

- Re-enrollment of existing students for the following year
- File attachments (birth certificate, vaccination record)
- Drag-and-drop form layout editor
- Per-child guardian (different guardian for child A vs child B in one submission) — v1 uses one guardian per request
- Payment/fees collection
- Class capacity enforcement (admin assigns class manually post-approval)
- Auto-promotion of students between school years (admin action only)

---

## 10. Sharp Edges for Implementation

- **`UNIQUE (tenant_id) WHERE is_current`** on `school_years` — flipping requires a transaction that unsets the old current first
- **Schema versioning** — old submitted requests must still render against their pinned schema after an admin edits the form
- **Hermetic tests** — all new tests MUST use `testpkg.CreateTestStudent` etc.; no hardcoded IDs (see CLAUDE.md)
- **Settings consumers** — anywhere enrollment code reads a setting, follow the `HasTenantOverride` → `Resolve*` → env var → fallback pattern from `.claude/rules/settings-system.md`
- **Migration version uniqueness** — new migration version numbers must not collide in `MigrationRegistry`
- **German UI** — all setting labels/descriptions and email templates in German
- **No production requests from Claude** — local dev / staging only

---

## 11. Estimated Scope

- Phase 1: ~1 week of focused work (mostly scaffolding, low risk)
- Phase 2: ~2 weeks (the bulk — form editor, public submission, admin review, approval transaction)
- Phase 3: ~1 week (export, attachments if prioritized)

Total: ~4 engineering weeks for a full v1 + polish.
