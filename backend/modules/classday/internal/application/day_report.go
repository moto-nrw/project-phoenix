package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
)

// studentStatusDayCancelled is the class-day-only marker for a pickup
// exception without a time: the care for this day was called off ("kommt
// heute nicht"). Not a persisted status day.
const studentStatusDayCancelled = "cancelled"

// dayReports builds the per-class day view (#1772) and the per-child
// supervision sheet (#2527) and records every read in the GDPR log.
type dayReports struct {
	rosters           DayRosters
	statusDays        StatusDays
	times             EffectiveDayTimes
	careDays          ports.CareDays
	classExceptions   ClassArrivalExceptions
	companions        Companions
	students          SheetStudents
	emergencyContacts EmergencyContacts
	accessLog         AccessLog
}

func newDayReports(deps ClassDayDependencies) *dayReports {
	return &dayReports{
		rosters: deps.Rosters, statusDays: deps.StatusDays, times: deps.Times, careDays: deps.CareDays,
		classExceptions: deps.ClassArrivalExceptions, companions: deps.Companions,
		students: deps.Students, emergencyContacts: deps.EmergencyContacts,
		accessLog: deps.AccessLog,
	}
}

// classDayFacts are the per-student day facts the report projects onto the
// roster rows. Grouped instead of passed as ten parallel maps: every one of
// them is keyed by student ID for the same date, and the builder reads them
// together.
type classDayFacts struct {
	statuses           map[int64]string
	statusReportedAt   map[int64]time.Time
	departures         map[int64]string
	arrivals           map[int64]string
	pickups            map[int64]string
	pickupRegular      map[int64]string
	pickupChanged      map[int64]bool
	pickupChangedAt    map[int64]time.Time
	arrivalCancelledAt map[int64]time.Time
	notScheduled       map[int64]bool
}

func newClassDayFacts() classDayFacts {
	return classDayFacts{
		statuses:           map[int64]string{},
		statusReportedAt:   map[int64]time.Time{},
		departures:         map[int64]string{},
		arrivals:           map[int64]string{},
		pickups:            map[int64]string{},
		pickupRegular:      map[int64]string{},
		pickupChanged:      map[int64]bool{},
		pickupChangedAt:    map[int64]time.Time{},
		arrivalCancelledAt: map[int64]time.Time{},
		notScheduled:       map[int64]bool{},
	}
}

// classDayWeekdayKey maps a calendar date onto the report day keys
// ("mon".."fri"). Weekend dates return "".
func classDayWeekdayKey(date timezone.Date) string {
	switch date.Weekday() {
	case time.Monday:
		return "mon"
	case time.Tuesday:
		return "tue"
	case time.Wednesday:
		return "wed"
	case time.Thursday:
		return "thu"
	case time.Friday:
		return "fri"
	default:
		return ""
	}
}

// classDay builds the read-only per-class day view for the Lehrkraft
// handoff (#1772): the full class (including students without any
// enrollment), per student whether they stay in care on that day, which
// offerings apply, the arrival/pickup times, and the departure plan. Caller
// scoping (which classes the account may see) happens before; this method
// only knows the class name. Every call records the access in the GDPR log.
func (s *dayReports) classDay(ctx context.Context, schoolClass string, date timezone.Date, actor classday.Actor) (*classday.DayReport, error) {
	schoolClass = strings.TrimSpace(schoolClass)
	if schoolClass == "" {
		return nil, fmt.Errorf("class day report: school class required: %w", errInvalidFilter)
	}
	// Fail fast instead of degrading: a partially wired service would serve
	// a sheet where a sick or abgemeldetes Kind shows as "bleibt in der
	// Betreuung" — the exact failure this view exists to prevent.
	if s.statusDays == nil || s.careDays == nil || s.times == nil {
		return nil, fmt.Errorf("class day report: status/schedule dependencies not configured")
	}
	roster, err := s.rosters.ClassRosterDay(ctx, schoolClass, date)
	if err != nil {
		return nil, err
	}
	weekday := classDayWeekdayKey(date)
	facts := newClassDayFacts()
	if weekday != "" {
		// Weekends render "Kein Schultag": the status, departure and schedule
		// enrichment is skipped, only the roster (names + count) is served.
		if facts, err = s.classDaySchoolDayFacts(ctx, roster, date, weekday); err != nil {
			return nil, err
		}
	}
	report := buildClassDayReport(schoolClass, date, strings.Join(roster.PhaseNames, ", "), roster.Rows, facts)
	report.EnrollmentKnown = len(roster.PhaseNames) > 0
	if weekday != "" {
		if report.ClassArrivalException, err = s.classDayArrivalException(ctx, schoolClass, date); err != nil {
			return nil, err
		}
	}
	if !report.EnrollmentKnown {
		// Without a covering phase the stays/leaves split is unknowable —
		// zero the counters so no consumer can print "alle gehen nach Hause".
		report.Totals.Staying = 0
		report.Totals.Leaving = 0
	}
	if err := s.recordClassDayViewAudit(ctx, report, actor.AccountID, actor.Roles); err != nil {
		return nil, err
	}
	return report, nil
}

