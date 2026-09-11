// Package devicescan is the public contract of the device-scan workflow: what
// a kiosk may ask for after it has scanned a card. It carries plain values and
// stable errors only; the workflows that fulfil it live behind it, and the
// HTTP resources under api/iot call nothing else.
//
// The staff clock contract below is served by the staff-clock workflow
// (services/iot/staffclock), which coordinates the public Workforce
// capability for the actual stamps (#2690).
package devicescan

import (
	"context"
	"errors"
	"time"
)

// Staff clock actions a kiosk may request.
const (
	StaffClockActionCheckIn    = "checkin"
	StaffClockActionCheckOut   = "checkout"
	StaffClockActionBreakStart = "break_start"
	StaffClockActionBreakEnd   = "break_end"
)

// Staff clock states the kiosk renders.
const (
	StaffClockStateCheckedOut = "checked_out"
	StaffClockStateCheckedIn  = "checked_in"
	StaffClockStateOnBreak    = "on_break"
)

// Staff clock failures. Every value is a stable code the kiosk maps to its own
// wording; the HTTP resource classifies with errors.Is.
var (
	ErrInvalidRFIDTag  = errors.New("invalid RFID tag")
	ErrRFIDTagNotFound = errors.New("RFID tag not found")
	ErrRFIDTagInactive = errors.New("RFID tag is inactive")
	ErrRFIDTagNotStaff = errors.New("RFID tag is not assigned to staff")
	ErrInvalidAction   = errors.New("invalid staff clock action")
	ErrStatusRequired  = errors.New("status is required for check-in")
	// ErrStaffClockRaced reports that a concurrent scan of the same card
	// already created today's session: a state conflict, not a server fault.
	ErrStaffClockRaced = errors.New("check-in for today was already recorded")
	// ErrStaffClockState reports a stamp that does not fit the staff member's
	// current state (already checked in, no active break, ...). The wording of
	// the wrapped cause is the message the kiosk shows.
	ErrStaffClockState = errors.New("staff clock state conflict")
	// ErrStaffClockInvalid reports a request the workflow rejected before any
	// write, such as an out-of-range planned break length.
	ErrStaffClockInvalid = errors.New("invalid staff clock request")
)

// PlannedStartNotReachedError reports a check-in before the planned start of
// the day while the tenant enforces the planned start. Times are Berlin wall
// clocks.
type PlannedStartNotReachedError struct {
	PlannedStartTime string
	CurrentTime      string
}

func (e *PlannedStartNotReachedError) Error() string { return "planned start not reached" }

// DeviationReasonRequiredError reports a stamp outside the tolerance window
// around the planned shift without the reason the tenant requires.
type DeviationReasonRequiredError struct {
	Action           string
	PlannedTime      string
	ActualTime       string
	DeviationMinutes int
}

func (e *DeviationReasonRequiredError) Error() string { return "deviation reason required" }

// StaffClockCommand is one explicit stamp the kiosk requests.
type StaffClockCommand struct {
	RFIDTag                string
	Action                 string
	Status                 string
	Reason                 string
	PlannedDurationMinutes *int
}

// StaffClockSession is the kiosk projection of a work session. The stored row
// carries tenant, audit and free-text note columns that a shared device in a
// hallway has no use for and must not receive.
type StaffClockSession struct {
	ID           int64      `json:"id"`
	StaffID      int64      `json:"staff_id"`
	CheckInTime  time.Time  `json:"check_in_time"`
	CheckOutTime *time.Time `json:"check_out_time,omitempty"`
	Status       string     `json:"status"`
	Source       string     `json:"source"`
}

// StaffClockBreak is the kiosk projection of an active break: enough to show
// that one is running, nothing more.
type StaffClockBreak struct {
	ID        int64     `json:"id"`
	StartedAt time.Time `json:"started_at"`
}

// StaffClockState is the authoritative kiosk state after a scan or a stamp.
type StaffClockState struct {
	StaffID              int64              `json:"staff_id"`
	StaffName            string             `json:"staff_name"`
	State                string             `json:"state"`
	AllowedActions       []string           `json:"allowed_actions"`
	Session              *StaffClockSession `json:"session,omitempty"`
	ActiveBreak          *StaffClockBreak   `json:"active_break,omitempty"`
	NetMinutes           int                `json:"net_minutes"`
	BreakMinutes         int                `json:"break_minutes"`
	RequiredBreakMinutes int                `json:"required_break_minutes"`
	IsBreakCompliant     bool               `json:"is_break_compliant"`
}

// StaffClock is the NFC staff time-tracking contract of the kiosk.
type StaffClock interface {
	// StaffClockState resolves a staff card and returns the current state.
	StaffClockState(ctx context.Context, rfidTag string) (*StaffClockState, error)
	// ExecuteStaffClock performs one stamp and returns the state afterwards.
	ExecuteStaffClock(ctx context.Context, command StaffClockCommand) (*StaffClockState, error)
}
