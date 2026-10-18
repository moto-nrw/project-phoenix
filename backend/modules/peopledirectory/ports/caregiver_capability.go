package ports

import (
	"context"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// CaregiverCapabilityService manages the operational caregiver capability of an
// existing account inside a tenant.
type CaregiverCapabilityService interface {
	GetCaregiverCapability(ctx context.Context, accountID int64) (*peopleValues.CaregiverCapabilityState, error)
	EnableCaregiverCapability(ctx context.Context, accountID int64, input peopleValues.EnableCaregiverCapabilityInput) (*peopleValues.CaregiverCapabilityState, error)
	DisableCaregiverCapability(ctx context.Context, accountID int64) (*peopleValues.CaregiverCapabilityState, error)
}

// CaregiverCapabilityServiceDependencies contains the repositories and ports
// required to manage caregiver capability on existing accounts. The bindings
// that keep an account a caregiver live in tables of other owners; each is
// reached through a port the composition root binds to that owner.
type CaregiverCapabilityServiceDependencies struct {
	RequestActorScope      func(context.Context) (accountID int64, scope string)
	Identity               CaregiverIdentity
	Audit                  CaregiverCapabilityAudit
	PersonRepo             PersonRepository
	StaffRepo              StaffRepository
	CaregiverBindingLock   CaregiverBindingLocker
	TeacherRepo            TeacherRepository
	GroupTeacherRepo       GroupTeacherBlockers
	GroupSubstitutionRepo  SubstitutionBlockers
	GroupSupervisorRepo    SupervisionBlockers
	ActivitySupervisorRepo PlannedSupervisionBlockers
	// RoleAssignments assigns and removes the caregiver role through the
	// Identity & Access role administration (#3314).
	RoleAssignments CaregiverRoleAssignments
}

// CaregiverCapabilityAudit records a caregiver capability change in the
// authentication audit trail (Audit Platform) inside the caller's transaction.
type CaregiverCapabilityAudit interface {
	RecordCaregiverCapabilityChange(ctx context.Context, accountID int64, enabled bool, metadata map[string]any) error
}

// GroupTeacherBlockers lists the group assignments a teacher still holds, as
// caregiver-capability blocker rows.
type GroupTeacherBlockers interface {
	ListGroupTeacherBlockers(ctx context.Context, teacherID, tenantID int64) ([]peopleValues.BlockerGroup, error)
}

// SubstitutionBlockers lists the current and upcoming group handovers a staff
// member holds, as caregiver-capability blocker rows.
type SubstitutionBlockers interface {
	ListActiveSubstitutionBlockers(ctx context.Context, staffID, tenantID int64) ([]peopleValues.BlockerSubstitution, error)
}

// SupervisionBlockers lists the Student Presence supervision a staff member
// still holds today, as caregiver-capability blocker rows.
type SupervisionBlockers interface {
	ListActiveSupervisionBlockers(ctx context.Context, staffID int64) ([]peopleValues.BlockerSupervision, error)
}

// PlannedSupervisionBlockers lists the planned activity supervisions
// (Timetable & Activities) a staff member holds, as caregiver-capability
// blocker rows.
type PlannedSupervisionBlockers interface {
	ListPlannedSupervisionBlockers(ctx context.Context, staffID, tenantID int64) ([]peopleValues.BlockerActivity, error)
}

// CaregiverRoleAssignments is the consumer-owned port over the Identity &
// Access role administration. Both commands join the caller's transaction
// and revoke the account's sessions at the school.
type CaregiverRoleAssignments interface {
	AssignRoleToAccount(ctx context.Context, accountID, roleID int64) error
	RemoveRoleFromAccount(ctx context.Context, accountID, roleID int64) error
}
