package config

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/models/config"
)

// Onboarding wizard for new schools (#2832, ADR 0035).
//
// The state (answers, skipped steps, completion, who hid the wizard) lives in
// Settings Platform's own tables. Progress is not stored: the school-setup-view
// projection reports on every read whether rooms, invitations, groups,
// children and parent invitations exist, and this service derives the steps
// from those facts and the school's settings.
//
// Every method expects the caller's tenant transaction on ctx.

// ErrSchoolSetupCompleted rejects writes after the school finished setup. Once
// completed, the presence mode is operator-only again and the wizard is gone.
var ErrSchoolSetupCompleted = errors.New("school setup is already completed")

// ErrSchoolSetupIncomplete rejects completion while an applicable step is
// neither done nor skipped.
var ErrSchoolSetupIncomplete = errors.New("school setup has open steps")

// ErrSetupStepNotSkippable rejects skipping the first step: the others depend
// on its answers.
var ErrSetupStepNotSkippable = errors.New("the first setup step cannot be skipped")

// ErrInvalidPresenceMode rejects a presence mode outside the registry options.
var ErrInvalidPresenceMode = errors.New("invalid presence mode")

// SchoolSetupFacts are the existence checks the projection answers.
type SchoolSetupFacts struct {
	StaffInvited    bool
	RoomCreated     bool
	GroupCreated    bool
	StudentEnrolled bool
	GuardianInvited bool
}

// SchoolSetupProgress is the consumer-owned port onto the school-setup-view
// projection.
type SchoolSetupProgress interface {
	SchoolSetupFacts(ctx context.Context, tenantID int64) (SchoolSetupFacts, error)
}

