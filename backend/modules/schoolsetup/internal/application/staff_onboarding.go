package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolsetup"
)

// StaffOnboarding implements schoolsetup.StaffOnboardingService: the first
// steps of a care worker (#3748). The progress is the person's own; the
// school-setup progress projection only answers whether the school has a
// group or a child yet.
type StaffOnboarding struct {
	store    schoolsetup.StaffStore
	progress schoolsetup.Progress
	now      func() time.Time
}

var _ schoolsetup.StaffOnboardingService = (*StaffOnboarding)(nil)

// NewStaffOnboarding wires the first steps. Every dependency is required.
func NewStaffOnboarding(store schoolsetup.StaffStore, progress schoolsetup.Progress, now func() time.Time) (*StaffOnboarding, error) {
	if store == nil || progress == nil || now == nil {
		return nil, errors.New("staff onboarding service: all dependencies are required")
	}
	return &StaffOnboarding{store: store, progress: progress, now: now}, nil
}

// StaffStatus returns the first steps of the calling person.
func (s *StaffOnboarding) StaffStatus(ctx context.Context, tenantID, accountID int64) (schoolsetup.StaffStatus, error) {
	state, err := s.store.StaffOnboardingOf(ctx, accountID)
	if err != nil {
		return schoolsetup.StaffStatus{}, err
	}
	status := schoolsetup.StaffStatus{DoneSteps: []string{}, SkippedSteps: []string{}}
	if state != nil {
		status.Dismissed = state.DismissedAt != nil
		status.DoneSteps = known(state.DoneSteps)
		status.SkippedSteps = known(state.SkippedSteps)
	}
	if status.Dismissed {
		// A hidden checklist needs no progress read.
		return status, nil
	}
	facts, err := s.progress.Facts(ctx, tenantID)
	if err != nil {
		return schoolsetup.StaffStatus{}, err
	}
	status.SchoolReady = facts.GroupCreated || facts.StudentEnrolled
	return status, nil
}

// SetStaffStepState marks a step done, skipped or open again.
func (s *StaffOnboarding) SetStaffStepState(ctx context.Context, tenantID, accountID int64, key, stepState string) error {
	step, ok := schoolsetup.ParseStaffStepKey(key)
	if !ok {
		return fmt.Errorf("%w: %q", schoolsetup.ErrUnknownStep, key)
	}
	if !schoolsetup.ValidStaffStepState(stepState) {
		return fmt.Errorf("%w: %q", schoolsetup.ErrUnknownStepState, stepState)
	}
	return s.update(ctx, tenantID, accountID, func(state *schoolsetup.StaffState) {
		state.SetStepState(step, stepState)
	})
}

// SetStaffDismissed hides the checklist for the calling person, or brings it
// back.
func (s *StaffOnboarding) SetStaffDismissed(ctx context.Context, tenantID, accountID int64, dismissed bool) error {
	return s.update(ctx, tenantID, accountID, func(state *schoolsetup.StaffState) {
		if !dismissed {
			state.DismissedAt = nil
			return
		}
		if state.DismissedAt == nil {
			now := s.now()
			state.DismissedAt = &now
		}
	})
}

// update applies change to the person's state and stores it. A person gets
// the state on the first write.
func (s *StaffOnboarding) update(ctx context.Context, tenantID, accountID int64, change func(*schoolsetup.StaffState)) error {
	state, err := s.store.StaffOnboardingOf(ctx, accountID)
	if err != nil {
		return err
	}
	if state == nil {
		state = &schoolsetup.StaffState{
			TenantID:     tenantID,
			AccountID:    accountID,
			DoneSteps:    []string{},
			SkippedSteps: []string{},
		}
	}
	change(state)
	return s.store.StoreStaffOnboarding(ctx, state)
}

// known drops step keys the checklist no longer has, so a retired step never
// reaches the client.
func known(keys []string) []string {
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		if _, ok := schoolsetup.ParseStaffStepKey(key); ok {
			result = append(result, key)
		}
	}
	return result
}
