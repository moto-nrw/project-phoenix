package api

import (
	owners "github.com/moto-ogs/project-phoenix/roots/owners"

	transactionruntime "github.com/moto-ogs/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-ogs/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-ogs/project-phoenix/modules/workforce/compose"
)

// NewUnobservedWorkforce composes the Workforce owner over an unobserved
// School Membership for the command roots, which record no runtime evidence.
func NewUnobservedWorkforce(db transactionruntime.Database) (workforceModule.Capability, error) {
	membership, err := owners.NewSchoolMembership(db)
	if err != nil {
		return nil, err
	}
	return workforceCompose.New(workforceCompose.Dependencies{
		LockStaffAssignment: owners.WorkforceStaffAssignmentLock(membership),
		DB:                  db.Pool(),
		LiveStaffIDs:        owners.WorkforceLiveStaffIDs(membership),
		Observe:             func(workforceCompose.Observation) {},
	})
}
