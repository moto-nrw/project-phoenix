---
paths:
  - "backend/auth/**"
  - "backend/api/**"
  - "backend/workflows/**"
  - "backend/modules/**"
  - "backend/database/repositories/**"
  - "frontend/src/**"
---

# Guardian Parent Portal Permissions

Parent portal authorization is relationship-scoped. A parent account can have different authority for different students, so parent portal checks must use the matching student-guardian relationship and its guardian role / permissions. Since #2756 the relationship (`users.student_guardian_relationships`, People Directory) carries the role, and `auth.guardian_student_access` (Identity & Access) carries the permissions; retained code reads both as one `users.StudentGuardian` row through the guardian-link projection.

## Core Rule

Do not authorize parent portal access or writes only from:

- active `auth.account_tenants`
- linked `users.guardian_profiles.account_id`
- existence of a student-guardian relationship

Those facts prove school membership and a guardian relationship. They do not prove parent portal authority.

Parent portal code must check explicit `parent_portal.*` permissions stored on `auth.guardian_student_access.permissions` through the public helpers in `backend/modules/securityruntime`.

## Separate Permission Systems

Staff/admin permissions and parent guardian permissions are different systems:

- Staff/admin permissions are account and tenant scoped. They use `auth.roles`, `auth.permissions`, JWT permissions, and `common.RequiresPermission` (`backend/api/common/security_principal_middleware.go`).
- Parent portal guardian permissions are student relationship scoped. They use `users.student_guardian_relationships.guardian_role` and `auth.guardian_student_access.permissions`.

Do not model per-child parent portal authority only with `auth.roles` or account-level permissions. One person may be a primary guardian for one student and pickup-only for another.

## Operational Fields Are Not Portal Permissions

These fields are not substitutes for explicit parent portal permission checks:

- `can_pickup` means the person may collect the child.
- `is_emergency_contact` means the person can be contacted in emergencies.
- `relationship_type` describes the relationship category.
- `is_primary` may influence default role assignment, but must not replace permission checks.

Role presets such as `primary_guardian`, `legal_guardian`, `co_guardian`, `pickup_only`, `emergency_contact`, `social_worker`, and `custom` may assign default permissions. Runtime enforcement must still check the concrete stored `parent_portal.*` permission.

## Expected Checks

Parent portal flows resolve a permitted child with the required action through
the gate in `backend/workflows/parentportal/internal/care` (`ResolvePermittedChild`):

```go
s.ResolvePermittedChild(ctx, accountID, studentID, securityruntime.GuardianPermissionSickNoteSubmit)
```

An owner command a flow calls (Care Plan, People Directory, Audit Platform)
does not repeat the relationship check: the flow keeps it, with the same
permission constant, before it opens the unit of work.

Use action-specific permissions:

- child visibility: `parent_portal.access`
- sick note submit: `parent_portal.sick_note.submit`
- parent note write: `parent_portal.notes.write`
- enrollment request visibility: `parent_portal.enrollments.view`
- enrollment submit when tied to an existing child: `parent_portal.enrollment.submit`
- meal participation changes: `parent_portal.meal_participation.manage`
- inviting or removing connected accounts: `parent_portal.related_accounts.manage`

Editing an open request needs the permission that submitting it needs.

School-level feature flags still apply after guardian permission passes.

## Where Code Belongs

- Guardian permission constants and pure helpers are exposed through `backend/modules/securityruntime/guardian_role.go`; private decisions live in `backend/modules/securityruntime/internal/application/authorize/`.
- Repository queries may filter by stored `parent_portal.*` permissions when they are the data-access boundary for parent portal visibility.
- Services orchestrate permission checks and feature flags.
- Handlers should not implement new inline guardian authorization rules.

Before adding new parent portal guardian authorization code, search:

```bash
rg "GuardianPermission|parent_portal|StudentGuardianHasPermission|ResolvePermittedChild" backend
```

Then extend the existing helper/service path rather than creating a new authorization mechanism.
