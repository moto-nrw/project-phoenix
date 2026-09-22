// Package application drives the onboarding wizard for new schools (#2832,
// ADR 0040).
//
// The state (answers, skipped steps, completion, who hid the wizard) lives in
// the wizard's own tables. Progress is not stored: the school-setup-view
// projection reports on every read whether rooms, invitations, groups,
// children and parent invitations exist, and the service derives the steps
// from those facts and the school's settings.
package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolsetup"
)

// Service implements schoolsetup.Service.
type Service struct {
	store    schoolsetup.Store
	progress schoolsetup.Progress
	settings schoolsetup.Settings
	presence schoolsetup.PresenceModeWriter
	now      func() time.Time
}

var _ schoolsetup.Service = (*Service)(nil)

// New wires the wizard. Every dependency is required.
func New(store schoolsetup.Store, progress schoolsetup.Progress, settings schoolsetup.Settings, presence schoolsetup.PresenceModeWriter, now func() time.Time) (*Service, error) {
	if store == nil || progress == nil || settings == nil || presence == nil || now == nil {
		return nil, errors.New("school setup service: all dependencies are required")
	}
	return &Service{store: store, progress: progress, settings: settings, presence: presence, now: now}, nil
}

// Status returns the wizard for the calling person.
func (s *Service) Status(ctx context.Context, tenantID, accountID int64) (schoolsetup.Status, error) {
	state, err := s.store.SetupOfSchool(ctx)
	if err != nil {
		return schoolsetup.Status{}, err
	}
	dismissed, err := s.store.IsDismissed(ctx, accountID)
	if err != nil {
		return schoolsetup.Status{}, err
	}
	basics, err := s.basics(ctx, state)
	if err != nil {
		return schoolsetup.Status{}, err
	}
	status := schoolsetup.Status{
		Completed: state != nil && state.CompletedAt != nil,
		Dismissed: dismissed,
		Basics:    basics,
	}
	if status.Completed {
		// A finished school needs no progress read.
		status.Steps = []schoolsetup.Step{}
		return status, nil
	}
	facts, err := s.progress.Facts(ctx, tenantID)
	if err != nil {
		return schoolsetup.Status{}, err
	}
	status.Steps = steps(state, basics, facts)
	return status, nil
}

// ConfirmBasics stores the answers of the first step that are not ordinary
// admin settings: the presence mode, which admins may set only during setup,
// and whether the school uses the parent app. Group mode and care plan go
// through the regular settings API before this call. The step may be
// confirmed again while setup is open.
func (s *Service) ConfirmBasics(ctx context.Context, tenantID, accountID int64, presenceMode string, parentAppUsed bool) error {
	if presenceMode != schoolsetup.PresenceModeDetailed && presenceMode != schoolsetup.PresenceModeBinary {
		return schoolsetup.ErrInvalidPresenceMode
	}
	state, err := s.openState(ctx, tenantID)
	if err != nil {
		return err
	}
	current, err := s.settings.PresenceMode(ctx)
	if err != nil {
		return fmt.Errorf("resolve presence mode: %w", err)
	}
	if current != presenceMode {
		if err := s.presence(ctx, tenantID, accountID, presenceMode); err != nil {
			return err
		}
	}
	now := s.now()
	state.ParentAppUsed = &parentAppUsed
	state.BasicsConfirmedAt = &now
	state.UpdatedBy = &accountID
	return s.store.StoreSetup(ctx, state)
}

// SetStepSkipped skips a step or takes the skip back. The first step cannot
// be skipped: the others depend on its answers.
func (s *Service) SetStepSkipped(ctx context.Context, tenantID, accountID int64, key string, skipped bool) error {
	step, ok := schoolsetup.ParseStepKey(key)
	if !ok {
		return fmt.Errorf("%w: %q", schoolsetup.ErrUnknownStep, key)
	}
	if step == schoolsetup.StepBasics {
		return schoolsetup.ErrStepNotSkippable
	}
	state, err := s.openState(ctx, tenantID)
	if err != nil {
		return err
	}
	state.SetSkipped(step, skipped)
	state.UpdatedBy = &accountID
	return s.store.StoreSetup(ctx, state)
}

