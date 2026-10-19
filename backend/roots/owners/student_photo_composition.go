package owners

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/realtime"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// ErrPhotoNoTenant refuses a photo route reached without a tenant context.
// Every other outcome is the owner's (modules/peopledirectory).
var ErrPhotoNoTenant = errors.New("no tenant context")

// StudentPhotoConsentRecorder appends the trail entry a photo consent implies.
type StudentPhotoConsentRecorder interface {
	RecordTransitions(
		ctx context.Context,
		before, after *peopleModule.StudentState,
		source string,
		actorAccountID *int64,
		changedAt time.Time,
	) error
}

type StudentPhotoRuntime struct {
	Cleanup     func(context.Context, string) error
	Settings    peoplePorts.PhotoSettings
	Broadcaster PhotoBroadcaster
	Unlinker    peoplePorts.PhotoUnlinker
	Consents    StudentPhotoConsentRecorder
	Logger      *slog.Logger
	Actor       func(context.Context) int64
}

// PhotoBroadcaster is the subset of realtime.Broadcaster the photo lifecycle
// uses to wake open tabs.
type PhotoBroadcaster interface {
	BroadcastToTenant(tenantID int64, event realtime.Event) error
}

func (r StudentPhotoRuntime) PhotoFeatureEnabled(ctx context.Context) (bool, error) {
	if r.Settings == nil {
		return false, nil
	}
	return r.Settings.ResolveBool(ctx, configModel.KeyStudentPhotosEnabled)
}

func (r StudentPhotoRuntime) ActingAccountID(ctx context.Context) int64 {
	if r.Actor == nil {
		return 0
	}
	return r.Actor(ctx)
}

func (r StudentPhotoRuntime) QueueUnlinkStoredPhoto(ctx context.Context, storedURL string) error {
	if r.Cleanup == nil {
		return nil
	}
	return r.Cleanup(ctx, storedURL)
}

func (r StudentPhotoRuntime) UnlinkStoredPhoto(storedURL string) {
	if r.Unlinker != nil {
		r.Unlinker.UnlinkStored(storedURL)
	}
}

func (r StudentPhotoRuntime) BroadcastPhotoChange(tenantID, studentID int64, source string) {
	if r.Broadcaster == nil || tenantID <= 0 {
		if tenantID <= 0 && r.Logger != nil {
			r.Logger.Warn("skipping student_updated broadcast — no tenant context",
				slog.Int64("student_id", studentID),
			)
		}
		return
	}
	label := source
	event := realtime.NewEvent(realtime.EventStudentUpdated, "", realtime.EventData{Source: &label})
	if err := r.Broadcaster.BroadcastToTenant(tenantID, event); err != nil && r.Logger != nil {
		r.Logger.Warn("failed to broadcast student_updated",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("student_id", studentID),
			slog.String("source", source),
			slog.String("error", err.Error()),
		)
	}
}

func (r StudentPhotoRuntime) RecordPhotoConsent(
	ctx context.Context,
	before, after peopleCompose.StudentPhotoConsentSnapshot,
	actorAccountID *int64,
	changedAt time.Time,
) error {
	if r.Consents == nil {
		return nil
	}
	beforeRow := &peopleModule.StudentState{PhotoConsentGivenAt: before.PhotoConsentGivenAt}
	beforeRow.ID = before.StudentID
	afterRow := &peopleModule.StudentState{PhotoConsentGivenAt: after.PhotoConsentGivenAt}
	afterRow.ID = after.StudentID
	return r.Consents.RecordTransitions(
		ctx, beforeRow, afterRow, auditModels.StudentConsentSourceTenantPortal, actorAccountID, changedAt)
}

// StudentPhotos adapts the owner capability to the retained contract the
// handlers still call. It owns no rules: the feature gate, the consent and the
// row lock live with People Directory.
type StudentPhotos struct {
	Directory StudentPhotoCapability
	Runtime   StudentPhotoRuntime
}

// StudentPhotoCapability is the owner surface this seam translates to.
type StudentPhotoCapability interface {
	FindStudentPhoto(ctx context.Context, studentID int64, filename string) (string, error)
	CommitStudentPhoto(ctx context.Context, studentID int64, storedURL string, consentAck bool) error
	ClearStudentPhoto(ctx context.Context, studentID int64) (string, error)
	PurgeStudentPhotos(ctx context.Context) ([]string, error)
	ApplyStudentPhotoConsent(ctx context.Context, current peopleModule.StudentPhotoState, requestedConsent *bool) (peopleModule.StudentPhotoState, error)
	ScheduleStudentPhotoUnlink(ctx context.Context, storedURL string) error
}

