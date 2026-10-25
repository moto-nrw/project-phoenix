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

// PickupChangeCreateInput asks for a Gehzeit, an Abholart, or both for one
// day. A zero PickupTime or an empty DepartureMode leaves that leg unchanged
// (#3945); at least one leg is required.
type PickupChangeCreateInput struct {
	StudentID         int64
	GuardianAccountID int64
	Date              calendar.Date
	PickupTime        time.Time
	DepartureMode     string
	Reason            string
	ReasonRequired    bool
	// Cutoff is resolved by the tenant-aware caller; its zero value has no
	// cutoff. A write-boundary caller builds it with NewSameDayCutoffAt, so
	// Closed reads the clock after the caller's locks are held.
	Cutoff SameDayCutoff
}

// MandatoryReasonPickupChange is the Gehzeit change of the legacy submission
// entry point (Service.CreatePickupChangeRequest): it always requires a
// reason, whatever the school's reason policy says. A caller that resolved
// the policy builds its own PickupChangeCreateInput.
func MandatoryReasonPickupChange(studentID, guardianAccountID int64, date calendar.Date, pickupTime time.Time, reason string) PickupChangeCreateInput {
	return PickupChangeCreateInput{
		StudentID: studentID, GuardianAccountID: guardianAccountID,
		Date: date, PickupTime: pickupTime, Reason: reason, ReasonRequired: true,
	}
}

type Submissions interface {
	CreateRequest(context.Context, int64, int64, json.RawMessage) (*Request, error)
	CreatePickupChange(context.Context, PickupChangeCreateInput) (*Request, error)
}

// ValidatePickupChange is shared by submission and guardian edits.
func ValidatePickupChange(input PickupChangeCreateInput, today calendar.Date) (string, error) {
	reason := strings.TrimSpace(input.Reason)
	if input.StudentID <= 0 || input.GuardianAccountID <= 0 || input.Date.IsZero() ||
		(input.PickupTime.IsZero() && input.DepartureMode == "") ||
		(input.DepartureMode != "" && !RequestableDepartureMode(input.DepartureMode)) ||
		(reason == "" && input.ReasonRequired) || utf8.RuneCountInString(reason) > 255 {
		return "", ErrInvalidPayload
	}
	if input.Date.Before(today) || input.Date.After(calendar.NewDate(today.Year(), today.Month()+2, today.Day())) {
		return "", ErrInvalidPayload
	}
	if input.Cutoff.Closed(input.Date) {
		return "", ErrPickupChangeCutoffPassed
	}
	return reason, nil
}
