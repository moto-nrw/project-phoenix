package services

import (
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

// CaregiverCapabilityViews binds People Directory's caregiver capability for
// the HTTP adapters that toggle it (#2736). db opens the administrative
// transaction of the operator routes.
func (f *Factory) CaregiverCapabilityViews(db *bun.DB) users.CaregiverCapabilityViews {
	return users.NewCaregiverCapabilityViews(f.CaregiverCapability, db)
}
