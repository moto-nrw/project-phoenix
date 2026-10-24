package compose

import (
	"context"
	"encoding/json"

	boundaryBinding "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type CaregiverCapabilityViews struct {
	application.CaregiverCapabilityViews
	db *bun.DB
}

func NewCaregiverCapabilityViews(service ports.CaregiverCapabilityService, db *bun.DB) CaregiverCapabilityViews {
	return CaregiverCapabilityViews{CaregiverCapabilityViews: application.NewCaregiverCapabilityViews(service), db: db}
}

// GetSchoolAccountCaregiverCapability reads the capability of an account in
// the given school inside an administrative transaction (operator routes).
func (v CaregiverCapabilityViews) GetSchoolAccountCaregiverCapability(ctx context.Context, schoolID, accountID int64) (json.RawMessage, error) {
	return application.GetSchoolAccountCaregiverCapability(ctx, schoolID, accountID, v.inSchool, v.GetCaregiverCapability)
}

// EnableSchoolAccountCaregiverCapability activates the capability of an
// account in the given school inside an administrative transaction.
func (v CaregiverCapabilityViews) EnableSchoolAccountCaregiverCapability(ctx context.Context, schoolID, accountID int64, firstName, lastName, position string) (json.RawMessage, error) {
	return application.EnableSchoolAccountCaregiverCapability(ctx, schoolID, accountID, firstName, lastName, position, v.inSchool, v.EnableCaregiverCapability)
}

// DisableSchoolAccountCaregiverCapability removes the capability of an
// account in the given school inside an administrative transaction.
func (v CaregiverCapabilityViews) DisableSchoolAccountCaregiverCapability(ctx context.Context, schoolID, accountID int64) (json.RawMessage, error) {
	return application.DisableSchoolAccountCaregiverCapability(ctx, schoolID, accountID, v.inSchool, v.DisableCaregiverCapability)
}

func (v CaregiverCapabilityViews) inSchool(ctx context.Context, schoolID int64, fn func(context.Context) (json.RawMessage, error)) (json.RawMessage, error) {
	return boundaryBinding.CaregiverCapabilityViewsInSchoolBinding(ctx, schoolID, fn, tenant.WithAdminTx, v.db)
}
