package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// CaregiverCapabilityViews binds the tenant and operator capabilities.
type CaregiverCapabilityViews struct {
	*application.SchoolCaregiverCapabilityViews
}

func NewCaregiverCapabilityViews(service ports.CaregiverCapabilityService, db *bun.DB) CaregiverCapabilityViews {
	binding := CaregiverCapabilityViews{}
	return CaregiverCapabilityViews{SchoolCaregiverCapabilityViews: application.NewSchoolCaregiverCapabilityViews(service, binding.inSchool(db), tenant.WithTenantID)}
}

// inSchool binds the registered administrative executor. Runtime view and
// error handling belong to the application; this closure only adapts its
// callback to the existing transaction function's Bun callback signature.
func (CaregiverCapabilityViews) inSchool(db *bun.DB) func(context.Context, func(context.Context) error) error {
	return func(ctx context.Context, callback func(context.Context) error) error {
		return tenant.WithAdminTx(ctx, db, func(adminCtx context.Context, _ bun.Tx) error { return callback(adminCtx) })
	}
}
