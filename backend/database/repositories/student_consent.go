package repositories

import (
	"context"
	"fmt"
	"time"

	auditRepositories "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/uptrace/bun"
)

// This file is the composition seam of the consent story (#3349): People
// Directory folds the child's live timestamps and the recorded trail into the
// shared portal projection, Audit Platform owns audit.student_consent_changes
// and every entry appended to it. The retained services still pass
// users.Student rows around, so the translation lives here.

// NewStudentConsentHistory binds the Audit Platform consent trail behind the
// People Directory projection seam.
func NewStudentConsentHistory(db *bun.DB) peopleCompose.StudentConsentHistory {
	return studentConsentHistory{repo: auditRepositories.NewStudentConsentChangeRepository(auditRootRuntime(db))}
}

type studentConsentHistory struct {
	repo auditModels.StudentConsentChangeRepository
}

// LatestPhotoWithdrawal reports the newest photo consent event only when it is
// a withdrawal; a later grant means the child's consent stands.
func (h studentConsentHistory) LatestPhotoWithdrawal(ctx context.Context, studentID int64) (*time.Time, error) {
	changes, err := h.repo.ListByStudentID(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("student consent history: list changes: %w", err)
	}
	for _, change := range changes {
		if change == nil || change.ConsentKey != auditModels.StudentConsentPhoto {
			continue
		}
		if change.Action != auditModels.StudentConsentWithdrawn {
			return nil, nil
		}
		withdrawnAt := change.CreatedAt
		return &withdrawnAt, nil
	}
	return nil, nil
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
	directory StudentConsentCapability
	trail     auditModels.StudentConsentChangeRepository
}

// NewStudentConsentsFor binds an already composed owner, so the serve root
// keeps one observed People Directory.
func NewStudentConsentsFor(directory StudentConsentCapability, trail auditModels.StudentConsentChangeRepository) *StudentConsents {
	return &StudentConsents{directory: directory, trail: trail}
}

// NewStudentConsents composes unobserved owners for test graphs and CLI roots.
func NewStudentConsents(db *bun.DB) *StudentConsents {
	return NewStudentConsentsFor(
		MustNewPeopleDirectory(db),
		auditRepositories.NewStudentConsentChangeRepository(auditRootRuntime(db)),
	)
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

type studentConsentField struct {
	key           string
	before, after *time.Time
}

// RecordTransitions appends one trail entry per effective change to one of the
// four consent timestamps. A field that did not move records nothing.
func (s *StudentConsents) RecordTransitions(
	ctx context.Context,
	before, after *userModels.Student,
	source string,
	actorAccountID *int64,
	changedAt time.Time,
) error {
	if s.trail == nil {
		return fmt.Errorf("student consent recorder: repository not wired")
	}
	if after == nil || after.ID <= 0 {
		return fmt.Errorf("student consent recorder: persisted student is required")
	}
	for _, field := range consentFields(before, after) {
		if (field.before != nil) == (field.after != nil) {
			continue
		}
		action := auditModels.StudentConsentWithdrawn
		eventTime := changedAt
		if field.after != nil {
			action = auditModels.StudentConsentGranted
			eventTime = *field.after
		}
		entry := &auditModels.StudentConsentChange{
			Model: auditModels.Model{
				CreatedAt: eventTime,
				UpdatedAt: eventTime,
			},
			StudentID:      after.ID,
			ConsentKey:     field.key,
			Action:         action,
			Source:         source,
			ActorAccountID: actorAccountID,
		}
		if err := s.trail.Create(ctx, entry); err != nil {
			return fmt.Errorf("student consent recorder: record %s transition: %w", field.key, err)
		}
	}
	return nil
}

func consentFields(before, after *userModels.Student) []studentConsentField {
	var beforeAGB, beforeDataProcessing, beforeEmail, beforePhoto *time.Time
	if before != nil {
		beforeAGB = before.AGBAcceptedAt
		beforeDataProcessing = before.DataProcessingAcceptedAt
		beforeEmail = before.EmailContactAcceptedAt
		beforePhoto = before.PhotoConsentGivenAt
	}
	return []studentConsentField{
		{key: auditModels.StudentConsentAGB, before: beforeAGB, after: after.AGBAcceptedAt},
		{key: auditModels.StudentConsentDataProcessing, before: beforeDataProcessing, after: after.DataProcessingAcceptedAt},
		{key: auditModels.StudentConsentEmailContact, before: beforeEmail, after: after.EmailContactAcceptedAt},
		{key: auditModels.StudentConsentPhoto, before: beforePhoto, after: after.PhotoConsentGivenAt},
	}
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
