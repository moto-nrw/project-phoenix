// Package schoolsetup is the public contract of the onboarding wizard for new
// schools (#2832, ADR 0042): the skipped steps, completion and the personal
// hiding of the wizard. Progress is not stored; the school-setup progress
// projection derives it on every read.
//
// Every method expects the caller's tenant transaction on ctx.
package schoolsetup

import (
	"context"
	"errors"
	"slices"
	"time"
)

// Service drives the wizard of one school for the calling person.
type Service interface {
	Status(ctx context.Context, tenantID, accountID int64) (Status, error)
	// SetStepSkipped skips a step or takes the skip back; an unknown step
	// key returns ErrUnknownStep.
	SetStepSkipped(ctx context.Context, tenantID, accountID int64, step string, skipped bool) error
	Complete(ctx context.Context, tenantID, accountID int64) error
	SetDismissed(ctx context.Context, tenantID, accountID int64, dismissed bool) error
}

// StepKey names one step of the wizard. The closing screen is not a step: it
// has nothing to finish.
type StepKey string

const (
	// StepTeam invites the staff.
	StepTeam StepKey = "team"
	// StepRooms creates rooms. Only for schools that track rooms.
	StepRooms StepKey = "rooms"
	// StepGroups creates groups. Only for schools with fixed groups.
	StepGroups StepKey = "groups"
	// StepStudents creates or imports the children.
	StepStudents StepKey = "students"
	// StepGuardians invites the parents.
	StepGuardians StepKey = "guardians"
)

// StepKeys lists every step in wizard order.
var StepKeys = []StepKey{StepTeam, StepRooms, StepGroups, StepStudents, StepGuardians}

// ParseStepKey accepts the known step keys and nothing else.
func ParseStepKey(raw string) (StepKey, bool) {
	for _, step := range StepKeys {
		if string(step) == raw {
			return step, true
		}
	}
	return "", false
}

// The setting values the steps depend on. Settings Platform's registry owns
// them; the wizard only compares.
const (
	PresenceModeDetailed = "detailed"
	GroupModeFixedGroups = "fixed_groups"
)

// Step is one step as the wizard renders it.
type Step struct {
	Key     string `json:"key"`
	Applies bool   `json:"applies"`
	Done    bool   `json:"done"`
	Skipped bool   `json:"skipped"`
}

// Basics are the school's settings that decide which steps apply. The wizard
// does not ask for them: a new school starts with the registry defaults.
type Basics struct {
	PresenceMode string `json:"presence_mode"`
	GroupMode    string `json:"group_mode"`
}

// Status is the wizard's whole read model.
type Status struct {
	// Completed hides the wizard for everybody at the school.
	Completed bool `json:"completed"`
	// Dismissed hides the wizard for the calling person only.
	Dismissed bool   `json:"dismissed"`
	Basics    Basics `json:"basics"`
	Steps     []Step `json:"steps"`
}

var (
	// ErrCompleted rejects writes after the school finished setup.
	ErrCompleted = errors.New("school setup is already completed")
	// ErrIncomplete rejects completion while an applicable step is neither
	// done nor skipped.
	ErrIncomplete = errors.New("school setup has open steps")
	// ErrUnknownStep rejects a step key the wizard does not have.
	ErrUnknownStep = errors.New("unknown setup step")
)

// ConflictCode gives the client a stable code for each conflict of the
// wizard.
func ConflictCode(err error) string {
	switch {
	case errors.Is(err, ErrCompleted):
		return "school_setup_completed"
	case errors.Is(err, ErrIncomplete):
		return "school_setup_incomplete"
	default:
		return "conflict"
	}
}

// State is the stored wizard state of one school. A school without a state
// is new and has not started; every school that existed when the tables were
// created got a completed state, so it never sees the wizard.
type State struct {
	TenantID     int64
	SkippedSteps []string
	CompletedAt  *time.Time
	UpdatedBy    *int64
}

// Skipped reports whether the school skipped the step.
func (s *State) Skipped(step StepKey) bool {
	return s != nil && slices.Contains(s.SkippedSteps, string(step))
}

// SetSkipped adds or removes the step from the skipped list.
func (s *State) SetSkipped(step StepKey, skipped bool) {
	kept := make([]string, 0, len(s.SkippedSteps)+1)
	for _, key := range s.SkippedSteps {
		if key != string(step) {
			kept = append(kept, key)
		}
	}
	if skipped {
		kept = append(kept, string(step))
	}
	s.SkippedSteps = kept
}

// Store keeps the wizard state of the school in the ambient tenant
// transaction and whether a person hid the wizard.
type Store interface {
	// Find returns the school's state, or (nil, nil) for a new school.
	SetupOfSchool(ctx context.Context) (*State, error)
	// Upsert replaces the school's state wholesale.
	StoreSetup(ctx context.Context, state *State) error
	// IsDismissed reports whether the account hid the wizard.
	IsDismissed(ctx context.Context, accountID int64) (bool, error)
	// SetDismissed hides or shows the wizard for the account.
	SetDismissed(ctx context.Context, tenantID, accountID int64, dismissed bool) error
}

// Facts are the existence checks the projection answers.
type Facts struct {
	StaffInvited    bool
	RoomCreated     bool
	GroupCreated    bool
	StudentEnrolled bool
	GuardianInvited bool
}

// Progress is the port onto the school-setup progress projection.
type Progress interface {
	Facts(ctx context.Context, tenantID int64) (Facts, error)
}

// Settings resolves the school's settings that decide which steps apply.
type Settings interface {
	PresenceMode(ctx context.Context) (string, error)
	GroupMode(ctx context.Context) (string, error)
}