// SchoolSetupSettings resolves the settings that decide which steps apply.
type SchoolSetupSettings interface {
	ResolveString(ctx context.Context, key string) (string, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
}

// PresenceModeWriter writes operations.presence_mode for the school on behalf
// of a school admin, in the caller's transaction. The composition binds it to
// the operator settings orchestration so the open-attendance guard, the side
// effect hook and the settings broadcast apply exactly as for an operator.
type PresenceModeWriter func(ctx context.Context, tenantID, accountID int64, mode string) error

// SchoolSetupStep is one step as the wizard renders it.
type SchoolSetupStep struct {
	Key     string `json:"key"`
	Applies bool   `json:"applies"`
	Done    bool   `json:"done"`
	Skipped bool   `json:"skipped"`
}

// SchoolSetupBasics are the answers of the first step.
type SchoolSetupBasics struct {
	PresenceMode     string `json:"presence_mode"`
	GroupMode        string `json:"group_mode"`
	TimetableEnabled bool   `json:"timetable_enabled"`
	// ParentAppUsed is nil until the school answered.
	ParentAppUsed *bool `json:"parent_app_used"`
}

// SchoolSetupStatus is the wizard's whole read model.
type SchoolSetupStatus struct {
	// Completed hides the wizard for everybody at the school.
	Completed bool `json:"completed"`
	// Dismissed hides the wizard for the calling person only.
	Dismissed bool              `json:"dismissed"`
	Basics    SchoolSetupBasics `json:"basics"`
	Steps     []SchoolSetupStep `json:"steps"`
}

// SchoolSetupService drives the onboarding wizard.
type SchoolSetupService struct {
	store    config.SchoolSetupRepository
	progress SchoolSetupProgress
	settings SchoolSetupSettings
	presence PresenceModeWriter
	now      func() time.Time
}

// NewSchoolSetupService wires the wizard. Every dependency is required.
func NewSchoolSetupService(store config.SchoolSetupRepository, progress SchoolSetupProgress, settings SchoolSetupSettings, presence PresenceModeWriter, now func() time.Time) (*SchoolSetupService, error) {
	if store == nil || progress == nil || settings == nil || presence == nil || now == nil {
		return nil, errors.New("school setup service: all dependencies are required")
	}
	return &SchoolSetupService{store: store, progress: progress, settings: settings, presence: presence, now: now}, nil
}

// Status returns the wizard for the calling person.
func (s *SchoolSetupService) Status(ctx context.Context, tenantID, accountID int64) (SchoolSetupStatus, error) {
	setup, err := s.store.Find(ctx)
	if err != nil {
		return SchoolSetupStatus{}, err
	}
	dismissed, err := s.store.IsDismissed(ctx, accountID)
	if err != nil {
		return SchoolSetupStatus{}, err
	}
	basics, err := s.basics(ctx, setup)
	if err != nil {
		return SchoolSetupStatus{}, err
	}
	status := SchoolSetupStatus{
		Completed: setup != nil && setup.CompletedAt != nil,
		Dismissed: dismissed,
		Basics:    basics,
	}
	if status.Completed {
		// A finished school needs no progress read.
		status.Steps = []SchoolSetupStep{}
		return status, nil
	}
	facts, err := s.progress.SchoolSetupFacts(ctx, tenantID)
	if err != nil {
		return SchoolSetupStatus{}, err
	}
	status.Steps = steps(setup, basics, facts)
	return status, nil
}

// ConfirmBasics stores the answers of the first step that are not ordinary
// admin settings: the presence mode, which admins may set only during setup,
// and whether the school uses the parent app. Group mode and care plan go
// through the regular settings API before this call. The step may be
// confirmed again while setup is open.
func (s *SchoolSetupService) ConfirmBasics(ctx context.Context, tenantID, accountID int64, presenceMode string, parentAppUsed bool) error {
	if presenceMode != config.PresenceModeDetailed && presenceMode != config.PresenceModeBinary {
		return ErrInvalidPresenceMode
	}
	setup, err := s.openSetup(ctx, tenantID)
	if err != nil {
		return err
	}
	current, err := s.settings.ResolveString(ctx, config.KeyPresenceMode)
	if err != nil {
		return fmt.Errorf("resolve presence mode: %w", err)
	}
	if current != presenceMode {
		if err := s.presence(ctx, tenantID, accountID, presenceMode); err != nil {
			return err
		}
	}
	now := s.now()
	setup.ParentAppUsed = &parentAppUsed
	setup.BasicsConfirmedAt = &now
	setup.UpdatedBy = &accountID
	return s.store.Upsert(ctx, setup)
}

// SetStepSkipped skips a step or takes the skip back. The first step cannot
// be skipped: the others depend on its answers.
func (s *SchoolSetupService) SetStepSkipped(ctx context.Context, tenantID, accountID int64, step config.SetupStep, skipped bool) error {
	if step == config.SetupStepBasics {
		return ErrSetupStepNotSkippable
	}
	setup, err := s.openSetup(ctx, tenantID)
	if err != nil {
		return err
	}
	setup.SetSkipped(step, skipped)
	setup.UpdatedBy = &accountID
	return s.store.Upsert(ctx, setup)
}

// Complete finishes setup for the whole school once every applicable step is
// done or skipped. Afterwards the wizard no longer opens and the presence mode
// is operator-only again.
func (s *SchoolSetupService) Complete(ctx context.Context, tenantID, accountID int64) error {
	status, err := s.Status(ctx, tenantID, accountID)
	if err != nil {
		return err
	}
	if status.Completed {
		return ErrSchoolSetupCompleted
	}
	for _, step := range status.Steps {
		if step.Applies && !step.Done && !step.Skipped {
			return ErrSchoolSetupIncomplete
		}
	}
	setup, err := s.openSetup(ctx, tenantID)
	if err != nil {
		return err
	}
	now := s.now()
	setup.CompletedAt = &now
	setup.UpdatedBy = &accountID
	return s.store.Upsert(ctx, setup)
}

// SetDismissed hides the wizard for the calling person, or brings it back
// (reachable from settings and help).
func (s *SchoolSetupService) SetDismissed(ctx context.Context, tenantID, accountID int64, dismissed bool) error {
	return s.store.SetDismissed(ctx, tenantID, accountID, dismissed)
}

// openSetup returns the school's state for a write, creating it for a new
// school, and refuses once setup is completed.
func (s *SchoolSetupService) openSetup(ctx context.Context, tenantID int64) (*config.SchoolSetup, error) {
	setup, err := s.store.Find(ctx)
	if err != nil {
		return nil, err
	}
	if setup == nil {
		return &config.SchoolSetup{TenantID: tenantID, SkippedSteps: []string{}}, nil
	}
	if setup.CompletedAt != nil {
		return nil, ErrSchoolSetupCompleted
	}
	return setup, nil
}

func (s *SchoolSetupService) basics(ctx context.Context, setup *config.SchoolSetup) (SchoolSetupBasics, error) {
	presenceMode, err := s.settings.ResolveString(ctx, config.KeyPresenceMode)
	if err != nil {
		return SchoolSetupBasics{}, fmt.Errorf("resolve presence mode: %w", err)
	}
	groupMode, err := s.settings.ResolveString(ctx, config.KeyGroupMode)
	if err != nil {
		return SchoolSetupBasics{}, fmt.Errorf("resolve group mode: %w", err)
	}
	timetableEnabled, err := s.settings.ResolveBool(ctx, config.KeyTimetableEnabled)
	if err != nil {
		return SchoolSetupBasics{}, fmt.Errorf("resolve care plan toggle: %w", err)
	}
	basics := SchoolSetupBasics{
		PresenceMode:     presenceMode,
		GroupMode:        groupMode,
		TimetableEnabled: timetableEnabled,
	}
	if setup != nil {
		basics.ParentAppUsed = setup.ParentAppUsed
	}
	return basics, nil
}

// steps derives the wizard steps. A step that does not apply is never done or
// skipped from the wizard's point of view. The parent step applies until the
// school answered "no": an unanswered question must not hide a step.
func steps(setup *config.SchoolSetup, basics SchoolSetupBasics, facts SchoolSetupFacts) []SchoolSetupStep {
	applies := map[config.SetupStep]bool{
		config.SetupStepBasics:    true,
		config.SetupStepTeam:      true,
		config.SetupStepRooms:     basics.PresenceMode == config.PresenceModeDetailed,
		config.SetupStepGroups:    basics.GroupMode == config.GroupModeFixedGroups,
		config.SetupStepStudents:  true,
		config.SetupStepGuardians: basics.ParentAppUsed == nil || *basics.ParentAppUsed,
	}
	done := map[config.SetupStep]bool{
		config.SetupStepBasics:    setup != nil && setup.BasicsConfirmedAt != nil,
		config.SetupStepTeam:      facts.StaffInvited,
		config.SetupStepRooms:     facts.RoomCreated,
		config.SetupStepGroups:    facts.GroupCreated,
		config.SetupStepStudents:  facts.StudentEnrolled,
		config.SetupStepGuardians: facts.GuardianInvited,
	}
	result := make([]SchoolSetupStep, 0, len(config.SetupSteps))
	for _, step := range config.SetupSteps {
		result = append(result, SchoolSetupStep{
			Key:     string(step),
			Applies: applies[step],
			Done:    applies[step] && done[step],
			Skipped: applies[step] && setup.Skipped(step),
		})
	}
	return result
}
