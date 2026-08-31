package users

import (
	"context"
	"fmt"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// StudentConsentChangeRecorder appends audit rows for effective changes to
// the four consent timestamps stored on users.students.
type StudentConsentChangeRecorder interface {
	RecordTransitions(
		ctx context.Context,
		before, after *userModels.Student,
		source string,
		actorAccountID *int64,
		changedAt time.Time,
	) error
}

type studentConsentRecorder struct {
	repo auditModels.StudentConsentChangeRepository
}

func NewStudentConsentRecorder(repo auditModels.StudentConsentChangeRepository) StudentConsentChangeRecorder {
	return &studentConsentRecorder{repo: repo}
}

type studentConsentField struct {
	key    string
	before *time.Time
	after  *time.Time
}

func (r *studentConsentRecorder) RecordTransitions(
	ctx context.Context,
	before, after *userModels.Student,
	source string,
	actorAccountID *int64,
	changedAt time.Time,
) error {
	if r == nil || r.repo == nil {
		return fmt.Errorf("student consent recorder: repository not wired")
	}
	if after == nil || after.ID <= 0 {
		return fmt.Errorf("student consent recorder: persisted student is required")
	}

	fields := consentFields(before, after)
	for _, field := range fields {
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
			Model: base.Model{
				CreatedAt: eventTime,
				UpdatedAt: eventTime,
			},
			StudentID:      after.ID,
			ConsentKey:     field.key,
			Action:         action,
			Source:         source,
			ActorAccountID: actorAccountID,
		}
		if err := r.repo.Create(ctx, entry); err != nil {
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
