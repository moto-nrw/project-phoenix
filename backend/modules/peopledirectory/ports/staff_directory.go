package ports

import (
	"context"
	"errors"
)

// The staff write refusals. The staff directory raises them and the root's
// staff membership runtime classifies them (#3753, #4007). Their messages are
// diagnostic detail; clients show the code catalog.
var (
	// ErrStaffAdoptionNotPermitted indicates a staff-creation request landed on
	// a person who already carries a live staff record, from a caller that may
	// only create. Adopting that record writes the notes and the caregiver
	// fields of someone who is already in the directory, which is an edit — and
	// POST /api/staff is gated on users:create alone.
	//
	// Since #2906 the required authority is staff:manage — the same one
	// PUT /api/staff/{id} needs. Admins hold it through the admin:* wildcard,
	// so this refuses the direct-API case, not the staff form.
	ErrStaffAdoptionNotPermitted = errors.New("für das Ändern eines vorhandenen Mitarbeiter-Datensatzes fehlt die Berechtigung")

	// ErrStaffLehrkraftCaregiverProfile indicates a caregiver profile was
	// requested for an account holding the Lehrkraft system role (#1772). That
	// role is class_day:read only and is provisioned without a profile on
	// purpose; the role-assignment paths refuse the same combination from the
	// other direction (ErrRoleLehrkraftCaregiverProfile).
	ErrStaffLehrkraftCaregiverProfile = errors.New("ein Lehrkraft-Konto kann kein Betreuungsprofil erhalten")

	// ErrStaffInUse indicates staff has attendance records or active supervisions
	ErrStaffInUse = errors.New("personal kann nicht gelöscht werden: Mitarbeiter/in hat aktive Aufsichten oder Anwesenheitseinträge")
)

// LehrkraftRoleQuery is the consumer-owned port over the Identity & Access
// role administration: whether the account holds the Lehrkraft system role at
// the tenant in context (#3314).
type LehrkraftRoleQuery interface {
	AccountHoldsLehrkraftRole(ctx context.Context, accountID int64) (bool, error)
}

// StaffDirectoryDependencies are the retained collaborators of the staff
// directory.
type StaffDirectoryDependencies struct {
	// Runtime runs the staff writes in the request's tenant transaction.
	Runtime        TenantTxRunner
	Persons        PersonRepository
	Staff          StaffRepository
	Teachers       TeacherRepository
	LehrkraftRoles LehrkraftRoleQuery
	// MayManageStaff reports whether the actor's permissions carry the
	// staff administration authority (staff:manage), under the Security
	// Runtime's wildcard rule.
	MayManageStaff func(actorPermissions []string) bool
}
