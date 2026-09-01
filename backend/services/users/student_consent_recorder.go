package users

import (
	"context"
	"fmt"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

const (
	StudentConsentStateGranted     = "granted"
	StudentConsentStateWithdrawn   = "withdrawn"
	StudentConsentStateNotRecorded = "not_recorded"
)

// StudentConsentState is the current staff/parent-facing state of one consent
// or required acknowledgement. ChangedAt is the grant/withdrawal time when
// known. CanWithdraw and CanGrant are decided by the calling portal's
// permission check and the recorded state.
type StudentConsentState struct {
	Key         string
	State       string
	ChangedAt   *time.Time
	CanWithdraw bool
	CanGrant    bool
}

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

// StudentConsentStateReader resolves the four live consent timestamps and the
// latest photo withdrawal into one shared projection. Both portals use this so
// staff cannot see a different state than parents.
type StudentConsentStateReader interface {
	CurrentStates(ctx context.Context, student *userModels.Student, canManagePhoto bool) ([]StudentConsentState, error)
}

type StudentConsentService interface {
	StudentConsentChangeRecorder
	StudentConsentStateReader
}

type studentConsentService struct {
	repo auditModels.StudentConsentChangeRepository
}

func NewStudentConsentService(repo auditModels.StudentConsentChangeRepository) StudentConsentService {
	return &studentConsentService{repo: repo}
}

func (r *studentConsentService) CurrentStates(
	ctx context.Context,
	student *userModels.Student,
	canManagePhoto bool,
) ([]StudentConsentState, error) {
	if r == nil || r.repo == nil {
		return nil, fmt.Errorf("student consent reader: repository not wired")
	}
	if student == nil || student.ID <= 0 {
		return nil, fmt.Errorf("student consent reader: persisted student is required")
	}

	var latestPhotoChange *auditModels.StudentConsentChange
	if student.PhotoConsentGivenAt == nil {
		changes, err := r.repo.ListByStudentID(ctx, student.ID)
		if err != nil {
			return nil, fmt.Errorf("student consent reader: list changes: %w", err)
		}
		for _, change := range changes {
			if change.ConsentKey == auditModels.StudentConsentPhoto {
				latestPhotoChange = change
				break
			}
		}
	}

	photo := currentConsentFromTimestamp(
		auditModels.StudentConsentPhoto,
		student.PhotoConsentGivenAt,
		canManagePhoto,
	)
	if latestPhotoChange != nil && latestPhotoChange.Action == auditModels.StudentConsentWithdrawn {
		changedAt := latestPhotoChange.CreatedAt
		photo = StudentConsentState{
			Key:       auditModels.StudentConsentPhoto,
			State:     StudentConsentStateWithdrawn,
			ChangedAt: &changedAt,
			CanGrant:  canManagePhoto,
		}
	}

	return []StudentConsentState{
		currentConsentFromTimestamp(auditModels.StudentConsentAGB, student.AGBAcceptedAt, false),
		currentConsentFromTimestamp(auditModels.StudentConsentDataProcessing, student.DataProcessingAcceptedAt, false),
		currentConsentFromTimestamp(auditModels.StudentConsentEmailContact, student.EmailContactAcceptedAt, false),
		photo,
	}, nil
}

func currentConsentFromTimestamp(key string, recordedAt *time.Time, canWithdraw bool) StudentConsentState {
	if recordedAt == nil {
		return StudentConsentState{Key: key, State: StudentConsentStateNotRecorded}
	}
	return StudentConsentState{
		Key:         key,
		State:       StudentConsentStateGranted,
		ChangedAt:   recordedAt,
		CanWithdraw: canWithdraw,
	}
}

type studentConsentField struct {
	key    string
	before *time.Time
	after  *time.Time
}

func (r *studentConsentService) RecordTransitions(
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
