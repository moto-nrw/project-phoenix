# Guardian Parent Portal Permissions

Parent portal authorization is relationship-scoped. A parent account can have different authority for different students, so parent portal checks must use the matching `users.students_guardians` row and its guardian role / permissions.

## Core Rule

Do not authorize parent portal access or writes only from:

- active `auth.account_tenants`
- linked `users.guardian_profiles.account_id`
- existence of a `users.students_guardians` row

Those facts prove school membership and a guardian relationship. They do not prove parent portal authority.

Parent portal code must check explicit `parent_portal.*` permissions stored on `users.students_guardians.permissions` through shared helpers in `backend/auth/authorize`.

## Separate Permission Systems

Staff/admin permissions and parent guardian permissions are different systems:

- Staff/admin permissions are account and tenant scoped. They use `auth.roles`, `auth.permissions`, JWT permissions, and `authorize.RequiresPermission`.
- Parent portal guardian permissions are student relationship scoped. They use `users.students_guardians.guardian_role` and `users.students_guardians.permissions`.

Do not model per-child parent portal authority only with `auth.roles` or account-level permissions. One person may be a primary guardian for one student and pickup-only for another.

## Operational Fields Are Not Portal Permissions

These fields are not substitutes for explicit parent portal permission checks:

- `can_pickup` means the person may collect the child.
- `is_emergency_contact` means the person can be contacted in emergencies.
- `relationship_type` describes the relationship category.
- `is_primary` may influence default role assignment, but must not replace permission checks.

Role presets such as `primary_guardian`, `legal_guardian`, `co_guardian`, `pickup_only`, `emergency_contact`, `social_worker`, and `custom` may assign default permissions. Runtime enforcement must still check the concrete stored `parent_portal.*` permission.

## Expected Checks

Parent portal services should resolve a permitted child with the required action:

```go
resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionSickNoteSubmit)
```

Use action-specific permissions:

- child visibility: `parent_portal.access`
- sick note submit: `parent_portal.sick_note.submit`
- parent note write: `parent_portal.notes.write`
- enrollment request visibility: `parent_portal.enrollments.view`
- enrollment submit when tied to an existing child: `parent_portal.enrollment.submit`
- meal participation changes: `parent_portal.meal_participation.manage`

School-level feature flags still apply after guardian permission passes.

## Where Code Belongs

- Permission constants and pure helper functions belong in `backend/auth/authorize`.
- Repository queries may filter by stored `parent_portal.*` permissions when they are the data-access boundary for parent portal visibility.
- Services orchestrate permission checks and feature flags.
- Handlers should not implement new inline guardian authorization rules.

Before adding new parent portal guardian authorization code, search:

```bash
rg "GuardianPermission|parent_portal|StudentGuardianHasPermission|resolvePermittedChild" backend
```

Then extend the existing helper/service path rather than creating a new authorization mechanism.
