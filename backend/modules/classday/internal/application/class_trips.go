package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// classTripArrivalReason is the reason the OGS reads at a child's arrival
// time when a Lehrkraft reported a day outing with a return time.
const classTripArrivalReason = "Ausflug"

// classTripReasonMaxRunes bounds the optional destination or occasion, the
// limit both the status-day note and the arrival reason accept.
const classTripReasonMaxRunes = 200

// classTrips is the second write seam of the class-day view (#4799): a
// Lehrkraft reports a class trip or day outing of an assigned class through
// "moto schule". It writes no rows of its own. A trip without a return time
// becomes the status "Klassenfahrt" for every day through the OGS's
// status-day write (ClassTripStatusDays); a one-day outing with a return time
// sets each child's arrival in the OGS through Care Plan's per-child day
// exception, the row the OGS enters in the child's plan. Both run in the
// caller's transaction, so a refusal for one child writes nothing for any
// child; an arrival the OGS already entered is never overwritten.
type classTrips struct {
	rosters    DayRosters
	writeScope ClassTripWriteScope
	statusDays ClassTripStatusDays
	arrivals   studentArrivals
	announcer  ArrivalScheduleAnnouncer
	today      func() calendar.Date
}

// studentArrivals is the part of Care Plan's arrival exceptions a day
// outing with a return time writes.
type studentArrivals interface {
	CreateOrReclaimExceptionForClass(ctx context.Context, schoolClass string, studentID int64, date calendar.Date, arrivalTime *time.Time, reason *string, staffID int64, resolveStaffID func() (int64, error)) (*careplan.ArrivalException, error)
}

// MayReportClassTrips applies operations.school_portal_write_scope.
func (t classTrips) MayReportClassTrips(ctx context.Context) (bool, error) {
	return t.mayReport(ctx)
}

// mayReport is the release check both the verdict and the write apply.
func (t classTrips) mayReport(ctx context.Context) (bool, error) {
	if t.statusDays == nil || t.arrivals == nil {
		return false, classday.ErrClassTripsNotConfigured
	}
	if t.writeScope == nil {
		return false, errors.New("class day class trips: settings not configured")
	}
	may, err := t.writeScope.SchoolMayReportClassTrips(ctx)
	if err != nil {
		return false, fmt.Errorf("class day class trips: resolve school portal write scope: %w", err)
	}
	return may, nil
}

// ReportClassTrip re-checks the school's release, validates the report,
// checks every child against the class roster of the first day and writes
// through the owning seam.
func (t classTrips) ReportClassTrip(ctx context.Context, in classday.ClassTripWrite) (*classday.ClassTripResult, error) {
	may, err := t.mayReport(ctx)
	if err != nil {
		return nil, err
	}
	if !may {
		return nil, classday.ErrClassTripsDisabled
	}
	dates, reason, err := t.validate(in)
	if err != nil {
		return nil, err
	}
	studentIDs := dedupeIDs(in.StudentIDs)
	if err := t.requireClassChildren(ctx, in.SchoolClass, dates[0], studentIDs); err != nil {
		return nil, err
	}

	if in.ReturnTime == nil {
		if err := t.statusDays.RecordClassTrip(ctx, in.SchoolClass, studentIDs, dates, reason); err != nil {
			return nil, err
		}
		return &classday.ClassTripResult{Kind: classday.ClassTripKindStatusDays, StudentCount: len(studentIDs), DateCount: len(dates)}, nil
	}

	// Every exception retains its student lock until the request commits.
	// Use one order even when callers select the same children in reverse.
	slices.Sort(studentIDs)
	arrival := calendar.NormalizeWallClock(*in.ReturnTime)
	arrivalReason := classTripArrivalReason
	if reason != "" {
		arrivalReason += ": " + reason
	}
	staffID := func() (int64, error) { return in.CreatedBy, nil }
	for _, studentID := range studentIDs {
		_, err := t.arrivals.CreateOrReclaimExceptionForClass(ctx, in.SchoolClass, studentID, dates[0], &arrival, &arrivalReason, in.CreatedBy, staffID)
		if err != nil {
			return nil, mapClassTripArrivalError(err)
		}
	}
	if t.announcer != nil {
		t.announcer.AnnounceArrivalScheduleChange(ctx)
	}
	return &classday.ClassTripResult{Kind: classday.ClassTripKindReturnTime, StudentCount: len(studentIDs), DateCount: 1}, nil
}

