package testutil

import (
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/uptrace/bun"
)

// The guardian portal route suites (modules/careplan/inbound/parent) compose
// the workflow's retained repositories, People Directory and Care Plan's care
// profiles through the legacy composition, which they may not import (#3421).

// NewParentRouteTestRepositories composes the repositories the parent route
// suites hand to the guardian portal workflow.
func NewParentRouteTestRepositories(db *bun.DB) (services.ParentRouteTestRepositories, error) {
	return services.NewParentRouteTestRepositories(db, services.MustNewMembershipTestRepositories(db))
}

// MustNewPeopleDirectory composes the People Directory owner and panics when
// a dependency is missing.
func MustNewPeopleDirectory(db *bun.DB) services.PeopleRepositorySuitePeople {
	return services.MustNewPeopleDirectory(db)
}

// NewParentPortalCareProfiles builds Care Plan's care-profile commands the
// guardian portal writes through.
func NewParentPortalCareProfiles(db *bun.DB) (services.ParentPortalCareProfiles, error) {
	return services.NewParentPortalCareProfiles(db)
}
