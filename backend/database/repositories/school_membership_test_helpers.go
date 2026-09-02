package repositories

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	"github.com/uptrace/bun"
)

// NewSchoolMembership composes the staff/teacher/guest owner behind the legacy
// composition seam for test graphs and CLI roots that do not record
// observations. Production roots compose the module themselves (api/base.go)
// so runtime evidence is kept.
func NewSchoolMembership(db *bun.DB) (schoolmembership.Capability, error) {
	return schoolMembershipCompose.New(schoolMembershipCompose.Dependencies{
		DB:      db,
		Observe: func(schoolMembershipCompose.Observation) {},
	})
}