// Complete finishes setup for the whole school once every applicable step is
// done or skipped. Afterwards the wizard no longer opens and the presence mode
// is operator-only again.
func (s *Service) Complete(ctx context.Context, tenantID, accountID int64) error {
	status, err := s.Status(ctx, tenantID, accountID)
	if err != nil {
		return err
	}
	if status.Completed {
		return schoolsetup.ErrCompleted
	}
	for _, step := range status.Steps {
		if step.Applies && !step.Done && !step.Skipped {
			return schoolsetup.ErrIncomplete
		}
	}
	state, err := s.openState(ctx, tenantID)
	if err != nil {
		return err
	}
	now := s.now()
	state.CompletedAt = &now
	state.UpdatedBy = &accountID
	return s.store.StoreSetup(ctx, state)
}

// SetDismissed hides the wizard for the calling person, or brings it back.
func (s *Service) SetDismissed(ctx context.Context, tenantID, accountID int64, dismissed bool) error {
	return s.store.SetDismissed(ctx, tenantID, accountID, dismissed)
}

// openState returns the school's state for a write, creating it for a new
// school, and refuses once setup is completed.
func (s *Service) openState(ctx context.Context, tenantID int64) (*schoolsetup.State, error) {
	state, err := s.store.SetupOfSchool(ctx)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return &schoolsetup.State{TenantID: tenantID, SkippedSteps: []string{}}, nil
	}
	if state.CompletedAt != nil {
		return nil, schoolsetup.ErrCompleted
	}
	return state, nil
}

func (s *Service) basics(ctx context.Context, state *schoolsetup.State) (schoolsetup.Basics, error) {
	presenceMode, err := s.settings.PresenceMode(ctx)
	if err != nil {
		return schoolsetup.Basics{}, fmt.Errorf("resolve presence mode: %w", err)
	}
	groupMode, err := s.settings.GroupMode(ctx)
	if err != nil {
		return schoolsetup.Basics{}, fmt.Errorf("resolve group mode: %w", err)
	}
	timetableEnabled, err := s.settings.TimetableEnabled(ctx)
	if err != nil {
		return schoolsetup.Basics{}, fmt.Errorf("resolve care plan toggle: %w", err)
	}
	basics := schoolsetup.Basics{
		PresenceMode:     presenceMode,
		GroupMode:        groupMode,
		TimetableEnabled: timetableEnabled,
	}
	if state != nil {
		basics.ParentAppUsed = state.ParentAppUsed
	}
	return basics, nil
}

// steps derives the wizard steps. A step that does not apply is never done or
// skipped from the wizard's point of view. The parent step applies until the
// school answered "no": an unanswered question must not hide a step.
func steps(state *schoolsetup.State, basics schoolsetup.Basics, facts schoolsetup.Facts) []schoolsetup.Step {
	applies := map[schoolsetup.StepKey]bool{
		schoolsetup.StepBasics:    true,
		schoolsetup.StepTeam:      true,
		schoolsetup.StepRooms:     basics.PresenceMode == schoolsetup.PresenceModeDetailed,
		schoolsetup.StepGroups:    basics.GroupMode == schoolsetup.GroupModeFixedGroups,
		schoolsetup.StepStudents:  true,
		schoolsetup.StepGuardians: basics.ParentAppUsed == nil || *basics.ParentAppUsed,
	}
	done := map[schoolsetup.StepKey]bool{
		schoolsetup.StepBasics:    state != nil && state.BasicsConfirmedAt != nil,
		schoolsetup.StepTeam:      facts.StaffInvited,
		schoolsetup.StepRooms:     facts.RoomCreated,
		schoolsetup.StepGroups:    facts.GroupCreated,
		schoolsetup.StepStudents:  facts.StudentEnrolled,
		schoolsetup.StepGuardians: facts.GuardianInvited,
	}
	result := make([]schoolsetup.Step, 0, len(schoolsetup.StepKeys))
	for _, step := range schoolsetup.StepKeys {
		result = append(result, schoolsetup.Step{
			Key:     string(step),
			Applies: applies[step],
			Done:    applies[step] && done[step],
			Skipped: applies[step] && state.Skipped(step),
		})
	}
	return result
}