func (s StudentPhotos) CommitUpload(ctx context.Context, req peoplePorts.CommitUploadRequest) error {
	if tenant.FromContext(ctx) <= 0 {
		return ErrPhotoNoTenant
	}
	return s.Directory.CommitStudentPhoto(ctx, req.StudentID, req.NewStoredURL, req.ConsentAck)
}

func (s StudentPhotos) CommitDelete(ctx context.Context, studentID int64) (string, error) {
	if tenant.FromContext(ctx) <= 0 {
		return "", ErrPhotoNoTenant
	}
	return s.Directory.ClearStudentPhoto(ctx, studentID)
}

func (s StudentPhotos) LookupForRead(ctx context.Context, studentID int64, filename string) (string, error) {
	if tenant.FromContext(ctx) <= 0 {
		return "", ErrPhotoNoTenant
	}
	return s.Directory.FindStudentPhoto(ctx, studentID, filename)
}

// PurgeAllPhotos detaches every photo inside the caller's transaction and
// returns the after-commit filesystem cleanup.
func (s StudentPhotos) PurgeAllPhotos(ctx context.Context, tenantID int64) (func(), error) {
	urls, err := s.Directory.PurgeStudentPhotos(ctx)
	if err != nil {
		return nil, err
	}
	detached := append([]string(nil), urls...)
	return func() {
		s.afterPhotoFeatureChange(tenantID, detached, peopleModule.StudentPhotoSourceFeatureDisabled)
	}, nil
}

// HandleFeatureToggle runs People Directory's ApplyPhotoFeatureSetting in the
// caller's transaction and returns the after-commit unlink and broadcasts.
func (s StudentPhotos) HandleFeatureToggle(ctx context.Context, tenantID int64, value any) (func(), error) {
	notify := s.deferTenantSettingsChanged(tenantID, configModel.KeyStudentPhotosEnabled)
	change, err := peopleModule.ApplyPhotoFeatureSetting(ctx, s.Directory, value)
	if err != nil {
		return nil, err
	}
	return func() {
		s.afterPhotoFeatureChange(tenantID, change.Purged, change.BroadcastSource)
		notify()
	}, nil
}

// afterPhotoFeatureChange removes the detached files and wakes the open tabs.
func (s StudentPhotos) afterPhotoFeatureChange(tenantID int64, detached []string, source string) {
	for _, url := range detached {
		s.Runtime.UnlinkStoredPhoto(url)
	}
	if len(detached) > 0 && s.Runtime.Logger != nil {
		s.Runtime.Logger.Info("student photos purged after feature disable",
			slog.Int("cleared_count", len(detached)),
		)
	}
	s.Runtime.BroadcastPhotoChange(tenantID, 0, source)
}

// ApplyConsentTransition reconciles the requested consent on the row the
// caller holds; the caller persists it with its own write.
func (s StudentPhotos) ApplyConsentTransition(ctx context.Context, requestedConsent *bool, fresh *peopleModule.StudentState) error {
	if fresh == nil {
		return nil
	}
	current := peopleModule.StudentPhotoState{
		StudentID:           fresh.ID,
		PhotoPath:           fresh.PhotoPath,
		PhotoConsentGivenAt: fresh.PhotoConsentGivenAt,
		PhotoConsentGivenBy: fresh.PhotoConsentGivenBy,
	}
	updated, err := s.Directory.ApplyStudentPhotoConsent(ctx, current, requestedConsent)
	if err != nil {
		return err
	}
	fresh.PhotoPath = updated.PhotoPath
	fresh.PhotoConsentGivenAt = updated.PhotoConsentGivenAt
	fresh.PhotoConsentGivenBy = updated.PhotoConsentGivenBy
	return nil
}

func (s StudentPhotos) ScheduleUnlinkAfterCommit(ctx context.Context, storedURL string) error {
	return s.Directory.ScheduleStudentPhotoUnlink(ctx, storedURL)
}

// deferTenantSettingsChanged returns a closure that broadcasts
// tenant_settings_changed so open tabs re-resolve tenant-scoped state.
func (s StudentPhotos) deferTenantSettingsChanged(tenantID int64, key string) func() {
	return func() {
		if s.Runtime.Broadcaster == nil {
			return
		}
		label := key
		event := realtime.NewEvent(realtime.EventTenantSettingsChanged, "", realtime.EventData{Source: &label})
		if err := s.Runtime.Broadcaster.BroadcastToTenant(tenantID, event); err != nil && s.Runtime.Logger != nil {
			s.Runtime.Logger.Warn("tenant_settings_changed broadcast failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("key", key),
				slog.String("error", err.Error()),
			)
		}
	}
}
