package application

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// GuardianAbsences writes a guardian's full-day status in the caller's
// tenant transaction.
type GuardianAbsences struct {
	Records ports.GuardianAbsenceRecords
}

func (s *GuardianAbsences) ReportGuardianAbsence(ctx context.Context, report careplan.GuardianAbsenceReport) ([]careplan.StudentStatusDay, error) {
	if len(report.Dates) == 0 {
		return nil, errors.New("guardian absence: at least one date is required")
	}
	sorted := slices.Clone(report.Dates)
	slices.SortFunc(sorted, compareDates)
	first, last := sorted[0], sorted[len(sorted)-1]
	if err := s.refuseManualPartialAbsence(ctx, report.StudentID, sorted); err != nil {
		return nil, err
	}
	if err := s.write(ctx, report); err != nil {
		return nil, err
	}
	rows, err := s.Records.ListStudentStatusDays(ctx, careplan.StudentStatusDayFilter{
		StudentIDs: []int64{report.StudentID}, From: first, To: last, ActiveOnly: true,
	})
	if err != nil {
		return nil, err
	}
	// The range spans min..max, so a non-contiguous report (Mon + Wed) can
	// also return an unrelated row in between, which must not be surfaced.
	return slices.DeleteFunc(rows, func(row careplan.StudentStatusDay) bool {
		return row.Status != report.Status || !slices.Contains(report.Dates, row.Date)
	}), nil
}

// refuseManualPartialAbsence serializes with staff writes on every requested
// day, in ascending date order, before anything is cleared. Automatic
// excusals (pulled-forward pickup times) coexist with a full-day status.
func (s *GuardianAbsences) refuseManualPartialAbsence(ctx context.Context, studentID int64, sorted []careplan.Date) error {
	for _, date := range sorted {
		if err := s.Records.LockStudentAndExceptionDay(ctx, studentID, date.String()); err != nil {
			return err
		}
	}
	rows, err := s.Records.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{
		StudentIDs: []int64{studentID}, From: sorted[0], To: sorted[len(sorted)-1],
	})
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.HasManualPartialAbsence() && slices.Contains(sorted, row.ExceptionDate) {
			return careplan.ErrManualPartialAbsenceConflict
		}
	}
	return nil
}

func (s *GuardianAbsences) write(ctx context.Context, report careplan.GuardianAbsenceReport) error {
	for _, other := range careplan.StudentStatusDayStatusesExcept(report.Status) {
		if err := s.Records.ClearStudentStatusDays(ctx, report.StudentID, other, report.Dates, report.ReportedAt, careplan.StudentStatusSourceParent); err != nil {
			return err
		}
	}
	for _, date := range report.Dates {
		guardianAccountID := report.GuardianAccountID
		if _, err := s.Records.UpsertStudentStatusDay(ctx, careplan.StudentStatusDay{
			StudentID: report.StudentID, Date: date, Status: report.Status, ReportedAt: report.ReportedAt,
			Source: careplan.StudentStatusSourceParent, GuardianAccountID: &guardianAccountID, Note: report.Note,
		}); err != nil {
			return err
		}
	}
	return nil
}

func compareDates(a, b careplan.Date) int {
	return calendar.Date(a).Compare(calendar.Date(b))
}

// GuardianPickupExceptions writes the guardian leg of a day's pickup
// exception. A nil Excusal skips the derived block excusal coupling.
// IsUniqueViolation classifies storage errors; the application layer cannot
// read driver errors itself.
type GuardianPickupExceptions struct {
	Records           ports.GuardianPickupRecords
	Excusal           ports.GuardianPickupExcusal
	IsUniqueViolation func(error) bool
}

// ApplyGuardianPickupException never touches a staff row. A nil time removes
// the guardian row, a non-nil time updates or creates it. A unique violation
// anywhere in the apply, the excusal sync included, means a concurrent write
// won: it returns ErrGuardianPickupExceptionRaced joined with the raw error.
func (s *GuardianPickupExceptions) ApplyGuardianPickupException(ctx context.Context, change careplan.GuardianPickupChange) error {
	err := s.apply(ctx, change)
	if err != nil && s.IsUniqueViolation(err) {
		return fmt.Errorf("%w: %w", careplan.ErrGuardianPickupExceptionRaced, err)
	}
	return err
}

func (s *GuardianPickupExceptions) apply(ctx context.Context, change careplan.GuardianPickupChange) error {
	existing, err := s.find(ctx, change.StudentID, change.Date)
	if err != nil {
		return err
	}
	if existing != nil && existing.Source == careplan.ExceptionSourceStaff {
		return careplan.ErrPickupExceptionStaffOwned
	}
	if err := s.writeLeg(ctx, existing, change); err != nil {
		return err
	}
	return s.syncExcusal(ctx, change.StudentID, change.Date)
}

func (s *GuardianPickupExceptions) writeLeg(ctx context.Context, existing *careplan.PickupException, change careplan.GuardianPickupChange) error {
	switch {
	case change.PickupTime == nil && existing == nil:
		return nil
	case change.PickupTime == nil:
		return s.WithdrawGuardianPickupException(ctx, *existing)
	case existing != nil:
		return s.update(ctx, *existing, change)
	default:
		return s.create(ctx, change)
	}
}

// WithdrawGuardianPickupException releases the derived excusal BEFORE the
// row goes away: the foreign key's ON DELETE SET NULL would otherwise strand
// the excused blocks as absent with no provenance to restore from.
func (s *GuardianPickupExceptions) WithdrawGuardianPickupException(ctx context.Context, row careplan.PickupException) error {
	if s.Excusal != nil {
		if err := s.Excusal.ReleaseBeforeDelete(ctx, &careplan.PickupException{ID: row.ID, ExcusedAuto: row.ExcusedAuto}); err != nil {
			return err
		}
	}
	return s.Records.DeletePickupException(ctx, row.ID)
}

func (s *GuardianPickupExceptions) update(ctx context.Context, row careplan.PickupException, change careplan.GuardianPickupChange) error {
	guardianID := change.GuardianAccountID
	row.PickupTime = change.PickupTime
	row.Reason = change.Reason
	row.Source = careplan.ExceptionSourceGuardian
	row.CreatedBy = 0
	row.CreatedByGuardian = &guardianID
	// Scanned TIME values (e.g. a carried-over excused_from) land on year 0.
	row.NormalizeWallClockTimes()
	return s.Records.UpdatePickupException(ctx, row)
}

func (s *GuardianPickupExceptions) create(ctx context.Context, change careplan.GuardianPickupChange) error {
	guardianID := change.GuardianAccountID
	_, err := s.Records.CreatePickupException(ctx, careplan.PickupException{
		TenantID: change.TenantID, StudentID: change.StudentID, ExceptionDate: change.Date,
		PickupTime: change.PickupTime, Reason: change.Reason,
		Source: careplan.ExceptionSourceGuardian, CreatedByGuardian: &guardianID,
	})
	return err
}

// syncExcusal couples the resulting row with the per-block excusal: a pull
// forward against the weekly baseline excuses the later blocks, moving it
// back releases them again.
func (s *GuardianPickupExceptions) syncExcusal(ctx context.Context, studentID int64, date careplan.Date) error {
	if s.Excusal == nil {
		return nil
	}
	row, err := s.find(ctx, studentID, date)
	if err != nil || row == nil {
		return err
	}
	_, err = s.Excusal.Sync(ctx, row.ID)
	return err
}

func (s *GuardianPickupExceptions) find(ctx context.Context, studentID int64, date careplan.Date) (*careplan.PickupException, error) {
	rows, err := s.Records.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{studentID}, Date: date})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}
