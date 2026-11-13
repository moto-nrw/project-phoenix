package studentpresencehttp

import (
	context "context"
	time "time"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence"

	absencerecords "github.com/moto-ogs/project-phoenix/modules/careplan/absencerecords"
	peopleModule "github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	calendar "github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

func boolPtrValue(v *bool) bool {
	return v != nil && *v
}

func statusReportedAt(now time.Time, existing *time.Time) time.Time {
	if existing != nil {
		return *existing
	}
	return now
}

// clearRequested reports whether the request explicitly lifts a status
// ("sick": false / "excused": false).
func clearRequested(requested *bool) bool {
	return requested != nil && !*requested
}

func (rs *Resource) PersistStudentStatusHistory(ctx context.Context, student *peopleModule.StudentState, requestedSick, requestedExcused *bool, wasSick, wasExcused bool, now time.Time, sickNote *string) error {
	if rs.StudentStatusDayService == nil {
		return nil
	}
	if student == nil {
		return nil
	}

	today := calendar.DateFromTime(now)
	if err := rs.StudentStatusDayService.ValidateProfileStatusChange(ctx, student.ID, today, requestedSick, requestedExcused); err != nil {
		return err
	}
	// Only the sick status carries a free-text reason; excused stays note-less.
	if err := rs.persistSingleStatusHistory(ctx, student.ID, absencerecords.StudentStatusDaySick, statusTransition{
		wasActive: wasSick, isActive: boolPtrValue(student.Sick), clearRequested: clearRequested(requestedSick),
	}, statusReportedAt(now, student.SickSince), today, now, sickNote); err != nil {
		return err
	}
	if err := rs.persistSingleStatusHistory(ctx, student.ID, absencerecords.StudentStatusDayExcused, statusTransition{
		wasActive: wasExcused, isActive: boolPtrValue(student.Excused), clearRequested: clearRequested(requestedExcused),
	}, statusReportedAt(now, student.ExcusedSince), today, now, nil); err != nil {
		return err
	}
	return nil
}

// statusTransition describes one status flag across an update. wasActive and
// isActive are the stored live flag before and after; clearRequested is set
// when the request explicitly lifts the status.
type statusTransition struct {
	wasActive      bool
	isActive       bool
	clearRequested bool
}

func (rs *Resource) persistSingleStatusHistory(ctx context.Context, studentID int64, status string, transition statusTransition, reportedAt time.Time, date calendar.Date, now time.Time, note *string) error {
	accountID := sessiontoken.ActorAccountIDFromCtx(ctx)
	if transition.isActive && transition.wasActive && note == nil {
		// The status stands unchanged: a day that already shows it keeps the
		// report that set it, with its source and report time (#6291).
		standing, err := rs.statusStandsOn(ctx, studentID, status, date)
		if err != nil || standing {
			return err
		}
	}
	if transition.isActive {
		return rs.StudentStatusDayService.UpsertReported(ctx, &absencerecords.StudentStatusDay{
			StudentID:           studentID,
			Date:                date,
			Status:              status,
			ReportedAt:          reportedAt,
			Source:              absencerecords.StudentStatusSourceManual,
			Note:                note,
			ReportedByAccountID: accountID,
		})
	}
	if transition.wasActive {
		// Lifting the flag clears the day's report and keeps who made it
		// (#6842); only a flag without a row gets one first.
		standing, err := rs.statusStandsOn(ctx, studentID, status, date)
		if err != nil {
			return err
		}
		if !standing {
			if err := rs.StudentStatusDayService.UpsertReported(ctx, &absencerecords.StudentStatusDay{
				StudentID:           studentID,
				Date:                date,
				Status:              status,
				ReportedAt:          reportedAt,
				Source:              absencerecords.StudentStatusSourceManual,
				ReportedByAccountID: accountID,
			}); err != nil {
				return err
			}
		}
		return rs.StudentStatusDayService.MarkCleared(ctx, studentID, status, date, now, absencerecords.StudentStatusSourceManual, accountID)
	}
	if transition.clearRequested {
		// Today's row can be active without the live flag: a parent's
		// Abmeldung (#1735) and a day planned in advance never set it. The
		// profile shows those rows as the same status, so lifting it must clear
		// them too, whoever entered them (#3854). Without an active row this is
		// a no-op.
		return rs.StudentStatusDayService.MarkCleared(ctx, studentID, status, date, now, absencerecords.StudentStatusSourceManual, accountID)
	}
	return nil
}

// statusStandsOn reports whether a row of the status stands for the day.
func (rs *Resource) statusStandsOn(ctx context.Context, studentID int64, status string, date calendar.Date) (bool, error) {
	rows, err := rs.StudentStatusDayService.FindSignedOffByStudentIDsAndDate(ctx, []int64{studentID}, date)
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if row.Status == status {
			return true, nil
		}
	}
	return false, nil
}

func (rs *Resource) NotifyAbsenceReported(ctx context.Context, tenantID int64, studentIDs []int64, status string, dates []calendar.Date, fromParent bool, actorAccountID int64) error {
	if rs.AbsenceNotifier == nil {
		return nil
	}
	return rs.AbsenceNotifier.NotifyAbsenceReported(ctx, studentpresence.AbsenceReport{
		TenantID:       tenantID,
		StudentIDs:     studentIDs,
		Status:         status,
		Dates:          dates,
		FromParent:     fromParent,
		ActorAccountID: actorAccountID,
	})
}
