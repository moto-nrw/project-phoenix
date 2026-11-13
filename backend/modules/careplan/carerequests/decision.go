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
	// ErrWeeklyPlanChanged refuses a weekly approval whose displayed diff no
	// longer matches the live plan (#6843): approving would silently
	// overwrite a change made after the reviewer opened the request.
	ErrWeeklyPlanChanged = errors.New("schedule: care plan changed since the weekly request was reviewed")
	// ErrWeeklyPlanImpactRequired refuses a weekly approval through the
	// decide route that names no reviewed diff.
	ErrWeeklyPlanImpactRequired = errors.New("schedule: weekly request impact token is required")
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
	// WeeklyImpact recomputes the token of the weekly diff the review queue
	// shows. A weekly approval with an ExpectedImpactToken needs it.
	WeeklyImpact WeeklyImpact
}

// WeeklyImpact answers the impact token of the weekly diff the review queue
// currently shows for a pending request (#6843).
type WeeklyImpact interface {
	WeeklyImpactToken(context.Context, Request) (string, error)
}

type Decisions interface {
	Decide(context.Context, DecideInput) (*ReviewItem, error)
	MarkDone(context.Context, int64, string, string, int64) error
}
