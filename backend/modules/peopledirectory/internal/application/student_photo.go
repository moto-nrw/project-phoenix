package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
)

// Broadcast source labels for the live refresh of a photo change.
const (
	PhotoSourceUploaded       = "photo_uploaded"
	PhotoSourceDeleted        = "photo_deleted"
	PhotoSourcePurgeOnDisable = "tenant_photos_disabled"
)

// StudentPhotoService owns the tenant-scoped student-photo lifecycle: the
// child's photo column, the per-tenant feature gate that serializes it, and
// the consent the photo depends on.
type StudentPhotoService struct {
	store   ports.StudentPhotoStore
	runtime ports.StudentPhotoRuntime
	tx      ports.Transaction
	observe ports.Observer
	now     func() time.Time
}

func NewStudentPhotos(
	store ports.StudentPhotoStore,
	runtime ports.StudentPhotoRuntime,
	tx ports.Transaction,
	observe ports.Observer,
	now func() time.Time,
) *StudentPhotoService {
	if store == nil || tx == nil || observe == nil || now == nil {
		panic("people directory application: all student photo dependencies are required")
	}
	return &StudentPhotoService{store: store, runtime: runtime, tx: tx, observe: observe, now: now}
}

// CommitPhoto swaps the child's stored photo. It validates the feature gate
// and the recorded consent twice: once on the snapshot and again under the row
// lock, because a feature disable or a consent withdrawal committing in
// between must not leave an orphaned image. Who may reach the route is the
// caller's decision, like on every other child-data route.
func (s *StudentPhotoService) CommitPhoto(ctx context.Context, studentID int64, storedURL string, consentAck bool) error {
	return s.run(ctx, "commit_student_photo", func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.requireFeatureEnabled(txCtx); err != nil {
			return err
		}
		photo, err := s.readPhoto(txCtx, stats, studentID, false)
		if err != nil {
			return err
		}
		if photo.PhotoConsentGivenAt == nil && !consentAck {
			return domain.ErrPhotoConsentRequired
		}

		// Serialize against a feature-disable purge.
		lockStats, err := s.store.LockPhotoFeature(txCtx)
		stats.Add(lockStats)
		if err != nil {
			return err
		}
		if err := s.requireFeatureEnabled(txCtx); err != nil {
			return domain.ErrPhotoFeatureDisabledMid
		}

		fresh, err := s.readPhoto(txCtx, stats, studentID, true)
		if err != nil {
			return err
		}
		if fresh.PhotoConsentGivenAt == nil && !consentAck {
			return domain.ErrPhotoConsentWithdrawn
		}

		before := fresh
		capturedOld := fresh.StoredPhoto()
		newURL := storedURL
		fresh.PhotoPath = &newURL
		changedAt := s.now()
		if consentAck && fresh.PhotoConsentGivenAt == nil {
			grantedAt, grantedBy := changedAt, s.runtime.ActingAccountID(txCtx)
			fresh.PhotoConsentGivenAt = &grantedAt
			fresh.PhotoConsentGivenBy = &grantedBy
		}
		saveStats, err := s.store.SavePhoto(txCtx, fresh)
		stats.Add(saveStats)
		if err != nil {
			return err
		}
		if consentAck {
			if err := s.recordConsent(txCtx, before, fresh, changedAt); err != nil {
				return err
			}
		}

		tenantID := s.tx.TenantID(txCtx)
		s.tx.RegisterAfterCommit(txCtx, func() {
			if capturedOld != "" && capturedOld != newURL {
				s.runtime.UnlinkStoredPhoto(capturedOld)
			}
			s.runtime.BroadcastPhotoChange(tenantID, studentID, PhotoSourceUploaded)
		})
		return nil
	})
}

// ClearPhoto detaches the child's stored photo and schedules the file cleanup
// and live refresh for after the commit. A child without a photo is a no-op,
// so a repeated delete stays successful.
func (s *StudentPhotoService) ClearPhoto(ctx context.Context, studentID int64) (clearedURL string, err error) {
	err = s.run(ctx, "clear_student_photo", func(txCtx context.Context, stats *domain.OperationStats) error {
		clearedURL = ""
		if err := s.requireFeatureEnabled(txCtx); err != nil {
			return err
		}
		if _, err := s.readPhoto(txCtx, stats, studentID, false); err != nil {
			return err
		}

		lockStats, err := s.store.LockPhotoFeature(txCtx)
		stats.Add(lockStats)
		if err != nil {
			return err
		}
		fresh, err := s.readPhoto(txCtx, stats, studentID, true)
		if err != nil {
			return err
		}
		if fresh.StoredPhoto() == "" {
			return nil
		}

		clearedURL = fresh.StoredPhoto()
		fresh.PhotoPath = nil
		saveStats, err := s.store.SavePhoto(txCtx, fresh)
		stats.Add(saveStats)
		if err != nil {
			return err
		}

		detached, tenantID := clearedURL, s.tx.TenantID(txCtx)
		s.tx.RegisterAfterCommit(txCtx, func() {
			s.runtime.UnlinkStoredPhoto(detached)
			s.runtime.BroadcastPhotoChange(tenantID, studentID, PhotoSourceDeleted)
		})
		return nil
	})
	if err != nil {
		return "", err
	}
	return clearedURL, nil
}

