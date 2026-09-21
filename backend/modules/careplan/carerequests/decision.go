package carerequests

import (
	"context"
	"errors"
)

var (
	ErrNotFound              = errors.New("schedule: care schedule change request not found")
	ErrNotPending            = errors.New("schedule: care schedule change request is not pending")
	ErrNotDecided            = errors.New("schedule: care schedule change request is not decided")
	ErrRejectReasonRequired  = errors.New("schedule: reject reason is required")
	ErrRejectReasonTooLong   = errors.New("schedule: reject reason too long")
	ErrGuardianAccessRevoked = errors.New("schedule: care request guardian access revoked")
)

type DecideInput struct {
	RequestID           int64
	Approve             bool
	Reason              string
	ReasonRequired      bool
	ReviewedBy          int64
	ExpectedImpactToken *string
	RequireImpactToken  bool
	ExpectedVersion     string
}

type Decisions interface {
	Decide(context.Context, DecideInput) (*ReviewItem, error)
	MarkDone(context.Context, int64, string, string, int64) error
}
