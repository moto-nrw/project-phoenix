package services

import (
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/uptrace/bun"
)

// CaregiverCapabilityViews binds People Directory's caregiver capability for
// the HTTP adapters that toggle it (#2736). db opens the administrative
// transaction of the operator routes.
func (f *Factory) CaregiverCapabilityViews(db *bun.DB) peopleCompose.CaregiverCapabilityViews {
	return peopleCompose.NewCaregiverCapabilityViews(f.CaregiverCapability, db)
}
