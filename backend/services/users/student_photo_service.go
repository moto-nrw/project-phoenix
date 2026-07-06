package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// PhotoUnlinker deletes a previously-stored photo file.
type PhotoUnlinker interface {
	UnlinkStored(storedURL string)
}

// PhotoBroadcaster is the subset of realtime.Broadcaster used here.
type PhotoBroadcaster interface {
	BroadcastToTenant(tenantID int64, event realtime.Event) error
}

// PhotoSettings is the narrow settings surface used here.
type PhotoSettings interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
}

// PhotoUserContext combines the read- and modify-side gates.
type PhotoUserContext interface {
	authorize.StudentReadUserContext
	authorize.StudentModifyUserContext
}

// CommitUploadRequest carries an already-stored upload into the commit path.
type CommitUploadRequest struct {
	StudentID    int64
	NewStoredURL string
	ConsentAck   bool
}

// StudentPhotoService owns the tenant-scoped student-photo lifecycle.
type StudentPhotoService interface {
	CommitUpload(ctx context.Context, req CommitUploadRequest) error
	CommitDelete(ctx context.Context, studentID int64) (clearedURL string, err error)
	LookupForRead(ctx context.Context, studentID int64, filename string) (storedURL string, err error)
	PurgeAllPhotos(ctx context.Context, tenantID int64) (postCommit func(), err error)
	HandleFeatureToggle(ctx context.Context, tenantID int64, value any) (postCommit func(), err error)
	// ApplyConsentTransition mutates fresh in place; caller must Update it.
	ApplyConsentTransition(ctx context.Context, requestedConsent *bool, fresh *userModels.Student)
	ScheduleUnlinkAfterCommit(ctx context.Context, storedURL string)
}

// Sentinel errors mapped to HTTP status by the handlers.
var (
	ErrPhotoFeatureDisabled    = errors.New("photo feature disabled")
	ErrPhotoFeatureDisabledMid = errors.New("photo feature disabled mid-operation")
	ErrPhotoStudentNotFound    = errors.New("student not found")
	ErrPhotoStudentForbidden   = errors.New("student photo access denied")
	ErrPhotoStudentReassigned  = errors.New("student reassigned out of caller's scope")
	ErrPhotoConsentRequired    = errors.New("photo consent required")
	ErrPhotoConsentWithdrawn   = errors.New("photo consent withdrawn mid-operation")
	ErrPhotoFilenameMismatch   = errors.New("photo filename mismatch")
	ErrPhotoNotSet             = errors.New("no photo set for this student")
	ErrPhotoNoTenant           = errors.New("no tenant context")
)

// Broadcast source labels for SSE event payloads.
const (
	photoSourceUploaded       = "photo_uploaded"
	photoSourceDeleted        = "photo_deleted"
	photoSourcePurgeOnDisable = "tenant_photos_disabled"
	photoSourceFeatureEnabled = "tenant_photos_enabled"
)

// StudentPhotoServiceDependencies aggregates the constructor arguments.
type StudentPhotoServiceDependencies struct {
	StudentRepo userModels.StudentRepository
	Settings    PhotoSettings
	UserContext PhotoUserContext
	Broadcaster PhotoBroadcaster
	Unlinker    PhotoUnlinker
	DB          *bun.DB
	Logger      *slog.Logger
}

type studentPhotoService struct {
	StudentPhotoServiceDependencies
}

func NewStudentPhotoService(deps StudentPhotoServiceDependencies) StudentPhotoService {
	return &studentPhotoService{StudentPhotoServiceDependencies: deps}
}

// CommitUpload validates feature/consent/access then swaps photo_path.
// Caller removes req.NewStoredURL on error.
func (s *studentPhotoService) CommitUpload(ctx context.Context, req CommitUploadRequest) error {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return ErrPhotoNoTenant
	}

	return tenant.WithTenantTx(ctx, s.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := s.requireFeatureEnabled(ctx); err != nil {
			return err
		}
		student, err := s.StudentRepo.FindByID(ctx, req.StudentID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrPhotoStudentNotFound
			}
			return err
		}
		if ok, _ := authorize.CanUpdateStudent(ctx, jwt.PermissionsFromCtx(ctx), student, s.UserContext); !ok {
			return ErrPhotoStudentForbidden
		}
		if student.PhotoConsentGivenAt == nil && !req.ConsentAck {
			return ErrPhotoConsentRequired
		}

		// Serialize against feature-disable purge.
		if err := s.StudentRepo.LockPhotoFeature(ctx); err != nil {
			return err
		}
		if err := s.requireFeatureEnabled(ctx); err != nil {
			return ErrPhotoFeatureDisabledMid
		}

		fresh, err := s.StudentRepo.FindByIDForUpdate(ctx, student.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrPhotoStudentNotFound
			}
			return err
		}
		if ok, _ := authorize.CanUpdateStudent(ctx, jwt.PermissionsFromCtx(ctx), fresh, s.UserContext); !ok {
			return ErrPhotoStudentReassigned
		}
		if fresh.PhotoConsentGivenAt == nil && !req.ConsentAck {
			return ErrPhotoConsentWithdrawn
		}

		var capturedOld string
		if fresh.PhotoPath != nil {
			capturedOld = *fresh.PhotoPath
		}
		newURL := req.NewStoredURL
		fresh.PhotoPath = &newURL
		if req.ConsentAck && fresh.PhotoConsentGivenAt == nil {
			s.stampConsent(ctx, fresh)
		}
		if err := s.StudentRepo.Update(ctx, fresh); err != nil {
			return err
		}

		oldCopy := capturedOld
		newCopy := newURL
		sid := student.ID
		tid := tenantID
		tenant.RegisterAfterCommit(ctx, func() {
			if oldCopy != "" && oldCopy != newCopy && s.Unlinker != nil {
				s.Unlinker.UnlinkStored(oldCopy)
			}
			s.broadcastStudentUpdated(tid, sid, photoSourceUploaded)
		})
		return nil
	})
}

