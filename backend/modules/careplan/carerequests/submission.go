package carerequests

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

var ErrAlreadyPending = errors.New("schedule: care request already pending for this student")
var ErrPickupChangeCutoffPassed = errors.New("schedule: same-day pickup change cutoff has passed")

// DayCutoff is resolved by the tenant-aware caller. Closed must evaluate its
// clock at the write boundary, after any caller-held locks have been acquired.
type DayCutoff interface{ Closed(calendar.Date) bool }

type PickupChangeCreateInput struct {
	StudentID         int64
	GuardianAccountID int64
	Date              calendar.Date
	PickupTime        time.Time
	Reason            string
	ReasonRequired    bool
	Cutoff            DayCutoff
}

type Submissions interface {
	CreateRequest(context.Context, int64, int64, json.RawMessage) (*Request, error)
	CreatePickupChange(context.Context, PickupChangeCreateInput) (*Request, error)
}

// ValidatePickupChange is shared by submission and guardian edits.
func ValidatePickupChange(input PickupChangeCreateInput, today calendar.Date) (string, error) {
	reason := strings.TrimSpace(input.Reason)
	if input.StudentID <= 0 || input.GuardianAccountID <= 0 || input.Date.IsZero() || input.PickupTime.IsZero() ||
		(reason == "" && input.ReasonRequired) || utf8.RuneCountInString(reason) > 255 {
		return "", ErrInvalidPayload
	}
	if input.Date.Before(today) || input.Date.After(calendar.NewDate(today.Year(), today.Month()+2, today.Day())) {
		return "", ErrInvalidPayload
	}
	if input.Cutoff != nil && input.Cutoff.Closed(input.Date) {
		return "", ErrPickupChangeCutoffPassed
	}
	return reason, nil
}
