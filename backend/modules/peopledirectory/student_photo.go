package peopledirectory

import (
	"context"
	"errors"
	"time"
)

// Sentinel outcomes of the student-photo lifecycle. The HTTP adapter maps them
// to status codes; the owner decides which of them a request produces.
var (
	ErrPhotoFeatureDisabled    = errors.New("photo feature disabled")
	ErrPhotoFeatureDisabledMid = errors.New("photo feature disabled mid-operation")
	ErrPhotoStudentForbidden   = errors.New("student photo access denied")
	ErrPhotoConsentRequired    = errors.New("photo consent required")
	ErrPhotoConsentWithdrawn   = errors.New("photo consent withdrawn mid-operation")
	ErrPhotoFilenameMismatch   = errors.New("photo filename mismatch")
	ErrPhotoNotSet             = errors.New("no photo set for this student")
	ErrPhotoNoTenant           = errors.New("no tenant context")
)

// StudentPhotoState is the photo slice of a child's row: where the image is
// stored and whether the voluntary consent behind it is on record.
type StudentPhotoState struct {
	StudentID           int64
	PhotoPath           *string
	PhotoConsentGivenAt *time.Time
	PhotoConsentGivenBy *int64
}

// StudentPhotoQuery serves the stored photo of a child.
type StudentPhotoQuery interface {
	// FindStudentPhoto returns the stored URL behind filename, refusing a
	// name that does not belong to this child. The access check runs first,
	// so a denied caller cannot probe which filenames exist.
	FindStudentPhoto(ctx context.Context, studentID int64, filename string) (string, error)
}

// StudentPhotoCommand changes it. Every command validates the tenant's photo
// feature, the caller's access and the recorded consent under the per-tenant
// photo lock, so a disable, a reassignment or a withdrawal committing halfway
// cannot leave an orphaned image behind.
type StudentPhotoCommand interface {
	// CommitStudentPhoto attaches an already stored upload. consentAck is
	// the caller's confirmation that the guardian agreed, required when the
	// child carries no recorded photo consent yet.
	CommitStudentPhoto(ctx context.Context, studentID int64, storedURL string, consentAck bool) error
	// ClearStudentPhoto detaches the stored photo and returns its URL, empty
	// when the child had none.
	ClearStudentPhoto(ctx context.Context, studentID int64) (string, error)
	// PurgeStudentPhotos detaches every photo of the tenant and returns the
	// URLs whose files the caller removes after the commit.
	PurgeStudentPhotos(ctx context.Context) ([]string, error)
	// ApplyStudentPhotoConsent reconciles a requested consent against the row
	// the caller holds and returns the photo slice it must write: a grant
	// stamps it, a withdrawal clears the consent and the photo together, and
	// a repeated grant keeps the original timestamp. The file a withdrawal
	// detaches is removed after the caller's commit.
	ApplyStudentPhotoConsent(ctx context.Context, current StudentPhotoState, requestedConsent *bool) StudentPhotoState
	// ScheduleStudentPhotoUnlink removes a detached file after the caller's
	// transaction commits; an empty URL is a no-op.
	ScheduleStudentPhotoUnlink(ctx context.Context, storedURL string)
}

func (m *Module) FindStudentPhoto(ctx context.Context, studentID int64, filename string) (string, error) {
	if studentID <= 0 {
		return "", invalidStudent("student ID is required")
	}
	return m.engine.FindStudentPhoto(ctx, studentID, filename)
}

func (m *Module) CommitStudentPhoto(ctx context.Context, studentID int64, storedURL string, consentAck bool) error {
	if studentID <= 0 {
		return invalidStudent("student ID is required")
	}
	if storedURL == "" {
		return invalidStudent("a stored photo URL is required")
	}
	return m.engine.CommitStudentPhoto(ctx, studentID, storedURL, consentAck)
}

func (m *Module) ClearStudentPhoto(ctx context.Context, studentID int64) (string, error) {
	if studentID <= 0 {
		return "", invalidStudent("student ID is required")
	}
	return m.engine.ClearStudentPhoto(ctx, studentID)
}

func (m *Module) PurgeStudentPhotos(ctx context.Context) ([]string, error) {
	return m.engine.PurgeStudentPhotos(ctx)
}

func (m *Module) ApplyStudentPhotoConsent(
	ctx context.Context,
	current StudentPhotoState,
	requestedConsent *bool,
) StudentPhotoState {
	return m.engine.ApplyStudentPhotoConsent(ctx, current, requestedConsent)
}

func (m *Module) ScheduleStudentPhotoUnlink(ctx context.Context, storedURL string) {
	m.engine.ScheduleStudentPhotoUnlink(ctx, storedURL)
}
