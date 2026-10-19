package application

import (
	"context"
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// SchoolCaregiverCapabilityViews applies caregiver operations to the explicit
// school inside the administrative executor supplied at construction.
type SchoolCaregiverCapabilityViews struct {
	CaregiverCapabilityViews
	admin         func(context.Context, func(context.Context) error) error
	schoolContext func(context.Context, int64) context.Context
}

func NewSchoolCaregiverCapabilityViews(service ports.CaregiverCapabilityService, admin func(context.Context, func(context.Context) error) error, schoolContext func(context.Context, int64) context.Context) *SchoolCaregiverCapabilityViews {
	return &SchoolCaregiverCapabilityViews{CaregiverCapabilityViews: NewCaregiverCapabilityViews(service), admin: admin, schoolContext: schoolContext}
}

func (v SchoolCaregiverCapabilityViews) GetSchoolAccountCaregiverCapability(ctx context.Context, schoolID, accountID int64) (json.RawMessage, error) {
	return v.inSchool(ctx, schoolID, func(ctx context.Context) (json.RawMessage, error) { return v.GetCaregiverCapability(ctx, accountID) })
}

func (v SchoolCaregiverCapabilityViews) EnableSchoolAccountCaregiverCapability(ctx context.Context, schoolID, accountID int64, firstName, lastName, position string) (json.RawMessage, error) {
	return v.inSchool(ctx, schoolID, func(ctx context.Context) (json.RawMessage, error) {
		return v.EnableCaregiverCapability(ctx, accountID, firstName, lastName, position)
	})
}

func (v SchoolCaregiverCapabilityViews) DisableSchoolAccountCaregiverCapability(ctx context.Context, schoolID, accountID int64) (json.RawMessage, error) {
	return v.inSchool(ctx, schoolID, func(ctx context.Context) (json.RawMessage, error) {
		return v.DisableCaregiverCapability(ctx, accountID)
	})
}

func (v SchoolCaregiverCapabilityViews) inSchool(ctx context.Context, schoolID int64, fn func(context.Context) (json.RawMessage, error)) (json.RawMessage, error) {
	var view json.RawMessage
	err := v.admin(ctx, func(adminCtx context.Context) error {
		var viewErr error
		view, viewErr = fn(v.schoolContext(adminCtx, schoolID))
		return viewErr
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}
