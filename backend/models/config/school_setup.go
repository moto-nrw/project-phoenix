package config

import (
	"context"
	"fmt"
	"slices"
	"time"
)

// SetupStep names one step of the onboarding wizard for new schools (#2832,
// ADR 0035). The closing screen is not a step: it has nothing to finish.
type SetupStep string

const (
	// SetupStepBasics answers how the school works: presence mode, care plan,
	// fixed groups or open care, parent app.
	SetupStepBasics SetupStep = "basics"
	// SetupStepTeam invites the staff.
	SetupStepTeam SetupStep = "team"
	// SetupStepRooms creates rooms. Only for schools that track rooms.
	SetupStepRooms SetupStep = "rooms"
	// SetupStepGroups creates groups. Only for schools with fixed groups.
	SetupStepGroups SetupStep = "groups"
	// SetupStepStudents creates or imports the children.
	SetupStepStudents SetupStep = "students"
	// SetupStepGuardians invites the parents. Only when the school uses the
	// parent app.
	SetupStepGuardians SetupStep = "guardians"
)

// SetupSteps lists every step in wizard order.
var SetupSteps = []SetupStep{
	SetupStepBasics,
	SetupStepTeam,
	SetupStepRooms,
	SetupStepGroups,
	SetupStepStudents,
	SetupStepGuardians,
}

// ParseSetupStep accepts the known step keys and nothing else.
func ParseSetupStep(raw string) (SetupStep, error) {
	for _, step := range SetupSteps {
		if string(step) == raw {
			return step, nil
		}
	}
	return "", fmt.Errorf("unknown setup step %q", raw)
}

// SchoolSetup is the wizard state of one school. A school without a row is
// new and has not started; every school that existed when the table was
// created got a completed row, so it never sees the wizard.
type SchoolSetup struct {
	ID       int64 `bun:"id,pk,autoincrement" json:"id"`
	TenantID int64 `bun:"tenant_id,notnull" json:"tenant_id"`
	// ParentAppUsed is the school's answer in the first step. It is not a
	// setting: it only decides whether the wizard shows the parent step. nil
	// means the school has not answered yet.
	ParentAppUsed     *bool      `bun:"parent_app_used" json:"parent_app_used"`
	SkippedSteps      []string   `bun:"skipped_steps,array,notnull" json:"skipped_steps"`
	BasicsConfirmedAt *time.Time `bun:"basics_confirmed_at" json:"basics_confirmed_at,omitempty"`
	CompletedAt       *time.Time `bun:"completed_at" json:"completed_at,omitempty"`
	UpdatedBy         *int64     `bun:"updated_by" json:"updated_by,omitempty"`
	CreatedAt         time.Time  `bun:"created_at,notnull,default:now()" json:"created_at"`
	UpdatedAt         time.Time  `bun:"updated_at,notnull,default:now()" json:"updated_at"`
}

func (s *SchoolSetup) GetTenantID() int64   { return s.TenantID }
func (s *SchoolSetup) SetTenantID(id int64) { s.TenantID = id }

// Skipped reports whether the school skipped the step.
func (s *SchoolSetup) Skipped(step SetupStep) bool {
	return s != nil && slices.Contains(s.SkippedSteps, string(step))
}

// SetSkipped adds or removes the step from the skipped list.
func (s *SchoolSetup) SetSkipped(step SetupStep, skipped bool) {
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

// SchoolSetupRepository stores the wizard state of the school in the ambient
// tenant transaction and whether a person hid the wizard.
type SchoolSetupRepository interface {
	// Find returns the school's state, or (nil, nil) for a new school.
	Find(ctx context.Context) (*SchoolSetup, error)
	// Upsert replaces the school's state wholesale.
	Upsert(ctx context.Context, setup *SchoolSetup) error
	// IsDismissed reports whether the account hid the wizard.
	IsDismissed(ctx context.Context, accountID int64) (bool, error)
	// SetDismissed hides or shows the wizard for the account.
	SetDismissed(ctx context.Context, tenantID, accountID int64, dismissed bool) error
}
