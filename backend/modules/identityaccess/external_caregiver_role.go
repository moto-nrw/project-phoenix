package identityaccess

import (
	"context"
	"strings"
)

// ExternalCaregiverRoleName is the school role every school starts with
// (#4573): the role of an external caregiver with an account. The role store
// keeps custom role names in lower case, so the stored name is "extern"; the
// role is recognised by name, case-insensitively and trimmed.
const ExternalCaregiverRoleName = "Extern"

// ExternalCaregiverRoleDescription is the description the role starts with.
// Migration 1.15.474 stores the same text for existing schools; keep both in
// sync.
const ExternalCaregiverRoleDescription = "Für externe Betreuungskräfte. Kann den Betreuungsplan ansehen und eigene Termine betreuen."

// ExternalCaregiverDefaultPermissions are the permissions the role starts
// with. schedules:read opens the own blocks of the day and running them
// (start, check-in, check-out, complete). It is not narrowed to the own
// blocks (#4573 D7, follow-up): it also opens the planner reads (instances,
// templates, the staff pool with absence reasons, gaps, the deviation
// history), a child's day and week with arrival and pickup times, and, when
// the school sets operational_overview_scope to all_staff, the school-wide
// „Heute“ view. Time tracking, staff, settings, roles, parent communication
// and student documents stay closed. Migration 1.15.474 provisions the same
// list for existing schools; keep both in sync.
var ExternalCaregiverDefaultPermissions = []string{"schedules:read"}

// ExternalCaregiverRoles finds and provisions the school's „Extern“ role.
type ExternalCaregiverRoles interface {
	// EnsureExternalCaregiverRole creates the role with its default
	// permissions at the school unless a role of that name (case-insensitive,
	// trimmed) exists there. Idempotent; never changes an existing role.
	// Runs in the caller's transaction and refuses a context scoped to
	// another school.
	EnsureExternalCaregiverRole(ctx context.Context, tenantID int64) (Role, error)
	// FindExternalCaregiverRole resolves the role at the school in ctx.
	// found=false when the school has no custom role named „Extern“ with
	// base_role 'user'. A context without a school is ErrTenantRequired.
	FindExternalCaregiverRole(ctx context.Context) (Role, bool, error)
}

// IsExternalCaregiverRoleName reports whether a role name names the „Extern“
// role: case-insensitive and ignoring surrounding whitespace.
func IsExternalCaregiverRoleName(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), ExternalCaregiverRoleName)
}
