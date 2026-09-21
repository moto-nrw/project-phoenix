package account

import (
	"context"
	"encoding/json"
)

// Caregivers is People Directory's caregiver capability as the account
// administration routes use it, in the tenant of the request (#2736). The
// root binds it. The state comes back as the JSON the routes render, so the
// routes need not name People Directory's rows.
type Caregivers interface {
	GetCaregiverCapability(ctx context.Context, accountID int64) (json.RawMessage, error)
	EnableCaregiverCapability(ctx context.Context, accountID int64, firstName, lastName, position string) (json.RawMessage, error)
	DisableCaregiverCapability(ctx context.Context, accountID int64) (json.RawMessage, error)
}

// The caregiver capability reports its failures by behaviour instead of by
// People Directory's error types.
type (
	// caregiverCapabilityBlocked names the bindings that keep the capability
	// from being removed.
	caregiverCapabilityBlocked interface {
		error
		CaregiverCapabilityBlockers() []string
	}
	// caregiverAccountMissing reports an account that is unknown or not in
	// the school.
	caregiverAccountMissing interface {
		error
		CaregiverAccountMissing() bool
	}
	// caregiverRequestInvalid reports rejected input.
	caregiverRequestInvalid interface {
		error
		CaregiverRequestInvalid() error
	}
	// caregiverFailure reports a failed service call and its cause.
	caregiverFailure interface {
		error
		CaregiverFailure() error
	}
)
