//go:build testhooks

package api

import (
	"fmt"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"

	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewEnrollmentFlowTimetableRows are the Timetable rows of the enrollment
// flow suites, with the deviation protocol bound to the Audit Platform.
func NewEnrollmentFlowTimetableRows(db transactionruntime.Database) owners.TimetableTestRepositories {
	tt, err := owners.NewTimetableTestRepositoriesWithMembers(db, owners.MustNewMembershipTestRepositories(db))
	if err != nil {
		panic(fmt.Sprintf("enrollment flow test rows: compose timetable: %v", err))
	}
	return withTimetableTestAudit(db, tt)
}

// OrganizationTestRows serve the Organisation & Tenancy owner the suites
// read schools through.
type OrganizationTestRows struct {
	School organizationtenancy.Capability
}

// NewOrganizationTestRows composes the owner over db.
func NewOrganizationTestRows(db transactionruntime.Database) *OrganizationTestRows {
	organizations, err := owners.NewOrganizationTenancy(db)
	if err != nil {
		panic(fmt.Sprintf("organization test rows: %v", err))
	}
	return &OrganizationTestRows{School: organizations}
}