// mapClassTripArrivalError keeps the school portal's business rejections
// independent of the Care Plan command's own error identities.
func mapClassTripArrivalError(err error) error {
	switch {
	case errors.Is(err, careplan.ErrBulkStudentNotFound):
		return MappedError{err: err, sentinel: classday.ErrClassTripStudentNotInClass}
	case errors.Is(err, careplan.ErrCareExceptionDayConflict), errors.Is(err, careplan.ErrCareExceptionContainsPartialAbsence):
		return MappedError{err: err, sentinel: classday.ErrClassTripArrivalConflict}
	default:
		return err
	}
}

// validate checks the shape of the report before anything is read: the
// range, the return time and the reason. It returns the days of the trip and
// the trimmed reason.
func (t classTrips) validate(in classday.ClassTripWrite) ([]calendar.Date, string, error) {
	if len(in.StudentIDs) == 0 {
		return nil, "", classday.ErrClassTripNoStudents
	}
	from, err := parseDate(in.From)
	if err != nil {
		return nil, "", err
	}
	to, err := parseDate(in.To)
	if err != nil {
		return nil, "", err
	}
	if to.Before(from) || to.After(from.AddDays(classday.ClassTripMaxDays-1)) {
		return nil, "", classday.ErrClassTripInvalidRange
	}
	if from.Before(t.todayDate()) {
		return nil, "", classday.ErrClassTripPastDate
	}
	if in.ReturnTime != nil {
		if to != from {
			return nil, "", classday.ErrClassTripReturnTimeMultiDay
		}
		if weekday := from.Weekday(); weekday == time.Saturday || weekday == time.Sunday {
			return nil, "", classday.ErrClassTripReturnTimeWeekend
		}
	}
	reason := ""
	if in.Reason != nil {
		reason = strings.TrimSpace(*in.Reason)
	}
	if utf8.RuneCountInString(reason) > classTripReasonMaxRunes {
		return nil, "", fmt.Errorf("reason cannot exceed %d characters", classTripReasonMaxRunes)
	}
	dates := make([]calendar.Date, 0, from.DaysUntil(to)+1)
	for day := from; !day.After(to); day = day.AddDays(1) {
		dates = append(dates, day)
	}
	return dates, reason, nil
}

// requireClassChildren refuses a child that is not an OGS child of the class
// on the first day of the trip. Class-list-only entries ("Keine Betreuung")
// have no OGS record to write to and are not selectable.
func (t classTrips) requireClassChildren(ctx context.Context, schoolClass string, day calendar.Date, studentIDs []int64) error {
	if t.rosters == nil {
		return classday.ErrClassTripsNotConfigured
	}
	roster, err := t.rosters.ClassRosterDay(ctx, schoolClass, day)
	if err != nil {
		return err
	}
	inClass := make(map[int64]bool)
	if roster != nil {
		for _, row := range roster.Rows {
			if row.StudentID > 0 && !row.ListEntry {
				inClass[row.StudentID] = true
			}
		}
	}
	for _, studentID := range studentIDs {
		if !inClass[studentID] {
			return classday.ErrClassTripStudentNotInClass
		}
	}
	return nil
}

func (t classTrips) todayDate() calendar.Date {
	if t.today != nil {
		return t.today()
	}
	return calendar.TodayDate()
}

// dedupeIDs keeps the first occurrence of every ID in request order.
func dedupeIDs(ids []int64) []int64 {
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}
