package services

import (
	"context"
	"log/slog"
	"time"

	jwtPkg "github.com/moto-nrw/project-phoenix/auth/jwt"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// This file binds the five surfaces the People Directory photo lifecycle needs
// beyond its own rows (#3349): the tenant feature flag, the caller's access,
// the acting account, the stored-file cleanup with the live refresh, and the
// Audit Platform consent trail. They only exist once the HTTP layer is up,
// which is why the owner takes them through a late binder.

// StudentPhotoConsentRecorder appends the trail entry a photo consent implies.
type StudentPhotoConsentRecorder interface {
	RecordTransitions(
		ctx context.Context,
		before, after *userModels.Student,
		source string,
		actorAccountID *int64,
		changedAt time.Time,
	) error
}

type studentPhotoRuntime struct {
	settings    users.PhotoSettings
	broadcaster PhotoBroadcaster
	unlinker    users.PhotoUnlinker
	consents    StudentPhotoConsentRecorder
	logger      *slog.Logger
}

// PhotoBroadcaster is the subset of realtime.Broadcaster the photo lifecycle
// uses to wake open tabs.
type PhotoBroadcaster interface {
	BroadcastToTenant(tenantID int64, event realtime.Event) error
}

func (r studentPhotoRuntime) PhotoFeatureEnabled(ctx context.Context) (bool, error) {
	if r.settings == nil {
		return false, nil
	}
	return r.settings.ResolveBool(ctx, configModel.KeyStudentPhotosEnabled)
}

func (r studentPhotoRuntime) ActingAccountID(ctx context.Context) int64 {
	return int64(jwtPkg.ClaimsFromCtx(ctx).ID)
}

func (r studentPhotoRuntime) UnlinkStoredPhoto(storedURL string) {
	if r.unlinker != nil {
		r.unlinker.UnlinkStored(storedURL)
	}
}

func (r studentPhotoRuntime) BroadcastPhotoChange(tenantID, studentID int64, source string) {
	if r.broadcaster == nil || tenantID <= 0 {
		if tenantID <= 0 && r.logger != nil {
			r.logger.Warn("skipping student_updated broadcast — no tenant context",
				slog.Int64("student_id", studentID),
			)
		}
		return
	}
	label := source
	event := realtime.NewEvent(realtime.EventStudentUpdated, "", realtime.EventData{Source: &label})
	if err := r.broadcaster.BroadcastToTenant(tenantID, event); err != nil && r.logger != nil {
		r.logger.Warn("failed to broadcast student_updated",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("student_id", studentID),
			slog.String("source", source),
			slog.String("error", err.Error()),
		)
	}
}

func (r studentPhotoRuntime) RecordPhotoConsent(
	ctx context.Context,
	before, after peopleCompose.StudentPhotoConsentSnapshot,
	actorAccountID *int64,
	changedAt time.Time,
) error {
	if r.consents == nil {
		return nil
	}
	beforeRow := &userModels.Student{PhotoConsentGivenAt: before.PhotoConsentGivenAt}
	beforeRow.ID = before.StudentID
	afterRow := &userModels.Student{PhotoConsentGivenAt: after.PhotoConsentGivenAt}
	afterRow.ID = after.StudentID
	return r.consents.RecordTransitions(
		ctx, beforeRow, afterRow, auditModels.StudentConsentSourceTenantPortal, actorAccountID, changedAt)
}

// studentPhotos adapts the owner capability to the retained contract the
// handlers still call. It owns no rules: the feature gate, the consent and the
// row lock live with People Directory.
type studentPhotos struct {
	directory StudentPhotoCapability
	runtime   studentPhotoRuntime
}

// StudentPhotoCapability is the owner surface this seam translates to.
type StudentPhotoCapability interface {
	FindStudentPhoto(ctx context.Context, studentID int64, filename string) (string, error)
	CommitStudentPhoto(ctx context.Context, studentID int64, storedURL string, consentAck bool) error
	ClearStudentPhoto(ctx context.Context, studentID int64) (string, error)
	PurgeStudentPhotos(ctx context.Context) ([]string, error)
	ApplyStudentPhotoConsent(ctx context.Context, current peopleModule.StudentPhotoState, requestedConsent *bool) peopleModule.StudentPhotoState
	ScheduleStudentPhotoUnlink(ctx context.Context, storedURL string)
}

func (s studentPhotos) CommitUpload(ctx context.Context, req users.CommitUploadRequest) error {
	if tenant.FromContext(ctx) <= 0 {
		return users.ErrPhotoNoTenant
	}
	return s.directory.CommitStudentPhoto(ctx, req.StudentID, req.NewStoredURL, req.ConsentAck)
}

func (s studentPhotos) CommitDelete(ctx context.Context, studentID int64) (string, error) {
	if tenant.FromContext(ctx) <= 0 {
		return "", users.ErrPhotoNoTenant
	}
	return s.directory.ClearStudentPhoto(ctx, studentID)
}

func (s studentPhotos) LookupForRead(ctx context.Context, studentID int64, filename string) (string, error) {
	if tenant.FromContext(ctx) <= 0 {
		return "", users.ErrPhotoNoTenant
	}
	return s.directory.FindStudentPhoto(ctx, studentID, filename)
}

// PurgeAllPhotos detaches every photo inside the caller's transaction and
// returns the after-commit filesystem cleanup.
func (s studentPhotos) PurgeAllPhotos(ctx context.Context, tenantID int64) (func(), error) {
	urls, err := s.directory.PurgeStudentPhotos(ctx)
	if err != nil {
		return nil, err
	}
	detached := append([]string(nil), urls...)
	return func() {
		for _, url := range detached {
			s.runtime.UnlinkStoredPhoto(url)
		}
		if len(detached) > 0 && s.runtime.logger != nil {
			s.runtime.logger.Info("student photos purged after feature disable",
				slog.Int("cleared_count", len(detached)),
			)
		}
		s.runtime.BroadcastPhotoChange(tenantID, 0, photoSourcePurgeOnDisable)
	}, nil
}

func (s studentPhotos) HandleFeatureToggle(ctx context.Context, tenantID int64, value any) (func(), error) {
	notify := s.deferTenantSettingsChanged(tenantID, configModel.KeyStudentPhotosEnabled)
	enabled, ok := value.(bool)
	if !ok || enabled {
		return func() {
			s.runtime.BroadcastPhotoChange(tenantID, 0, photoSourceFeatureEnabled)
			notify()
		}, nil
	}
	purge, err := s.PurgeAllPhotos(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return func() {
		if purge != nil {
			purge()
		}
		notify()
	}, nil
}

// ApplyConsentTransition reconciles the requested consent on the row the
// caller holds; the caller persists it with its own write.
func (s studentPhotos) ApplyConsentTransition(ctx context.Context, requestedConsent *bool, fresh *userModels.Student) {
	if fresh == nil {
		return
	}
	current := peopleModule.StudentPhotoState{
		StudentID:           fresh.ID,
		PhotoPath:           fresh.PhotoPath,
		PhotoConsentGivenAt: fresh.PhotoConsentGivenAt,
		PhotoConsentGivenBy: fresh.PhotoConsentGivenBy,
	}
	updated := s.directory.ApplyStudentPhotoConsent(ctx, current, requestedConsent)
	fresh.PhotoPath = updated.PhotoPath
	fresh.PhotoConsentGivenAt = updated.PhotoConsentGivenAt
	fresh.PhotoConsentGivenBy = updated.PhotoConsentGivenBy
}

func (s studentPhotos) ScheduleUnlinkAfterCommit(ctx context.Context, storedURL string) {
	s.directory.ScheduleStudentPhotoUnlink(ctx, storedURL)
}

// deferTenantSettingsChanged returns a closure that broadcasts
// tenant_settings_changed so open tabs re-resolve tenant-scoped state.
func (s studentPhotos) deferTenantSettingsChanged(tenantID int64, key string) func() {
	return func() {
		if s.runtime.broadcaster == nil {
			return
		}
		label := key
		event := realtime.NewEvent(realtime.EventTenantSettingsChanged, "", realtime.EventData{Source: &label})
		if err := s.runtime.broadcaster.BroadcastToTenant(tenantID, event); err != nil && s.runtime.logger != nil {
			s.runtime.logger.Warn("tenant_settings_changed broadcast failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("key", key),
				slog.String("error", err.Error()),
			)
		}
	}
}

// Broadcast source labels for the two tenant-wide photo events.
const (
	photoSourcePurgeOnDisable = "tenant_photos_disabled"
	photoSourceFeatureEnabled = "tenant_photos_enabled"
)

// NewStudentPhotos binds the owner capability and the runtime it needs, and
// returns the retained contract the handlers call.
func NewStudentPhotos(
	directory StudentPhotoCapability,
	slot *peopleCompose.StudentPhotoRuntime,
	deps StudentPhotoRuntimeDependencies,
) users.StudentPhotoService {
	runtime := studentPhotoRuntime{
		settings: deps.Settings, broadcaster: deps.Broadcaster,
		unlinker: deps.Unlinker, consents: deps.Consents, logger: deps.Logger,
	}
	if slot != nil {
		*slot = runtime
	}
	return studentPhotos{directory: directory, runtime: runtime}
}

// StudentPhotoRuntimeDependencies are the surfaces the photo lifecycle needs
// from the other owners.
type StudentPhotoRuntimeDependencies struct {
	Settings    users.PhotoSettings
	Broadcaster PhotoBroadcaster
	Unlinker    users.PhotoUnlinker
	Consents    StudentPhotoConsentRecorder
	Logger      *slog.Logger
}