// classDaySchoolDayFacts loads the day facts of a school day: statuses,
// departures, effective times and the "kommt heute nicht" cancellations.
func (s *dayReports) classDaySchoolDayFacts(ctx context.Context, roster *DayRoster, date timezone.Date, weekday string) (classDayFacts, error) {
	facts := newClassDayFacts()
	studentIDs := make([]int64, 0, len(roster.Rows))
	for _, row := range roster.Rows {
		if row.StudentID > 0 {
			studentIDs = append(studentIDs, row.StudentID)
		}
	}
	var err error
	if facts.statuses, facts.statusReportedAt, err = s.classDayStatuses(ctx, studentIDs, date); err != nil {
		return facts, err
	}
	if facts.departures, err = s.classDayDepartures(ctx, roster.Students, weekday); err != nil {
		return facts, err
	}
	if err := s.classDayEffectiveTimes(ctx, studentIDs, date, &facts); err != nil {
		return facts, err
	}
	cancelled, err := s.classDayCancellations(ctx, studentIDs, date, &facts)
	if err != nil {
		return facts, err
	}
	for studentID := range cancelled {
		// A stronger reported status (sick / class trip / excused) keeps
		// precedence over the plain "kommt heute nicht" cancellation.
		if facts.statuses[studentID] != "" {
			continue
		}
		facts.statuses[studentID] = studentStatusDayCancelled
		// The cancellation is a timeless day exception; its creation time is
		// when the Abmeldung became known (#2294). Arrival exceptions win
		// over pickup exceptions in the day planning.
		if stamp, ok := facts.arrivalCancelledAt[studentID]; ok {
			facts.statusReportedAt[studentID] = stamp
		} else if stamp, ok := facts.pickupChangedAt[studentID]; ok {
			facts.statusReportedAt[studentID] = stamp
		}
	}
	return facts, nil
}

// classDayArrivalException loads the class-wide arrival day exception of the
// date (#2962), nil when there is none. It is read from the exception rows
// rather than derived from the children's effective times: the line belongs
// to the class, and it stays visible on a day no enrolled child has care.
func (s *dayReports) classDayArrivalException(ctx context.Context, schoolClass string, date timezone.Date) (*classday.DayArrivalException, error) {
	if s.classExceptions == nil {
		return nil, nil
	}
	rows, err := s.classExceptions.ListClassArrivalExceptions(ctx, schoolClass, date, date)
	if err != nil {
		// A deployment without the exception repository still serves the
		// sheet; the exception line is an extra, not the report.
		if errors.Is(err, ports.ErrClassArrivalExceptionsNotConfigured) {
			return nil, nil
		}
		return nil, fmt.Errorf("class day report: load class arrival exception: %w", err)
	}
	for _, row := range rows {
		if row.Date != date {
			continue
		}
		out := &classday.DayArrivalException{
			ArrivalTime: row.ArrivalTime.Format("15:04"),
			Origin:      row.Origin,
		}
		if out.Origin == "" {
			out.Origin = classday.ArrivalExceptionOriginOGS
		}
		if row.Reason != nil {
			out.Reason = strings.TrimSpace(*row.Reason)
		}
		return out, nil
	}
	return nil, nil
}

// recordClassDayViewAudit writes the GDPR access log for a class day view:
// care and departure data of an entire class served to one account, under
// its own resource type and the caller's actual roles.
//
// Deduplicated to one row per actor, class, report date and calendar day of
// access: the view revalidates itself (interval + tab focus), and a second
// identical row every few minutes would bloat the append-only table and
// destroy the log's evidential value. A view on a LATER calendar day writes
// again — "looked at it again the next day" stays auditable.
func (s *dayReports) recordClassDayViewAudit(ctx context.Context, report *classday.DayReport, actorAccountID int64, actorRole string) error {
	if s.accessLog == nil {
		return nil
	}
	const errPrefix = "class day view audit"
	date := timezone.Date(report.Date)
	seen, err := s.accessLog.ClassDayViewSeenSince(ctx, actorAccountID, report.SchoolClass, date.String(), timezone.TodayDate().BerlinMidnight())
	if err != nil {
		return fmt.Errorf("class day view audit dedupe: %w", err)
	}
	if seen {
		return nil
	}
	entry, err := newAccessRecord(errPrefix, actorAccountID, actorRole, date.BerlinMidnight(), date.EndOfDay(), time.Now())
	if err != nil {
		return err
	}
	entry.Metadata = map[string]any{
		"report":        "class_day",
		"school_class":  report.SchoolClass,
		"date":          date.String(),
		"student_count": report.Totals.Students,
	}
	if err := s.accessLog.RecordClassDayView(ctx, entry); err != nil {
		return fmt.Errorf("%s write: %w", errPrefix, err)
	}
	return nil
}

// newAccessRecord validates the acting account and defaults the role (the
// actor_role column is NOT NULL and never carries an empty string).
func newAccessRecord(errPrefix string, actorAccountID int64, actorRole string, rangeStart, rangeEnd, accessedAt time.Time) (AccessRecord, error) {
	if actorAccountID <= 0 {
		return AccessRecord{}, fmt.Errorf("%s: actor account id required", errPrefix)
	}
	if strings.TrimSpace(actorRole) == "" {
		actorRole = "unknown"
	}
	return AccessRecord{
		ActorAccountID: actorAccountID,
		ActorRole:      actorRole,
		RangeStart:     rangeStart,
		RangeEnd:       rangeEnd,
		AccessedAt:     accessedAt,
	}, nil
}
