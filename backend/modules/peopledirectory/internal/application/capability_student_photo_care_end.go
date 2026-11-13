package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

var _ peopledirectory.StudentPhotoCareEndRetention = (*Service)(nil)

// EndStudentPhotosAfterCare removes photo data in one savepoint and a fixed
// number of statements. Locked children are deferred without waiting; every
// write, audit entry and durable cleanup intent succeeds or rolls back together.
func (s *Service) EndStudentPhotosAfterCare(
	ctx context.Context,
	today calendar.Date,
	logEnd peopledirectory.StudentPhotoCareEndLog,
) (peopledirectory.StudentPhotoCareEndResult, error) {
	var result peopledirectory.StudentPhotoCareEndResult
	photos := s.studentPhotos
	tenantID, candidates, err := photos.photosPastPurpose(ctx, today, logEnd)
	if err != nil || len(candidates) == 0 {
		return result, err
	}
	err = observeRun(ctx, photos.observe, "end_student_photos_after_care", photos.tx.RunSavepoint, func(spCtx context.Context, stats *domain.OperationStats) error {
		records, present, lockStats, err := photos.store.TryLockPhotoRecords(spCtx, candidates)
		stats.Add(lockStats)
		if err != nil {
			return err
		}
		ended, ids, urls, outcomes := photoCareEndBatch(tenantID, candidates, records, present, today)
		result = outcomes
		if len(ids) == 0 {
			return nil
		}
		saveStats, err := photos.store.ClearPhotos(spCtx, ids)
		stats.Add(saveStats)
		if err != nil {
			return err
		}
		if err := logEnd(spCtx, ended); err != nil {
			return fmt.Errorf("people directory: log ended photos: %w", err)
		}
		if err := photos.scheduleUnlinks(spCtx, urls); err != nil {
			return err
		}
		result.Ended = len(ids)
		return nil
	})
	return result, err
}

// photoCareEndBatch re-checks each locked record and prepares the bulk write
// and its log without database work inside the loop.
func photoCareEndBatch(tenantID int64, candidates []int64, records map[int64]domain.StudentRecord, present map[int64]bool, today calendar.Date) (
	[]peopledirectory.StudentPhotoCareEnd, []int64, []string, peopledirectory.StudentPhotoCareEndResult,
) {
	var result peopledirectory.StudentPhotoCareEndResult
	var ended []peopledirectory.StudentPhotoCareEnd
	var ids []int64
	var urls []string
	for _, id := range candidates {
		record, locked := records[id]
		outcome := photoCareEndOutcome(record, present[id], locked, today)
		result.Children = append(result.Children, peopledirectory.StudentPhotoCareEndChild{StudentID: id, Outcome: outcome})
		if outcome == peopledirectory.StudentPhotoCareDeferred {
			result.Deferred++
		}
		if outcome != peopledirectory.StudentPhotoCareEnded {
			continue
		}
		ids = append(ids, id)
		ended = append(ended, peopledirectory.StudentPhotoCareEnd{
			TenantID: tenantID, StudentID: id, LastCareDay: lastCareDay(record),
			Graduated: record.IsAlumnus(), PhotoRemoved: record.PhotoPath != nil,
			ConsentEnded: record.PhotoConsentGivenAt != nil,
		})
		if record.PhotoPath != nil && *record.PhotoPath != "" {
			urls = append(urls, *record.PhotoPath)
		}
	}
	return ended, ids, urls, result
}

func photoCareEndOutcome(record domain.StudentRecord, present, locked bool, today calendar.Date) string {
	switch {
	case !present:
		return peopledirectory.StudentPhotoCareGone
	case !locked:
		return peopledirectory.StudentPhotoCareDeferred
	case !domain.PhotoPurposeEndedOn(record, today) || !domain.HoldsPhotoData(record):
		return peopledirectory.StudentPhotoCareKept
	default:
		return peopledirectory.StudentPhotoCareEnded
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
		ids, listStats, listErr := s.store.ListPhotosPastPurpose(txCtx, today)
		stats.Add(listStats)
		candidates = ids
		return listErr
	})
	return tenantID, candidates, err
}

// joinTransaction runs fn in the caller's transaction, which the run
// already required.
func joinTransaction(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

// lastCareDay is the record's last care day, nil when none is recorded.
func lastCareDay(record domain.StudentRecord) *calendar.Date {
	if record.EnrolledUntil == "" {
		return nil
	}
	day := calendar.Date(record.EnrolledUntil)
	return &day
}
