package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PhotoSourceCareEnded labels the live refresh of a photo the end of care
// removed.
const PhotoSourceCareEnded = "photo_care_ended"

// errPhotoCareEndUnchanged leaves a child's savepoint without a write: the
// row no longer qualifies under its lock.
var errPhotoCareEndUnchanged = errors.New("people directory: photo purpose still holds")

// StudentPhotoCareEnd is the nightly end of the photos of children whose
// care is over (#6650), over the photo lifecycle's rows and runtime.
type StudentPhotoCareEnd struct {
	photos *StudentPhotoService
	logger *slog.Logger
}

var _ peopledirectory.StudentPhotoCareEndRetention = (*StudentPhotoCareEnd)(nil)

func NewStudentPhotoCareEnd(photos *StudentPhotoService, logger *slog.Logger) *StudentPhotoCareEnd {
	if photos == nil || logger == nil {
		panic("people directory application: the photo care end needs the photo lifecycle and a logger")
	}
	return &StudentPhotoCareEnd{photos: photos, logger: logger}
}

// EndStudentPhotosAfterCare removes, in the caller's tenant transaction, the
// photo data of every child whose care is over on today (#6650). See
// peopledirectory.StudentPhotoCareEndRetention for the invariants: one child
// per savepoint, all or nothing; no lock wait (NOWAIT), a held row is
// deferred and found again next run; the file only after the commit; one
// log line per child and run; no entry in the consent trail.
func (c *StudentPhotoCareEnd) EndStudentPhotosAfterCare(
	ctx context.Context,
	today calendar.Date,
	logEnd peopledirectory.StudentPhotoCareEndLog,
) (peopledirectory.StudentPhotoCareEndResult, error) {
	s, logger := c.photos, c.logger
	var result peopledirectory.StudentPhotoCareEndResult
	if logEnd == nil || s.runtime == nil {
		return result, errors.New("people directory: the deletion log and the photo runtime are required to end photos after care")
	}
	if _, err := calendar.ParseDate(string(today)); err != nil {
		return result, fmt.Errorf("people directory: end photos after care: today: %w", err)
	}
	if err := s.tx.RequireTransaction(ctx); err != nil {
		return result, err
	}
	tenantID := s.tx.TenantID(ctx)
	if tenantID <= 0 {
		return result, domain.ErrTenantRequired
	}
	var candidates []int64
	err := observeRun(ctx, s.observe, "list_photos_past_purpose", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		ids, listStats, listErr := s.store.ListPhotosPastPurpose(txCtx, today.String())
		stats.Add(listStats)
		candidates = ids
		return listErr
	})
	if err != nil {
		return result, err
	}
	for _, studentID := range candidates {
		ended, err := s.endPhotoAfterCare(ctx, tenantID, studentID, today, logEnd)
		switch {
		case err == nil:
			result.Ended++
			logger.Info("student photo ended after care",
				"tenant_id", tenantID,
				"student_id", studentID,
				"photo_removed", ended.PhotoRemoved,
				"consent_ended", ended.ConsentEnded,
			)
		case errors.Is(err, errPhotoCareEndUnchanged):
			logger.Info("student photo kept: care no longer over under the lock",
				"tenant_id", tenantID,
				"student_id", studentID,
			)
		case errors.Is(err, domain.ErrStudentLockBusy):
			result.Deferred++
			logger.Info("student photo end deferred: row busy",
				"tenant_id", tenantID,
				"student_id", studentID,
			)
		case errors.Is(err, domain.ErrStudentNotFound):
			// Deleted between the list and the lock: nothing is left to end.
			logger.Info("student photo end skipped: child gone",
				"tenant_id", tenantID,
				"student_id", studentID,
			)
		default:
			return result, fmt.Errorf("people directory: end photo of child %d after care: %w", studentID, err)
		}
	}
	return result, nil
}

// endPhotoAfterCare ends one child's photo in its own savepoint. Every
// refusal rolls the savepoint back, so the child keeps all of its photo
// data or loses all of it.
func (s *StudentPhotoService) endPhotoAfterCare(
	ctx context.Context,
	tenantID, studentID int64,
	today calendar.Date,
	logEnd peopledirectory.StudentPhotoCareEndLog,
) (peopledirectory.StudentPhotoCareEnd, error) {
	var ended peopledirectory.StudentPhotoCareEnd
	err := observeRun(ctx, s.observe, "end_student_photo_after_care", s.tx.RunSavepoint, func(spCtx context.Context, stats *domain.OperationStats) error {
		record, found, lockStats, err := s.store.TryLockPhotoRecord(spCtx, studentID)
		stats.Add(lockStats)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrStudentNotFound
		}
		if !domain.PhotoPurposeEndedOn(record, today) || !domain.HoldsPhotoData(record) {
			return errPhotoCareEndUnchanged
		}
		storedURL := ""
		if record.PhotoPath != nil {
			storedURL = *record.PhotoPath
		}
		ended = peopledirectory.StudentPhotoCareEnd{
			TenantID: tenantID, StudentID: studentID, LastCareDay: record.EnrolledUntil,
			Graduated: record.IsAlumnus(), PhotoRemoved: storedURL != "",
			ConsentEnded: record.PhotoConsentGivenAt != nil,
		}
		saveStats, err := s.store.SavePhoto(spCtx, domain.StudentPhoto{StudentID: studentID, Status: record.Status})
		stats.Add(saveStats)
		if err != nil {
			return err
		}
		if err := logEnd(spCtx, ended); err != nil {
			return fmt.Errorf("people directory: log ended photo: %w", err)
		}
		if storedURL != "" {
			// The durable queue entry commits with the cleared row; the
			// after-commit unlink is the fast path the queue backs up.
			if err := s.runtime.QueueUnlinkStoredPhoto(spCtx, storedURL); err != nil {
				return err
			}
		}
		s.tx.RegisterAfterCommit(spCtx, func() {
			if storedURL != "" {
				s.runtime.UnlinkStoredPhoto(storedURL)
			}
			s.runtime.BroadcastPhotoChange(tenantID, studentID, PhotoSourceCareEnded)
		})
		return nil
	})
	return ended, err
}
