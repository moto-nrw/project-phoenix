package application

import (
	"context"
	"encoding/json"
	"errors"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain/userscontract"
	ports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// CaregiverCapabilityViews serves People Directory's caregiver capability to
// the two HTTP adapters that toggle it (#2736): the tenant account
// administration of Identity & Access and the operator's school account
// routes of Organisation & Tenancy. Neither may name the retained People
// Directory rows, so the state comes back as the JSON the routes render and
// every failure carries one of the behaviours below instead of a users type:
//
//   - CaregiverCapabilityBlockers() []string — the capability cannot be
//     removed while these bindings exist;
//   - CaregiverAccountMissing() bool — the account is unknown or not in the
//     school;
//   - CaregiverRequestInvalid() error — the input was rejected;
//   - CaregiverFailure() error — the service failed, with its cause.
//
// Each wrapper keeps the original error text and chain.
type CaregiverCapabilityViews struct {
	service ports.CaregiverCapabilityService
}

// NewCaregiverCapabilityViews binds the caregiver capability for the HTTP
// adapters. db opens the administrative transaction of the operator routes.
func NewCaregiverCapabilityViews(service ports.CaregiverCapabilityService) CaregiverCapabilityViews {
	return CaregiverCapabilityViews{service: service}
}

// GetCaregiverCapability reads the capability in the tenant of ctx.
func (v CaregiverCapabilityViews) GetCaregiverCapability(ctx context.Context, accountID int64) (json.RawMessage, error) {
	return caregiverView(v.service.GetCaregiverCapability(ctx, accountID))
}

// EnableCaregiverCapability activates the account as a caregiver in the
// tenant of ctx, completing the missing profile data.
func (v CaregiverCapabilityViews) EnableCaregiverCapability(ctx context.Context, accountID int64, firstName, lastName, position string) (json.RawMessage, error) {
	return caregiverView(v.service.EnableCaregiverCapability(ctx, accountID, peopleValues.EnableCaregiverCapabilityInput{
		FirstName: firstName,
		LastName:  lastName,
		Position:  position,
	}))
}

// DisableCaregiverCapability removes the capability in the tenant of ctx.
func (v CaregiverCapabilityViews) DisableCaregiverCapability(ctx context.Context, accountID int64) (json.RawMessage, error) {
	return caregiverView(v.service.DisableCaregiverCapability(ctx, accountID))
}

func caregiverView(state *peopleValues.CaregiverCapabilityState, err error) (json.RawMessage, error) {
	if err != nil {
		return nil, caregiverCapabilityError(err)
	}
	view, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	return view, nil
}

func caregiverCapabilityError(err error) error {
	if blocked, ok := errors.AsType[*peopleValues.CaregiverCapabilityBlockedError](err); ok {
		var reasons []string
		if blocked.Reasons != nil {
			reasons = make([]string, 0, len(blocked.Reasons))
		}
		for _, reason := range blocked.Reasons {
			reasons = append(reasons, string(reason))
		}
		return caregiverBlockedError{error: blocked, chain: err, blockers: reasons}
	}
	if _, notAssigned := errors.AsType[*peopleValues.AccountNotAssignedToTenantError](err); notAssigned || errors.Is(err, userscontract.ErrAccountNotFound) {
		return caregiverAccountMissingError{error: err}
	}
	if invalid, ok := errors.AsType[*userscontract.ValidationError](err); ok {
		return caregiverInvalidError{error: err, invalid: invalid}
	}
	if failure, ok := errors.AsType[*userscontract.UsersError](err); ok {
		return caregiverFailureError{error: err, cause: failure.Err}
	}
	return err
}

// caregiverBlockedError reads as the blocked error itself, whose text the
// routes render, and unwraps to the full chain.
type caregiverBlockedError struct {
	error
	chain    error
	blockers []string
}

func (e caregiverBlockedError) Unwrap() error                         { return e.chain }
func (e caregiverBlockedError) CaregiverCapabilityBlockers() []string { return e.blockers }

type caregiverAccountMissingError struct{ error }

func (e caregiverAccountMissingError) Unwrap() error                 { return e.error }
func (e caregiverAccountMissingError) CaregiverAccountMissing() bool { return true }

type caregiverInvalidError struct {
	error
	invalid *userscontract.ValidationError
}

func (e caregiverInvalidError) Unwrap() error                  { return e.error }
func (e caregiverInvalidError) CaregiverRequestInvalid() error { return e.invalid }

type caregiverFailureError struct {
	error
	cause error
}

func (e caregiverFailureError) Unwrap() error           { return e.error }
func (e caregiverFailureError) CaregiverFailure() error { return e.cause }
