package operator

import (
	"context"
	"encoding/json"
)

// SchoolAccountCaregivers is People Directory's caregiver capability of a
// school account as the operator routes use it (#2736): each call runs in
// an administrative transaction for the given school. The root binds it.
// The state comes back as the JSON the routes render, so the routes need
// not name People Directory's rows.
type SchoolAccountCaregivers interface {
	GetSchoolAccountCaregiverCapability(ctx context.Context, schoolID, accountID int64) (json.RawMessage, error)
	EnableSchoolAccountCaregiverCapability(ctx context.Context, schoolID, accountID int64, firstName, lastName, position string) (json.RawMessage, error)
	DisableSchoolAccountCaregiverCapability(ctx context.Context, schoolID, accountID int64) (json.RawMessage, error)
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
)
