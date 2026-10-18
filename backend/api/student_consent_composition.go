package api

import (
	"context"
	"fmt"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
)

// This file is the composition seam of the consent story (#3349): People
// Directory folds the child's live timestamps and the recorded trail into the
// shared portal projection, Audit Platform owns audit.student_consent_changes
// and every entry appended to it. The retained services still pass
// users.Student rows around, so the translation lives here.

type studentConsentHistory struct {
	repo interface {
		ListByStudentID(ctx context.Context, studentID int64) ([]*auditModels.StudentConsentChange, error)
	}
}

// LatestPhotoWithdrawal reads the trail newest first and lets the People
// Directory decide whether the photo consent stands withdrawn.
func (h studentConsentHistory) LatestPhotoWithdrawal(ctx context.Context, studentID int64) (*time.Time, error) {
	changes, err := h.repo.ListByStudentID(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("student consent history: list changes: %w", err)
	}
	events := make([]peopleModule.StudentConsentEvent, 0, len(changes))
	for _, change := range changes {
		if change != nil {
			events = append(events, peopleModule.StudentConsentEvent{
				Key: change.ConsentKey, Withdrawn: change.Action == auditModels.StudentConsentWithdrawn, At: change.CreatedAt,
			})
		}
	}
	return peopleModule.PhotoConsentWithdrawnAt(events), nil
}

// StudentConsentCapability is the People Directory surface behind the shared
// consent projection.
type StudentConsentCapability interface {
	peopleModule.StudentConsentQuery
}

// StudentConsents adapts both owners to the retained model-typed contract the
// legacy services still call, satisfied structurally so this seam does not
// depend on them.
type StudentConsents struct {
	studentConsentTrail
	directory StudentConsentCapability
}

// studentConsentTrail records the consent transitions in the Audit Platform
// trail; it needs no People Directory read, so the photo lifecycle records
// through it before the directory exists.
type studentConsentTrail struct {
	trail StudentConsentAppender
}

// NewStudentConsentsFor binds an already composed owner, so the serve root
// keeps one observed People Directory.
func NewStudentConsentsFor(directory StudentConsentCapability, trail StudentConsentAppender) *StudentConsents {
	return &StudentConsents{studentConsentTrail: studentConsentTrail{trail: trail}, directory: directory}
}

// CurrentStates resolves the four consent states of a student row the caller
// already read.
func (s *StudentConsents) CurrentStates(
	ctx context.Context,
	student *userModels.Student,
	canManagePhoto bool,
) ([]userModels.StudentConsentState, error) {
	if student == nil || student.ID <= 0 {
		return nil, fmt.Errorf("student consent reader: persisted student is required")
	}
	states, err := s.directory.CurrentStudentConsents(ctx, studentConsentSnapshot(student), canManagePhoto)
	if err != nil {
		return nil, err
	}
	result := make([]userModels.StudentConsentState, 0, len(states))
	for _, state := range states {
		result = append(result, userModels.StudentConsentState{
			Key: state.Key, State: state.State, ChangedAt: state.ChangedAt,
			CanWithdraw: state.CanWithdraw, CanGrant: state.CanGrant,
		})
	}
	return result, nil
}

// RecordTransitions appends one trail entry per effective change to one of the
// four consent timestamps. A field that did not move records nothing.
func (s studentConsentTrail) RecordTransitions(
	ctx context.Context,
	before, after *userModels.Student,
	source string,
	actorAccountID *int64,
	changedAt time.Time,
) error {
	if after == nil || after.ID <= 0 {
		if s.trail == nil {
			return fmt.Errorf("student consent recorder: repository not wired")
		}
		return fmt.Errorf("student consent recorder: persisted student is required")
	}
	var beforeSnapshot peopleModule.StudentConsentSnapshot
	if before != nil {
		beforeSnapshot = studentConsentSnapshot(before)
	}
	return s.RecordStudentConsentTransitions(ctx, beforeSnapshot, studentConsentSnapshot(after), source, actorAccountID, changedAt)
}

// CurrentStudentConsents resolves the four consent states of a child the
// caller already read, in the owner's projection.
func (s *StudentConsents) CurrentStudentConsents(
	ctx context.Context,
	snapshot peopleModule.StudentConsentSnapshot,
	canManagePhoto bool,
) ([]peopleModule.StudentConsentState, error) {
	if snapshot.StudentID <= 0 {
		return nil, fmt.Errorf("student consent reader: persisted student is required")
	}
	return s.directory.CurrentStudentConsents(ctx, snapshot, canManagePhoto)
}

// RecordStudentConsentTransitions is RecordTransitions over the owner's
// consent snapshots: one trail entry per transition the People Directory
// reads from them.
func (s studentConsentTrail) RecordStudentConsentTransitions(
	ctx context.Context,
	before, after peopleModule.StudentConsentSnapshot,
	source string,
	actorAccountID *int64,
	changedAt time.Time,
) error {
	if s.trail == nil {
		return fmt.Errorf("student consent recorder: repository not wired")
	}
	if after.StudentID <= 0 {
		return fmt.Errorf("student consent recorder: persisted student is required")
	}
	for _, transition := range peopleModule.StudentConsentTransitions(before, after, changedAt) {
		action := auditModels.StudentConsentWithdrawn
		if transition.Granted {
			action = auditModels.StudentConsentGranted
		}
		// The People Directory's consent keys are the trail's discriminators.
		entry := &auditModels.StudentConsentChange{
			Model: auditModels.Model{
				CreatedAt: transition.At,
				UpdatedAt: transition.At,
			},
			StudentID:      after.StudentID,
			ConsentKey:     transition.Key,
			Action:         action,
			Source:         source,
			ActorAccountID: actorAccountID,
		}
		if err := s.trail.Create(ctx, entry); err != nil {
			return fmt.Errorf("student consent recorder: record %s transition: %w", transition.Key, err)
		}
	}
	return nil
}

func studentConsentSnapshot(student *userModels.Student) peopleModule.StudentConsentSnapshot {
	return peopleModule.StudentConsentSnapshot{
		StudentID:                student.ID,
		AGBAcceptedAt:            student.AGBAcceptedAt,
		DataProcessingAcceptedAt: student.DataProcessingAcceptedAt,
		EmailContactAcceptedAt:   student.EmailContactAcceptedAt,
		PhotoConsentGivenAt:      student.PhotoConsentGivenAt,
	}
}
