// Package application drives the onboarding wizard for new schools (#2832,
// ADR 0040).
//
// The state (skipped steps, completion, who hid the wizard) lives in the
// wizard's own tables. Progress is not stored: the school-setup-view
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
	now      func() time.Time
}

var _ schoolsetup.Service = (*Service)(nil)

// New wires the wizard. Every dependency is required.
func New(store schoolsetup.Store, progress schoolsetup.Progress, settings schoolsetup.Settings, now func() time.Time) (*Service, error) {
	if store == nil || progress == nil || settings == nil || now == nil {
		return nil, errors.New("school setup service: all dependencies are required")
	}
	return &Service{store: store, progress: progress, settings: settings, now: now}, nil
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
	basics, err := s.basics(ctx)
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

// SetStepSkipped skips a step or takes the skip back.
func (s *Service) SetStepSkipped(ctx context.Context, tenantID, accountID int64, key string, skipped bool) error {
	step, ok := schoolsetup.ParseStepKey(key)
	if !ok {
		return fmt.Errorf("%w: %q", schoolsetup.ErrUnknownStep, key)
	}
	return s.update(ctx, tenantID, accountID, func(state *schoolsetup.State) {
		state.SetSkipped(step, skipped)
	})
}

// Complete finishes setup for the whole school once every applicable step is
// done or skipped. Afterwards the wizard no longer opens.
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
	now := s.now()
	return s.update(ctx, tenantID, accountID, func(state *schoolsetup.State) {
		state.CompletedAt = &now
	})
}

// SetDismissed hides the wizard for the calling person, or brings it back.
func (s *Service) SetDismissed(ctx context.Context, tenantID, accountID int64, dismissed bool) error {
	return s.store.SetDismissed(ctx, tenantID, accountID, dismissed)
}

// update applies change to the school's state and stores it. A new school
// gets its state on the first write; a completed school refuses every write.
func (s *Service) update(ctx context.Context, tenantID, accountID int64, change func(*schoolsetup.State)) error {
	state, err := s.store.SetupOfSchool(ctx)
	if err != nil {
		return err
	}
	if state == nil {
		state = &schoolsetup.State{TenantID: tenantID, SkippedSteps: []string{}}
	}
	if state.CompletedAt != nil {
		return schoolsetup.ErrCompleted
	}
	change(state)
	state.UpdatedBy = &accountID
	return s.store.StoreSetup(ctx, state)
}

func (s *Service) basics(ctx context.Context) (schoolsetup.Basics, error) {
	presenceMode, err := s.settings.PresenceMode(ctx)
	if err != nil {
		return schoolsetup.Basics{}, fmt.Errorf("resolve presence mode: %w", err)
	}
	groupMode, err := s.settings.GroupMode(ctx)
	if err != nil {
		return schoolsetup.Basics{}, fmt.Errorf("resolve group mode: %w", err)
	}
	return schoolsetup.Basics{PresenceMode: presenceMode, GroupMode: groupMode}, nil
}

// steps derives the wizard steps. A step that does not apply is never done or
// skipped from the wizard's point of view.
func steps(state *schoolsetup.State, basics schoolsetup.Basics, facts schoolsetup.Facts) []schoolsetup.Step {
	applies := map[schoolsetup.StepKey]bool{
		schoolsetup.StepTeam:      true,
		schoolsetup.StepRooms:     basics.PresenceMode == schoolsetup.PresenceModeDetailed,
		schoolsetup.StepGroups:    basics.GroupMode == schoolsetup.GroupModeFixedGroups,
		schoolsetup.StepStudents:  true,
		schoolsetup.StepGuardians: true,
	}
	done := map[schoolsetup.StepKey]bool{
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