// CommitDelete clears photo_path and schedules unlink + broadcast after commit.
func (s *studentPhotoService) CommitDelete(ctx context.Context, studentID int64) (string, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return "", ErrPhotoNoTenant
	}

	var clearedURL string
	err := tenant.WithTenantTx(ctx, s.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := s.requireFeatureEnabled(ctx); err != nil {
			return err
		}
		// Pre-lock auth check on the snapshot.
		snapshot, err := s.StudentRepo.FindByID(ctx, studentID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrPhotoStudentNotFound
			}
			return err
		}
		if ok, _ := authorize.CanUpdateStudent(ctx, jwt.PermissionsFromCtx(ctx), snapshot, s.UserContext); !ok {
			return ErrPhotoStudentForbidden
		}

		if err := s.StudentRepo.LockPhotoFeature(ctx); err != nil {
			return err
		}
		fresh, err := s.StudentRepo.FindByIDForUpdate(ctx, studentID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrPhotoStudentNotFound
			}
			return err
		}
		if ok, _ := authorize.CanUpdateStudent(ctx, jwt.PermissionsFromCtx(ctx), fresh, s.UserContext); !ok {
			return ErrPhotoStudentReassigned
		}

		if fresh.PhotoPath == nil || *fresh.PhotoPath == "" {
			return nil // idempotent
		}
		clearedURL = *fresh.PhotoPath
		fresh.PhotoPath = nil
		if err := s.StudentRepo.Update(ctx, fresh); err != nil {
			return err
		}

		urlCopy := clearedURL
		sid := studentID
		tid := tenantID
		tenant.RegisterAfterCommit(ctx, func() {
			if s.Unlinker != nil {
				s.Unlinker.UnlinkStored(urlCopy)
			}
			s.broadcastStudentUpdated(tid, sid, photoSourceDeleted)
		})
		return nil
	})
	if err != nil {
		return "", err
	}
	return clearedURL, nil
}

// LookupForRead checks feature flag, read access, and filename ownership.
func (s *studentPhotoService) LookupForRead(ctx context.Context, studentID int64, filename string) (string, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return "", ErrPhotoNoTenant
	}
	if filename == "" {
		return "", ErrPhotoFilenameMismatch
	}

	var storedURL string
	err := tenant.WithTenantTx(ctx, s.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := s.requireFeatureEnabled(ctx); err != nil {
			return err
		}
		student, err := s.StudentRepo.FindByID(ctx, studentID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrPhotoStudentNotFound
			}
			return err
		}
		if !authorize.CanReadStudent(
			ctx,
			jwt.PermissionsFromCtx(ctx),
			student,
			s.UserContext,
			s.Settings,
			s.Logger,
		) {
			return ErrPhotoStudentForbidden
		}
		if student.PhotoPath == nil || *student.PhotoPath == "" {
			return ErrPhotoNotSet
		}
		// Compare filename after auth so denied callers cannot probe it.
		storedURL = *student.PhotoPath
		if photoFilenameOf(storedURL) != filename {
			return ErrPhotoFilenameMismatch
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return storedURL, nil
}

// PurgeAllPhotos clears photo_path inside the caller's tx and returns the
// after-commit filesystem cleanup closure.
func (s *studentPhotoService) PurgeAllPhotos(ctx context.Context, tenantID int64) (func(), error) {
	urls, err := s.StudentRepo.PurgeAllPhotos(ctx)
	if err != nil {
		return nil, err
	}
	tid := tenantID
	captured := append([]string(nil), urls...)
	return func() {
		for _, url := range captured {
			if s.Unlinker != nil {
				s.Unlinker.UnlinkStored(url)
			}
		}
		if len(captured) > 0 && s.Logger != nil {
			s.Logger.Info("student photos purged after feature disable",
				slog.Int("cleared_count", len(captured)),
			)
		}
		s.broadcastStudentUpdated(tid, 0, photoSourcePurgeOnDisable)
	}, nil
}

