package compose

import (
	"context"
	"encoding/json"

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
	return v.inSchool(ctx, schoolID, func(ctx context.Context) (json.RawMessage, error) {
		return v.GetCaregiverCapability(ctx, accountID)
	})
}

// EnableSchoolAccountCaregiverCapability activates the capability of an
// account in the given school inside an administrative transaction.
func (v CaregiverCapabilityViews) EnableSchoolAccountCaregiverCapability(ctx context.Context, schoolID, accountID int64, firstName, lastName, position string) (json.RawMessage, error) {
	return v.inSchool(ctx, schoolID, func(ctx context.Context) (json.RawMessage, error) {
		return v.EnableCaregiverCapability(ctx, accountID, firstName, lastName, position)
	})
}

// DisableSchoolAccountCaregiverCapability removes the capability of an
// account in the given school inside an administrative transaction.
func (v CaregiverCapabilityViews) DisableSchoolAccountCaregiverCapability(ctx context.Context, schoolID, accountID int64) (json.RawMessage, error) {
	return v.inSchool(ctx, schoolID, func(ctx context.Context) (json.RawMessage, error) {
		return v.DisableCaregiverCapability(ctx, accountID)
	})
}

func (v CaregiverCapabilityViews) inSchool(ctx context.Context, schoolID int64, fn func(context.Context) (json.RawMessage, error)) (json.RawMessage, error) {
	var view json.RawMessage
	err := tenant.WithAdminTx(ctx, v.db, func(adminCtx context.Context, _ bun.Tx) error {
		var viewErr error
		view, viewErr = fn(tenant.WithTenantID(adminCtx, schoolID))
		return viewErr
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}
