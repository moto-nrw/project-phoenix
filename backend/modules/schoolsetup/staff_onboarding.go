package schoolsetup

import (
	"context"
	"errors"
	"slices"
	"time"
)

// The first steps of a care worker (#3748, ADR 0043 addendum): a personal
// checklist of guided tours through the daily work in moto. Unlike the school
// wizard above, progress belongs to the person and a step counts as done once
// the person finished its tour; nothing in the school's data marks it.

// StaffOnboardingService drives the first steps of the calling person.
type StaffOnboardingService interface {
	StaffStatus(ctx context.Context, tenantID, accountID int64) (StaffStatus, error)
	// SetStaffStepState marks a step done, skipped or open again. An unknown
	// step returns ErrUnknownStep, an unknown state ErrUnknownStepState.
	SetStaffStepState(ctx context.Context, tenantID, accountID int64, step, state string) error
	// SetStaffDismissed hides the checklist for the person, or brings it back.
	SetStaffDismissed(ctx context.Context, tenantID, accountID int64, dismissed bool) error
}

// StaffStepKey names one step of the care worker's first steps.
type StaffStepKey string

const (
	// StaffStepGroups shows the person's own group. Only with fixed groups.
	StaffStepGroups StaffStepKey = "groups"
	// StaffStepStudents finds a child and opens its details.
	StaffStepStudents StaffStepKey = "students"
	// StaffStepAttendance checks a child in and out. Only with web attendance.
	StaffStepAttendance StaffStepKey = "attendance"
	// StaffStepCalendar shows the person's appointments and assignments.
	StaffStepCalendar StaffStepKey = "calendar"
	// StaffStepSupervision starts a supervision. Only with rooms tracked.
	StaffStepSupervision StaffStepKey = "supervision"
	// StaffStepWorkTime records working time.
	StaffStepWorkTime StaffStepKey = "work_time"
)

// StaffStepKeys lists every step in checklist order. Which of them apply
// depends on the school's settings and the person's permissions, both of
// which the client already holds; the server only keeps what the person did.
var StaffStepKeys = []StaffStepKey{
	StaffStepGroups,
	StaffStepStudents,
	StaffStepAttendance,
	StaffStepCalendar,
	StaffStepSupervision,
	StaffStepWorkTime,
}

// ParseStaffStepKey accepts the known step keys and nothing else.
func ParseStaffStepKey(raw string) (StaffStepKey, bool) {
	for _, step := range StaffStepKeys {
		if string(step) == raw {
			return step, true
		}
	}
	return "", false
}

// The states a person can give a step.
const (
	StaffStepStateOpen    = "open"
	StaffStepStateDone    = "done"
	StaffStepStateSkipped = "skipped"
)

// ErrUnknownStepState rejects a step state other than open, done or skipped.
var ErrUnknownStepState = errors.New("unknown setup step state")

// StaffStatus is the checklist's whole read model for one person.
type StaffStatus struct {
	// Dismissed hides the checklist for the person, for good.
	Dismissed bool `json:"dismissed"`
	// SchoolReady: the school has a group or a child. Before that the tours
	// would show empty pages, so the checklist waits.
	SchoolReady  bool     `json:"school_ready"`
	DoneSteps    []string `json:"done_steps"`
	SkippedSteps []string `json:"skipped_steps"`
}

// StaffState is the stored progress of one person. A person without a state
// has not started; every account that was active when the table was created
// got a dismissed state, so only people who join afterwards see the
// checklist.
type StaffState struct {
	TenantID     int64
	AccountID    int64
	DoneSteps    []string
	SkippedSteps []string
	DismissedAt  *time.Time
}

// SetStepState moves the step into the given state; a step is never done and
// skipped at once.
func (s *StaffState) SetStepState(step StaffStepKey, state string) {
	s.DoneSteps = without(s.DoneSteps, string(step))
	s.SkippedSteps = without(s.SkippedSteps, string(step))
	switch state {
	case StaffStepStateDone:
		s.DoneSteps = append(s.DoneSteps, string(step))
	case StaffStepStateSkipped:
		s.SkippedSteps = append(s.SkippedSteps, string(step))
	}
}

func without(keys []string, key string) []string {
	kept := make([]string, 0, len(keys)+1)
	for _, candidate := range keys {
		if candidate != key {
			kept = append(kept, candidate)
		}
	}
	return kept
}

// ValidStaffStepState reports whether state is one a person can set.
func ValidStaffStepState(state string) bool {
	return slices.Contains(
		[]string{StaffStepStateOpen, StaffStepStateDone, StaffStepStateSkipped},
		state,
	)
}

// StaffStore keeps the first-steps progress of each person in the ambient
// tenant transaction.
type StaffStore interface {
	// StaffOnboardingOf returns the person's state, or (nil, nil) if the
	// person has not started.
	StaffOnboardingOf(ctx context.Context, accountID int64) (*StaffState, error)
	// StoreStaffOnboarding replaces the person's state wholesale.
	StoreStaffOnboarding(ctx context.Context, state *StaffState) error
}
