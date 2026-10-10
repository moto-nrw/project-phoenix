package users

import (
	"context"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// CaregiverCapabilityService manages the operational caregiver capability of an
// existing account inside a tenant.
type CaregiverCapabilityService interface {
	GetCaregiverCapability(ctx context.Context, accountID int64) (*userModels.CaregiverCapabilityState, error)
	EnableCaregiverCapability(ctx context.Context, accountID int64, input userModels.EnableCaregiverCapabilityInput) (*userModels.CaregiverCapabilityState, error)
	DisableCaregiverCapability(ctx context.Context, accountID int64) (*userModels.CaregiverCapabilityState, error)
}