// FindPhoto resolves the stored URL a read route may serve. The filename is
// compared after the access check, so a denied caller cannot probe it.
func (s *StudentPhotoService) FindPhoto(ctx context.Context, studentID int64, filename string) (storedURL string, err error) {
	if filename == "" {
		return "", domain.ErrPhotoFilenameMismatch
	}
	err = s.run(ctx, "find_student_photo", func(txCtx context.Context, stats *domain.OperationStats) error {
		storedURL = ""
		if err := s.requireFeatureEnabled(txCtx); err != nil {
			return err
		}
		photo, err := s.readPhoto(txCtx, stats, studentID, false)
		if err != nil {
			return err
		}
		if photo.StoredPhoto() == "" {
			return domain.ErrPhotoNotSet
		}
		storedURL = photo.StoredPhoto()
		if domain.PhotoFilenameOf(storedURL) != filename {
			storedURL = ""
			return domain.ErrPhotoFilenameMismatch
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return storedURL, nil
}

// PurgePhotos detaches every stored photo of the tenant inside the caller's
// transaction and returns the URLs whose files the caller must remove after
// the commit.
func (s *StudentPhotoService) PurgePhotos(ctx context.Context) (result []string, err error) {
	err = s.run(ctx, "purge_student_photos", func(txCtx context.Context, stats *domain.OperationStats) error {
		lockStats, err := s.store.LockPhotoFeature(txCtx)
		stats.Add(lockStats)
		if err != nil {
			return err
		}
		var purgeStats domain.OperationStats
		result, purgeStats, err = s.store.PurgePhotos(txCtx)
		stats.Add(purgeStats)
		return err
	})
	return result, err
}

// ApplyPhotoConsent reconciles a requested consent against the row the caller
// holds and returns the photo slice as it must be written. The caller persists
// it with its own write, so the consent commits with the edit that carries it;
// the file a withdrawal detaches is scheduled for removal after that commit.
func (s *StudentPhotoService) ApplyPhotoConsent(
	ctx context.Context,
	current domain.StudentPhoto,
	requestedConsent *bool,
) domain.StudentPhoto {
	if requestedConsent == nil || s.runtime == nil {
		return current
	}
	updated, transition := domain.ApplyPhotoConsent(
		requestedConsent, current, s.now(), s.runtime.ActingAccountID(ctx))
	if transition.RemovedPhotoPath != "" {
		detached := transition.RemovedPhotoPath
		s.tx.RegisterAfterCommit(ctx, func() { s.runtime.UnlinkStoredPhoto(detached) })
	}
	return updated
}

// ScheduleUnlink removes a detached file after the caller's commit. An empty
// URL is a no-op so callers need no conditional.
func (s *StudentPhotoService) ScheduleUnlink(ctx context.Context, storedURL string) {
	if storedURL == "" || s.runtime == nil {
		return
	}
	detached := storedURL
	s.tx.RegisterAfterCommit(ctx, func() { s.runtime.UnlinkStoredPhoto(detached) })
}

// readPhoto loads the photo slice, refusing a missing child and a graduate
// alike: an alumnus is out of every photo route's reach.
func (s *StudentPhotoService) readPhoto(
	ctx context.Context,
	stats *domain.OperationStats,
	studentID int64,
	lock bool,
) (domain.StudentPhoto, error) {
	read := s.store.FindPhoto
	if lock {
		read = s.store.LockPhoto
	}
	photo, found, readStats, err := read(ctx, studentID)
	stats.Add(readStats)
	if err != nil {
		return domain.StudentPhoto{}, err
	}
	if !found || photo.IsAlumnus() {
		return domain.StudentPhoto{}, domain.ErrStudentNotFound
	}
	return photo, nil
}

func (s *StudentPhotoService) requireFeatureEnabled(ctx context.Context) error {
	if s.runtime == nil {
		return domain.ErrPhotoFeatureDisabled
	}
	enabled, err := s.runtime.PhotoFeatureEnabled(ctx)
	if err != nil {
		return fmt.Errorf("photo feature lookup failed: %w", err)
	}
	if !enabled {
		return domain.ErrPhotoFeatureDisabled
	}
	return nil
}

func (s *StudentPhotoService) recordConsent(
	ctx context.Context,
	before, after domain.StudentPhoto,
	changedAt time.Time,
) error {
	actor := s.runtime.ActingAccountID(ctx)
	var actorAccountID *int64
	if actor > 0 {
		actorAccountID = &actor
	}
	return s.runtime.RecordPhotoConsent(
		ctx,
		domain.StudentConsentSnapshot{StudentID: before.StudentID, PhotoConsentGivenAt: before.PhotoConsentGivenAt},
		domain.StudentConsentSnapshot{StudentID: after.StudentID, PhotoConsentGivenAt: after.PhotoConsentGivenAt},
		actorAccountID,
		changedAt,
	)
}

func (s *StudentPhotoService) run(ctx context.Context, operation string, fn func(context.Context, *domain.OperationStats) error) error {
	if s.runtime == nil {
		return domain.ErrPhotoFeatureDisabled
	}
	return observeRun(ctx, s.observe, operation, s.tx.RunWrite, fn)
}
