package testutil

import (
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/uptrace/bun"
)

// The People Directory and School Membership compositions the root suites
// (api, cmd) read through since they left the repository composition
// (#2743); composed in services, which the root suites may not import.

// NewUnobservedTimetableDependencies composes the owner capabilities the
// retained repositories read through, unobserved.
func NewUnobservedTimetableDependencies(db *bun.DB) services.TimetableTestDependencies {
	return services.NewUnobservedTimetableDependencies(db)
}

// MustNewMembershipTestRepositories composes the People Directory and School
// Membership repositories the repository test graphs read through.
func MustNewMembershipTestRepositories(db *bun.DB) services.MembershipTestDependencies {
	return services.MustNewMembershipTestRepositories(db)
}

// NewPeopleDirectory composes the People Directory owner, unobserved.
func NewPeopleDirectory(db *bun.DB) (services.PeopleRepositorySuitePeople, error) {
	return services.NewPeopleDirectory(db)
}
