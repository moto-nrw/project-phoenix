# Parent Guardian Permissions Implementation Plan

## Problem

The parents app currently treats every portal-linked guardian for a student as effectively equal. If an account has an active tenant mapping, a linked `users.guardian_profiles.account_id`, and a `users.students_guardians` relationship to the student, the parent portal can list that child and use enabled parent actions.

That is too broad for real-world guardian relationships. A person may be a legal guardian for one child, a pickup-only contact for another child, and an emergency contact for a third. Parent portal authorization therefore has to be relationship-scoped, not only account-scoped or tenant-scoped.

The existing relationship row already stores useful metadata:

- `relationship_type`
- `is_primary`
- `is_emergency_contact`
- `can_pickup`
- `pickup_notes`
- `emergency_priority`
- `permissions` JSONB

The missing piece is explicit, enforced parent-portal permissions.

## Design Principles

- Staff/admin permissions stay in the existing `auth.roles`, `auth.permissions`, and `RequiresPermission` system.
- Parent portal guardian permissions are per student and live on `users.students_guardians`.
- Staff should manage guardian authority mostly through role presets.
- Runtime enforcement must check concrete stored `parent_portal.*` permissions, not infer access from relationship metadata.
- `can_pickup` remains operational pickup authorization and must not imply parent portal access.
- `is_emergency_contact` remains contact metadata and must not imply parent portal access.
- `relationship_type` describes the relationship category and may influence defaults, but must not replace explicit permission checks.

## Guardian Portal Permissions

Add typed constants near the existing guardian authorization helpers, likely in `backend/auth/authorize/guardian_permission.go` or a sibling file.

```go
const (
	GuardianPermissionPortalAccess     = "parent_portal.access"
	GuardianPermissionSickNoteSubmit   = "parent_portal.sick_note.submit"
	GuardianPermissionNotesWrite       = "parent_portal.notes.write"
	GuardianPermissionEnrollmentsView  = "parent_portal.enrollments.view"
	GuardianPermissionEnrollmentSubmit = "parent_portal.enrollment.submit"
)
```

Add helpers so services never manipulate raw permission maps directly:

```go
func StudentGuardianHasPermission(sg *users.StudentGuardian, permission string) bool
func StudentGuardianGrantPermissions(sg *users.StudentGuardian, permissions ...string)
func StudentGuardianPermissionSet(role GuardianRole) map[string]any
```

The permission JSON shape should be boolean values:

```json
{
  "parent_portal.access": true,
  "parent_portal.sick_note.submit": true
}
```

## Guardian Role Presets

Add a role/preset field separate from `relationship_type`:

```sql
guardian_role TEXT NOT NULL DEFAULT 'custom'
```

Suggested roles:

- `primary_guardian`
- `legal_guardian`
- `co_guardian`
- `emergency_contact`
- `pickup_only`
- `social_worker`
- `custom`

Recommended defaults:

| Role | Default parent portal permissions | Operational defaults |
|---|---|---|
| `primary_guardian` | access, sick notes, notes, enrollments view, enrollment submit | may be primary |
| `legal_guardian` | access, sick notes, notes, enrollments view, enrollment submit | no automatic pickup unless selected |
| `co_guardian` | access, sick notes, notes, enrollments view, enrollment submit | no automatic pickup unless selected |
| `emergency_contact` | none | `is_emergency_contact=true` |
| `pickup_only` | none | `can_pickup=true` |
| `social_worker` | none by default | school-specific/custom |
| `custom` | stored permissions only | stored flags only |

## Migration And Backfill

Create a migration that:

1. Adds `guardian_role` to `users.students_guardians`.
2. Backfills `guardian_role` conservatively.
3. Backfills `permissions` JSONB based on the assigned role.

Recommended backfill:

| Existing row | Backfilled role | Backfilled permissions |
|---|---|---|
| `is_primary = true` | `primary_guardian` | full parent portal permissions |
| `relationship_type IN ('parent', 'guardian')` | `legal_guardian` | full parent portal permissions |
| `can_pickup = true` and relationship is `relative` or `other` | `pickup_only` | no parent portal permissions |
| `is_emergency_contact = true` and no stronger match | `emergency_contact` | no parent portal permissions |
| everything else | `custom` | keep existing permissions or `{}` |

This preserves access for existing likely legal guardians while avoiding automatic portal access for relatives, friends, pickup contacts, or social workers.

## Parent Portal Access Changes

Update parent child lookup in `backend/database/repositories/parent/child_repository.go`.

`ListByAccount` and `FindForAccount` must require `parent_portal.access` on the matching `users.students_guardians` row.

For boolean JSONB values, the SQL predicate can be:

```sql
COALESCE((sg.permissions ->> 'parent_portal.access')::boolean, false) = true
```

This ensures children are visible in the parents app only when the guardian has explicit portal access for that child.

## Parent Write Authorization Changes

Current writes use an ownership check that proves only that a guardian is linked to the child. Replace or extend it with a permission-aware resolver:

