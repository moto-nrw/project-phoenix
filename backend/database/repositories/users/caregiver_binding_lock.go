package users

import (
	"context"
	"errors"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// CaregiverBindingOwners is the consumer-owned port over the four owners whose
// tables can retain caregiver capability. Each owner takes a SHARE ROW
// EXCLUSIVE table lock on its own rows for the caller's transaction; People
// Directory never names a foreign table itself.
type CaregiverBindingOwners interface {
	// LockGroupAssignments locks education.group_teacher (School Membership).
	LockGroupAssignments(context.Context) error
	// LockGroupSubstitutions locks education.group_substitution (Workforce).
	LockGroupSubstitutions(context.Context) error
	// LockGroupSupervisions locks active.group_supervisors (Student Presence).
	LockGroupSupervisions(context.Context) error
	// LockPlannedSupervisors locks activities.supervisors (Timetable & Activities).
	LockPlannedSupervisors(context.Context) error
}

type caregiverBindingLocker struct {
	owners CaregiverBindingOwners
}

func NewCaregiverBindingLocker(owners CaregiverBindingOwners) userModels.CaregiverBindingLocker {
	return &caregiverBindingLocker{owners: owners}
}

// LockCaregiverCapabilityBindings takes the four table locks in a fixed order
// inside the caller's transaction. The caller retries the whole transaction on
// a deadlock, because concurrent binding writers lock the same tables in a
// different order.
func (r *caregiverBindingLocker) LockCaregiverCapabilityBindings(ctx context.Context) error {
	if r.owners == nil {
		return errors.New("caregiver binding lock requires the binding owners")
	}
	steps := []struct {
		table string
		lock  func(context.Context) error
	}{
		{"education.group_teacher", r.owners.LockGroupAssignments},
		{"education.group_substitution", r.owners.LockGroupSubstitutions},
		{"active.group_supervisors", r.owners.LockGroupSupervisions},
		{"activities.supervisors", r.owners.LockPlannedSupervisors},
	}
	for _, step := range steps {
		if err := step.lock(ctx); err != nil {
			return &modelBase.DatabaseError{
				Op:  "lock caregiver capability binding table " + step.table,
				Err: err,
			}
		}
	}
	return nil
}