func (s *studentPhotoService) HandleFeatureToggle(ctx context.Context, tenantID int64, value any) (func(), error) {
	notify := s.deferTenantSettingsChanged(tenantID, configModel.KeyStudentPhotosEnabled)
	boolVal, ok := value.(bool)
	if !ok || boolVal {
		tid := tenantID
		return func() {
			s.broadcastStudentUpdated(tid, 0, photoSourceFeatureEnabled)
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

// deferTenantSettingsChanged returns a closure that broadcasts
// tenant_settings_changed so tabs re-resolve tenant-scoped state.
func (s *studentPhotoService) deferTenantSettingsChanged(tenantID int64, key string) func() {
	return func() {
		if s.Broadcaster == nil {
			return
		}
		source := key
		event := realtime.NewEvent(realtime.EventTenantSettingsChanged, "", realtime.EventData{
			Source: &source,
		})
		if err := s.Broadcaster.BroadcastToTenant(tenantID, event); err != nil && s.Logger != nil {
			s.Logger.Warn("tenant_settings_changed broadcast failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("key", key),
				slog.String("error", err.Error()),
			)
		}
	}
}

// photoConsentTransition is the pure result of reconciling requested consent
// against the row's current state — no IO.
type photoConsentTransition struct {
	grantedNow       bool
	withdrawnNow     bool
	removedPhotoPath string
}

// Re-stamping an already-granted row is a no-op so idempotent PUTs preserve
// the original audit timestamp.
func applyPhotoConsent(requestedConsent *bool, fresh *userModels.Student) photoConsentTransition {
	if requestedConsent == nil {
		return photoConsentTransition{}
	}
	hadConsent := fresh.PhotoConsentGivenAt != nil
	wantConsent := *requestedConsent

	if !hadConsent && wantConsent {
		return photoConsentTransition{grantedNow: true}
	}
	if hadConsent && !wantConsent {
		removed := ""
		if fresh.PhotoPath != nil {
			removed = *fresh.PhotoPath
		}
		fresh.PhotoPath = nil
		fresh.PhotoConsentGivenAt = nil
		fresh.PhotoConsentGivenBy = nil
		return photoConsentTransition{withdrawnNow: true, removedPhotoPath: removed}
	}
	return photoConsentTransition{}
}

func (s *studentPhotoService) ApplyConsentTransition(ctx context.Context, requestedConsent *bool, fresh *userModels.Student) {
	transition := applyPhotoConsent(requestedConsent, fresh)
	if transition.grantedNow {
		s.stampConsent(ctx, fresh)
	}
	if transition.withdrawnNow && transition.removedPhotoPath != "" {
		s.ScheduleUnlinkAfterCommit(ctx, transition.removedPhotoPath)
	}
}

// ScheduleUnlinkAfterCommit short-circuits on empty URL so callers can pass
// through without conditionals.
func (s *studentPhotoService) ScheduleUnlinkAfterCommit(ctx context.Context, storedURL string) {
	if storedURL == "" {
		return
	}
	url := storedURL
	tenant.RegisterAfterCommit(ctx, func() {
		if s.Unlinker != nil {
			s.Unlinker.UnlinkStored(url)
		}
	})
}

func (s *studentPhotoService) requireFeatureEnabled(ctx context.Context) error {
	if s.Settings == nil {
		return ErrPhotoFeatureDisabled
	}
	enabled, err := s.Settings.ResolveBool(ctx, configModel.KeyStudentPhotosEnabled)
	if err != nil {
		return fmt.Errorf("photo feature lookup failed: %w", err)
	}
	if !enabled {
		return ErrPhotoFeatureDisabled
	}
	return nil
}

func (s *studentPhotoService) stampConsent(ctx context.Context, student *userModels.Student) {
	actingClaims := jwt.ClaimsFromCtx(ctx)
	actingID := int64(actingClaims.ID)
	now := time.Now()
	student.PhotoConsentGivenAt = &now
	student.PhotoConsentGivenBy = &actingID
}

func (s *studentPhotoService) broadcastStudentUpdated(tenantID, studentID int64, source string) {
	if s.Broadcaster == nil || tenantID <= 0 {
		if tenantID <= 0 && s.Logger != nil {
			s.Logger.Warn("skipping student_updated broadcast — no tenant context",
				slog.Int64("student_id", studentID),
			)
		}
		return
	}
	src := source
	event := realtime.NewEvent(realtime.EventStudentUpdated, "", realtime.EventData{
		Source: &src,
	})
	if err := s.Broadcaster.BroadcastToTenant(tenantID, event); err != nil && s.Logger != nil {
		s.Logger.Warn("failed to broadcast student_updated",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("student_id", studentID),
			slog.String("source", source),
			slog.String("error", err.Error()),
		)
	}
}

func photoFilenameOf(storedURL string) string {
	for i := len(storedURL) - 1; i >= 0; i-- {
		if storedURL[i] == '/' {
			return storedURL[i+1:]
		}
	}
	return storedURL
}
