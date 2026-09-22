package users

import (
	"context"
	"encoding/json"
	"errors"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
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
	service CaregiverCapabilityService
	db      *bun.DB
}

// NewCaregiverCapabilityViews binds the caregiver capability for the HTTP
// adapters. db opens the administrative transaction of the operator routes.
func NewCaregiverCapabilityViews(service CaregiverCapabilityService, db *bun.DB) CaregiverCapabilityViews {
	return CaregiverCapabilityViews{service: service, db: db}
}

// GetCaregiverCapability reads the capability in the tenant of ctx.
func (v CaregiverCapabilityViews) GetCaregiverCapability(ctx context.Context, accountID int64) (json.RawMessage, error) {
	return caregiverView(v.service.GetCaregiverCapability(ctx, accountID))
}

// EnableCaregiverCapability activates the account as a caregiver in the
// tenant of ctx, completing the missing profile data.
func (v CaregiverCapabilityViews) EnableCaregiverCapability(ctx context.Context, accountID int64, firstName, lastName, position string) (json.RawMessage, error) {
	return caregiverView(v.service.EnableCaregiverCapability(ctx, accountID, userModels.EnableCaregiverCapabilityInput{
		FirstName: firstName,
		LastName:  lastName,
		Position:  position,
	}))
}

// DisableCaregiverCapability removes the capability in the tenant of ctx.
func (v CaregiverCapabilityViews) DisableCaregiverCapability(ctx context.Context, accountID int64) (json.RawMessage, error) {
	return caregiverView(v.service.DisableCaregiverCapability(ctx, accountID))
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

func caregiverView(state *userModels.CaregiverCapabilityState, err error) (json.RawMessage, error) {
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
	if blocked, ok := errors.AsType[*CaregiverCapabilityBlockedError](err); ok {
		var reasons []string
		if blocked.Reasons != nil {
			reasons = make([]string, 0, len(blocked.Reasons))
		}
		for _, reason := range blocked.Reasons {
			reasons = append(reasons, string(reason))
		}
		return caregiverBlockedError{error: blocked, chain: err, blockers: reasons}
	}
	if _, notAssigned := errors.AsType[*AccountNotAssignedToTenantError](err); notAssigned || errors.Is(err, ErrAccountNotFound) {
		return caregiverAccountMissingError{error: err}
	}
	if invalid, ok := errors.AsType[*ValidationError](err); ok {
		return caregiverInvalidError{error: err, invalid: invalid}
	}
	if failure, ok := errors.AsType[*UsersError](err); ok {
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
	invalid *ValidationError
}

func (e caregiverInvalidError) Unwrap() error                  { return e.error }
func (e caregiverInvalidError) CaregiverRequestInvalid() error { return e.invalid }

type caregiverFailureError struct {
	error
	cause error
}

func (e caregiverFailureError) Unwrap() error           { return e.error }
func (e caregiverFailureError) CaregiverFailure() error { return e.cause }
