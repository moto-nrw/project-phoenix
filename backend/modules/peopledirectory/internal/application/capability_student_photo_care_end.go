package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

var _ peopledirectory.StudentPhotoCareEndRetention = (*Service)(nil)

// errPhotoCareEndUnchanged leaves a child's savepoint without a write: the
// row no longer qualifies under its lock.
var errPhotoCareEndUnchanged = errors.New("people directory: photo purpose still holds")

// EndStudentPhotosAfterCare removes, in the caller's tenant transaction, the
// photo data of every child whose care is over on today (#6650). See
// peopledirectory.StudentPhotoCareEndRetention for the invariants: one child
// per savepoint, all or nothing; no lock wait (NOWAIT), a held row is
// deferred and found again next run; the file only after the commit; no
// entry in the consent trail.
func (s *Service) EndStudentPhotosAfterCare(
	ctx context.Context,
	today calendar.Date,
	logEnd peopledirectory.StudentPhotoCareEndLog,
) (peopledirectory.StudentPhotoCareEndResult, error) {
	var result peopledirectory.StudentPhotoCareEndResult
	photos := s.studentPhotos
	tenantID, candidates, err := photos.photosPastPurpose(ctx, today, logEnd)
	if err != nil {
		return result, err
	}
	for _, studentID := range candidates {
		outcome, err := photoCareEndOutcome(photos.endPhotoAfterCare(ctx, tenantID, studentID, today, logEnd))
		if err != nil {
			return result, fmt.Errorf("people directory: end photo of child %d after care: %w", studentID, err)
		}
		switch outcome {
		case peopledirectory.StudentPhotoCareEnded:
			result.Ended++
		case peopledirectory.StudentPhotoCareDeferred:
			result.Deferred++
		}
		result.Children = append(result.Children, peopledirectory.StudentPhotoCareEndChild{StudentID: studentID, Outcome: outcome})
	}
	return result, nil
}

// photoCareEndOutcome sorts a child's result: the expected refusals leave
// it for a later run or end nothing; every other error aborts the run.
func photoCareEndOutcome(err error) (string, error) {
	switch {
	case err == nil:
		return peopledirectory.StudentPhotoCareEnded, nil
	case errors.Is(err, domain.ErrStudentLockBusy):
		return peopledirectory.StudentPhotoCareDeferred, nil
	case errors.Is(err, errPhotoCareEndUnchanged):
		return peopledirectory.StudentPhotoCareKept, nil
	case errors.Is(err, domain.ErrStudentNotFound):
		return peopledirectory.StudentPhotoCareGone, nil
	default:
		return "", err
	}
}

// photosPastPurpose checks the run's preconditions and lists its
// candidates from the database.
func (s *StudentPhotoService) photosPastPurpose(
	ctx context.Context,
	today calendar.Date,
	logEnd peopledirectory.StudentPhotoCareEndLog,
) (int64, []int64, error) {
	if _, unbound := s.runtime.(DisabledStudentPhotos); logEnd == nil || s.runtime == nil || unbound {
		// Without the root's runtime no file could be queued for removal:
		// refuse instead of clearing rows whose files would stay.
		return 0, nil, errors.New("people directory: the deletion log and the photo runtime are required to end photos after care")
	}
	if _, err := calendar.ParseDate(string(today)); err != nil {
		return 0, nil, fmt.Errorf("people directory: end photos after care: today: %w", err)
	}
	if err := s.tx.RequireTransaction(ctx); err != nil {
		return 0, nil, err
	}
	tenantID := s.tx.TenantID(ctx)
	if tenantID <= 0 {
		return 0, nil, domain.ErrTenantRequired
	}
	var candidates []int64
	err := observeRun(ctx, s.observe, "list_photos_past_purpose", joinTransaction, func(txCtx context.Context, stats *domain.OperationStats) error {
		ids, listStats, listErr := s.store.ListPhotosPastPurpose(txCtx, today.String())
		stats.Add(listStats)
		candidates = ids
		return listErr
	})
	return tenantID, candidates, err
}

// joinTransaction runs fn in the caller's transaction, which the run
// already required.
func joinTransaction(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

// endPhotoAfterCare ends one child's photo in its own savepoint. Every
// refusal rolls the savepoint back, so the child keeps all of its photo
// data or loses all of it.
func (s *StudentPhotoService) endPhotoAfterCare(
	ctx context.Context,
	tenantID, studentID int64,
	today calendar.Date,
	logEnd peopledirectory.StudentPhotoCareEndLog,
) error {
	return observeRun(ctx, s.observe, "end_student_photo_after_care", s.tx.RunSavepoint, func(spCtx context.Context, stats *domain.OperationStats) error {
		record, err := s.lockPhotoPastPurpose(spCtx, stats, studentID, today)
		if err != nil {
			return err
		}
		ended := peopledirectory.StudentPhotoCareEnd{
			TenantID: tenantID, StudentID: studentID, LastCareDay: record.EnrolledUntil,
			Graduated: record.IsAlumnus(), PhotoRemoved: record.PhotoPath != nil,
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
		storedURL := ""
		if record.PhotoPath != nil {
			storedURL = *record.PhotoPath
		}
		// Queues the durable cleanup entry with the cleared row and removes
		// the file after the commit; an empty URL is a no-op.
		return s.ScheduleUnlink(spCtx, storedURL)
	})
}

// lockPhotoPastPurpose takes the child's row without waiting and re-checks
// under the lock that the care is over and the row still holds photo data.
func (s *StudentPhotoService) lockPhotoPastPurpose(
	ctx context.Context,
	stats *domain.OperationStats,
	studentID int64,
	today calendar.Date,
) (domain.StudentRecord, error) {
	record, found, lockStats, err := s.store.TryLockPhotoRecord(ctx, studentID)
	stats.Add(lockStats)
	switch {
	case err != nil:
		return record, err
	case !found:
		return record, domain.ErrStudentNotFound
	case !domain.PhotoPurposeEndedOn(record, today) || !domain.HoldsPhotoData(record):
		return record, errPhotoCareEndUnchanged
	}
	return record, nil
}