```go
resolvePermittedChild(ctx, accountID, studentID, requiredPermission)
```

Use these checks:

| Parent action | Required permission |
|---|---|
| child feature lookup | `parent_portal.access` |
| list sick days | `parent_portal.access` |
| submit sick note | `parent_portal.sick_note.submit` |
| list parent notes | `parent_portal.access` |
| add parent note | `parent_portal.notes.write` |
| view enrollment requests | `parent_portal.enrollments.view` where request is tied to a child, or documented account-level fallback |

School-level feature flags still apply after guardian permission passes.

## Enrollment Rules

Parent-authenticated enrollment submit currently checks active school membership because a new child may not exist yet. Keep that behavior for the first version:

- An active parent account at a school may submit a new enrollment for that school.
- On approval, the primary guardian receives a portal role and full parent portal permissions.
- Additional guardians remain contact-only by default unless staff later grants a stronger role.

Update enrollment approval so it sets relationship permissions explicitly:

| Created link | Role | Permissions |
|---|---|---|
| primary guardian | `primary_guardian` or `legal_guardian` | full parent portal permissions |
| additional guardian | `emergency_contact`, `pickup_only`, or `co_guardian` depending on form semantics | no portal permissions by default |

The current additional-guardian approval path uses `relationship_type="guardian"`, `is_emergency_contact=true`, and `can_pickup=true`. The new `guardian_role` field prevents those contact relationships from accidentally receiving parent portal permissions.

## Guardian API Changes

Update guardian relationship create/update DTOs to accept `guardian_role`.

For the first version, the backend should derive permissions from the selected role server-side. Avoid making the frontend the source of truth for the permission set.

Example request shape:

```json
{
  "relationship_type": "relative",
  "guardian_role": "pickup_only",
  "can_pickup": true,
  "is_emergency_contact": false
}
```

Later, a custom editor can expose individual permission toggles for `guardian_role="custom"`.

## Staff UI Changes

Update the guardian relationship form to include a role preset selector.

Likely files:

- `frontend/src/components/guardians/guardian-relationship-fields.tsx`
- `frontend/src/components/guardians/guardian-form-modal.tsx`
- `frontend/src/components/guardians/guardian-picker-panel.tsx`
- `frontend/src/lib/guardian-api.ts`

The UI should make these concepts distinct:

- relationship type: who the person is
- guardian role: permission preset
- pickup flag: operational pickup authorization
- emergency contact flag: operational emergency contact metadata

Do not imply that pickup-only contacts can use the parent portal.

## Documentation For Agents

Add an AI-facing rule so future changes do not reintroduce broad checks.

Recommended location:

- `.claude/rules/guardian-parent-permissions.md`
- linked from `backend/CLAUDE.md`

The rule must state:

- Staff/admin permissions use `auth.roles`, `auth.permissions`, and `RequiresPermission`.
- Parent portal guardian permissions are different and are scoped to `users.students_guardians`.
- Parent portal access must never be authorized only from `auth.account_tenants`, `guardian_profiles.account_id`, or existence of a guardian link.
- Parent portal services must use shared helpers from `backend/auth/authorize`.
- Operational fields (`can_pickup`, `is_emergency_contact`) are not portal permissions.
- Role presets may assign default permissions, but enforcement must check concrete stored `parent_portal.*` permissions.

## Tests

Backend tests:

- guardian with `parent_portal.access` sees child
- linked guardian without `parent_portal.access` does not see child
- guardian with `parent_portal.sick_note.submit` can submit sick note
- guardian with only `parent_portal.access` cannot submit sick note
- guardian with `parent_portal.notes.write` can add note
- guardian without `parent_portal.notes.write` cannot add note
- school feature flags still reject after guardian permission passes
- migration grants existing primary and parent/legal guardians portal permissions
- migration does not grant pickup-only relatives portal permissions
- enrollment approval grants primary guardian portal permissions
- enrollment approval does not grant additional guardians portal permissions by default

Frontend tests:

- role preset selector updates visible operational defaults
- pickup-only role does not imply parent portal access
- primary/legal roles describe parent portal capabilities
- create/update payload sends `guardian_role`

## Rollout Order

1. Add guardian permission constants and helpers.
2. Add migration and backfill.
3. Update parent child repository filtering.
4. Update parent write authorization.
5. Update enrollment approval defaults.
6. Update guardian API DTOs and service handling.
7. Update staff UI role selector.
8. Add backend and frontend tests.
9. Re-run parent portal and guardian management smoke tests.

## Open Decisions

- Should `relationship_type="guardian"` always migrate to `legal_guardian`, or should some schools manually review those rows first?
- Should `social_worker` have a default read-only parent portal capability, or always be custom?
- Should a parent account with active school membership always be allowed to submit a new-child enrollment, or should that become a separate account-level school permission?
- Should custom per-permission editing ship in the first version, or should the first version expose presets only?
