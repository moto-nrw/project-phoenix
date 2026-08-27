package schedule

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
)

// Care-day derivation (#1747).
//
// A child assigned to a timetable block is not automatically expected on every
// occurrence of that block: the assignment says "belongs to this activity", the
// care plan (Betreuungsplan) says "is at the OGS on this weekday". Assigning a
// whole group or year to an activity — possible since #1838 — makes the gap
// visible: every member shows up as expected on every weekday, including the
// days they are not booked for.
//
// This derivation intersects the two. It writes nothing: the assignment rows in
// schedule.instance_students stay untouched, so a care-plan change by the
// parents takes effect on the next read without re-materializing anything.
//
// The decision itself is NOT re-implemented here. ResolveDayPlanning
// (day_planning.go) already owns the precedence used by the student search, and
// a second set of rules would drift from it. This file only assembles its
// inputs in bulk and maps its outcome onto the tri-state below.

// opResolveCareDay names this derivation in ScheduleError.
const opResolveCareDay = "resolve care day"

// CareDayStatus is the derived per-child, per-day care-plan verdict.
type CareDayStatus string

const (
	// CareDayScheduled — the care plan puts the child in the OGS that day.
	CareDayScheduled CareDayStatus = "scheduled"

	// CareDayNotScheduled — the child is not booked into care that day: no
	// arrival and no pickup plans the weekday, and nobody said otherwise. Not
	// expected, and never an absence — there was no care to miss.
	CareDayNotScheduled CareDayStatus = "not_scheduled"

	// CareDayCancelled — somebody explicitly cancelled the day for this child
	// ("Kommt heute nicht": a same-day arrival or pickup exception without a
	// time). Not expected either, but this is a REPORTED ABSENCE, not a
	// non-booking: it must still be stamped absent when the block ends so the
	// attendance history and the exports keep showing it (#1747 review).
	CareDayCancelled CareDayStatus = "cancelled"

	// CareDayUnknown — no care plan on file at all, so the plan cannot say
	// anything about that day. Treated as expected: schools that do not
	// maintain arrival/pickup plans must keep seeing their full roster, and a
	// child must never disappear because data is missing.
	CareDayUnknown CareDayStatus = "unknown"
)

// Expected reports whether a child with this status belongs in the expected
// count. Both "not booked" and "cancelled" say the child is not coming; only a
// missing or affirmative plan keeps them in.
func (s CareDayStatus) Expected() bool {
	return s != CareDayNotScheduled && s != CareDayCancelled
}

// ExemptFromAbsence reports whether ending a block may skip the
// expected → absent stamp for this child.
//
// ONLY a non-booking qualifies. A cancellation looks the same on the planner
// (not expected), but writing no row would erase the absence from the
// attendance history and the exports — the day WAS booked and the child did
// not come, which is exactly what an absence records.
func (s CareDayStatus) ExemptFromAbsence() bool { return s == CareDayNotScheduled }

// AttendanceRowCareDay maps one timetable attendance row onto the care-day
// verdict every reader groups, counts, and renders by (#1747 review). It is the
// single implementation behind the planner list (api/timetable), the operation
// roster, and the planned-now cards, so those three can never disagree about
// the same child.
//
// planVerdict is what the derivation says about that child on the instance's
// date; the empty string means "not resolved" and reads as unknown — a missing
// fact never excludes a child.
//
// On a completed instance the verdict is frozen: ending the block wrote the
// absences and stamped not_scheduled on the children it spared, so that column
// IS the verdict. Reading the current care plan there would let a later plan
// edit relabel a finished day.
//
// A row somebody set by hand (ManualStatusAt) reports unknown regardless of
// what the plan says: staff setting an unbooked slot back to 'expected' means
// "the plan is wrong, this child is coming", and the planner must show that
// instead of the derivation it overrides (#1747 review).
//
// A row that already carries a real attendance status tells its own story and
// reports unknown — with two exceptions that still count as non-bookings when
// the plan says the child was never expected:
//
//  1. An absence a broad day status (sick / excused / class trip) wrote and
//     still owns via student_status_day_id. ApplyStatusDay stamps every
//     expected row of the day, including days the child was never booked into
//     care (a check-in and a manual PATCH both clear that column).
//  2. An absence a partial-day excusal wrote and still owns via
//     pickup_exception_id. ApplyPartialAbsence does the same for blocks that
//     start at or after the cutoff; without recognizing that provenance the
//     planner treats the active absence as real and under-counts non-scheduled
//     children until session end.
//
// Until the block ends and MarkNotScheduled undoes it, such a row is a false
// absence, so it keeps the non-booking verdict. A cancelled care day WAS
// booked, so its absence is real and stays one, and a manual absence is never
// relabelled.
func AttendanceRowCareDay(instanceCompleted bool, row *schedule.InstanceStudent, planVerdict CareDayStatus) CareDayStatus {
	if row == nil {
		return CareDayUnknown
	}
	if row.ManualStatusAt != nil {
		return CareDayUnknown
	}
	if instanceCompleted {
		if row.NotScheduled && row.Status == schedule.AttendanceStatusExpected {
			return CareDayNotScheduled
		}
		return CareDayUnknown
	}
	if row.Status != schedule.AttendanceStatusExpected {
		// Plan-owned absences on a not-scheduled day remain non-bookings while
		// the block is active. Manual status already returned above.
		if planVerdict == CareDayNotScheduled &&
			(row.StudentStatusDayID != nil || row.PickupExceptionID != nil) {
			return CareDayNotScheduled
		}
		return CareDayUnknown
	}
	if planVerdict != "" {
		return planVerdict
	}
	return CareDayUnknown
}

// CareDayService derives care-day status for many children at once.
type CareDayService interface {
	// ResolveForDate returns the care-day status of every requested child on
	// one date. Children without an entry in the result are unknown.
	ResolveForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]CareDayStatus, error)

	// ResolveForRange returns the care-day status of every requested child for
	// every date in the inclusive [from, to] window, keyed student → date.
	// Query count stays constant regardless of window length: recurring staff
	// plans, booking-derived pickup baselines and exceptions are batch-loaded
	// and combined in memory.
	ResolveForRange(ctx context.Context, studentIDs []int64, from, to timezone.Date) (map[int64]map[timezone.Date]CareDayStatus, error)
}

// CareDayDependencies carries the read boundaries the derivation uses.
type CareDayDependencies struct {
	// ArrivalBaselines resolves the arrival plan the same way every other
	// reader sees it (#2414): with the booking mode on, a stale row on an
	// unbooked weekday plans nothing. Without it the resolver would keep
	// marking a deregistered child as expected — the 19.08. incident.
	ArrivalBaselines  ArrivalBaselineReader
	ArrivalSchedules  schedule.StudentArrivalScheduleRepository
	ArrivalExceptions schedule.StudentArrivalExceptionRepository
	PickupBaselines   PickupBaselineReader
	PickupExceptions  schedule.StudentPickupExceptionRepository
	CareParticipation CareParticipationResolver
}

type CareParticipationResolver interface {
	ParticipatingStudentIDsByDate(ctx context.Context, studentIDs []int64, from, to timezone.Date) (map[timezone.Date]map[int64]bool, error)
}

type careDayService struct {
	deps CareDayDependencies
}

// NewCareDayService builds the care-day derivation.
func NewCareDayService(deps CareDayDependencies) CareDayService {
	if deps.ArrivalSchedules == nil || deps.ArrivalExceptions == nil ||
		deps.PickupBaselines == nil || deps.PickupExceptions == nil {
		panic("schedule.NewCareDayService: required dependency is nil")
	}
	return &careDayService{deps: deps}
}

// WireCareParticipation attaches the lifecycle after both services exist in
// the factory. Care-day construction happens earlier because CareLifecycle
// itself depends on schedule repositories.
func WireCareParticipation(service CareDayService, resolver CareParticipationResolver) {
	concrete, ok := service.(*careDayService)
	if !ok {
		panic("schedule care-day service does not support participation wiring")
	}
	concrete.deps.CareParticipation = resolver
}

func (s *careDayService) ResolveForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]CareDayStatus, error) {
	byStudent, err := s.ResolveForRange(ctx, studentIDs, date, date)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]CareDayStatus, len(byStudent))
	for studentID, byDate := range byStudent {
		out[studentID] = byDate[date]
	}
	return out, nil
}

func (s *careDayService) ResolveForRange(
	ctx context.Context, studentIDs []int64, from, to timezone.Date,
) (map[int64]map[timezone.Date]CareDayStatus, error) {
	out := make(map[int64]map[timezone.Date]CareDayStatus, len(studentIDs))
	if len(studentIDs) == 0 || to.Before(from) {
		return out, nil
	}

	plans, err := s.loadCarePlans(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}

	for _, studentID := range studentIDs {
		byDate := make(map[timezone.Date]CareDayStatus)
		for date := from; !date.After(to); date = date.AddDays(1) {
			byDate[date] = plans.statusFor(studentID, date)
		}
		out[studentID] = byDate
	}
	if err := s.applyCareParticipation(ctx, out, studentIDs, from, to); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *careDayService) applyCareParticipation(
	ctx context.Context, out map[int64]map[timezone.Date]CareDayStatus,
	studentIDs []int64, from, to timezone.Date,
) error {
	if s.deps.CareParticipation == nil {
		return &ScheduleError{Op: opResolveCareDay, Err: errors.New("care participation resolver is not configured")}
	}
	participatingByDate, err := s.deps.CareParticipation.ParticipatingStudentIDsByDate(ctx, studentIDs, from, to)
	if err != nil {
		return &ScheduleError{Op: opResolveCareDay, Err: err}
	}
	for date := from; !date.After(to); date = date.AddDays(1) {
		participating := participatingByDate[date]
		for _, studentID := range studentIDs {
			if !participating[studentID] {
				out[studentID][date] = CareDayNotScheduled
			}
		}
	}
	return nil
}

// carePlans is the in-memory projection every date lookup reads from.
type carePlans struct {
	// The arrival plan is undated. Pickup baselines can change within the
	// range when booking validity changes, so they are keyed by date.
	arrivalByStudentWeekday map[int64]map[int]*schedule.StudentArrivalSchedule
	arrivalByStudentDate    map[int64]map[timezone.Date]*schedule.StudentArrivalSchedule
	pickupByStudentDate     map[int64]map[timezone.Date]*schedule.StudentPickupSchedule
	hasPlan                 map[int64]map[timezone.Date]bool
	bookingsAuthoritative   bool

	arrivalExceptions map[int64]map[timezone.Date]*schedule.StudentArrivalException
	pickupExceptions  map[int64]map[timezone.Date]*schedule.StudentPickupException
}

func (s *careDayService) loadCarePlans(
	ctx context.Context, studentIDs []int64, from, to timezone.Date,
) (*carePlans, error) {
	plans := newCarePlans()
	if err := s.loadArrivalPlans(ctx, plans, studentIDs, from, to); err != nil {
		return nil, err
	}
	if err := s.loadPickupPlans(ctx, plans, studentIDs, from, to); err != nil {
		return nil, err
	}
	if err := s.loadCareExceptions(ctx, plans, studentIDs, from, to); err != nil {
		return nil, err
	}
	return plans, nil
}

func newCarePlans() *carePlans {
	return &carePlans{
		arrivalByStudentWeekday: map[int64]map[int]*schedule.StudentArrivalSchedule{},
		arrivalByStudentDate:    map[int64]map[timezone.Date]*schedule.StudentArrivalSchedule{},
		pickupByStudentDate:     map[int64]map[timezone.Date]*schedule.StudentPickupSchedule{},
		hasPlan:                 map[int64]map[timezone.Date]bool{},
		arrivalExceptions:       map[int64]map[timezone.Date]*schedule.StudentArrivalException{},
		pickupExceptions:        map[int64]map[timezone.Date]*schedule.StudentPickupException{},
	}
}

func (s *careDayService) loadArrivalPlans(
	ctx context.Context,
	plans *carePlans,
	studentIDs []int64,
	from, to timezone.Date,
) error {
	if s.deps.ArrivalBaselines == nil {
		return s.loadStoredArrivalPlans(ctx, plans, studentIDs, from, to)
	}
	arrivals, err := s.deps.ArrivalBaselines.Project(ctx, studentIDs, from, to)
	if err != nil {
		return &ScheduleError{Op: opResolveCareDay, Err: err}
	}
	plans.bookingsAuthoritative = arrivals.BookingsAuthoritative
	for _, studentID := range studentIDs {
		for date := from; !date.After(to); date = date.AddDays(1) {
			if arrivals.HasPlan(studentID, date) {
				plans.markHasPlan(studentID, date)
			}
			row := arrivals.ForDate(studentID, date)
			if row == nil {
				continue
			}
			if plans.arrivalByStudentDate[studentID] == nil {
				plans.arrivalByStudentDate[studentID] = make(map[timezone.Date]*schedule.StudentArrivalSchedule)
			}
			plans.arrivalByStudentDate[studentID][date] = row
		}
	}
	return nil
}

// loadStoredArrivalPlans is the pre-#2414 path, kept for callers wired without
// a baseline reader (CLI, older tests).
func (s *careDayService) loadStoredArrivalPlans(
	ctx context.Context,
	plans *carePlans,
	studentIDs []int64,
	from, to timezone.Date,
) error {
	arrivals, err := s.deps.ArrivalSchedules.FindByStudentIDs(ctx, studentIDs)
	if err != nil {
		return &ScheduleError{Op: opResolveCareDay, Err: err}
	}
	for _, row := range arrivals {
		if row == nil {
			continue
		}
		byWeekday, ok := plans.arrivalByStudentWeekday[row.StudentID]
		if !ok {
			byWeekday = map[int]*schedule.StudentArrivalSchedule{}
			plans.arrivalByStudentWeekday[row.StudentID] = byWeekday
		}
		byWeekday[row.Weekday] = row
		for date := from; !date.After(to); date = date.AddDays(1) {
			plans.markHasPlan(row.StudentID, date)
		}
	}
	return nil
}

func (s *careDayService) loadPickupPlans(
	ctx context.Context,
	plans *carePlans,
	studentIDs []int64,
	from, to timezone.Date,
) error {
	pickups, err := s.deps.PickupBaselines.Project(ctx, studentIDs, from, to)
	if err != nil {
		return &ScheduleError{Op: opResolveCareDay, Err: err}
	}
	for _, studentID := range studentIDs {
		for date := from; !date.After(to); date = date.AddDays(1) {
			if pickups.HasPlan(studentID, date) {
				plans.markHasPlan(studentID, date)
			}
			row := pickups.ForDate(studentID, date)
			if row == nil {
				continue
			}
			if plans.pickupByStudentDate[studentID] == nil {
				plans.pickupByStudentDate[studentID] = make(map[timezone.Date]*schedule.StudentPickupSchedule)
			}
			plans.pickupByStudentDate[studentID][date] = row
		}
	}
	return nil
}

func (s *careDayService) loadCareExceptions(
	ctx context.Context,
	plans *carePlans,
	studentIDs []int64,
	from, to timezone.Date,
) error {
	arrivalExceptions, err := s.deps.ArrivalExceptions.FindByStudentIDsAndDateRange(ctx, studentIDs, from, to)
	if err != nil {
		return &ScheduleError{Op: opResolveCareDay, Err: err}
	}
	for _, exc := range arrivalExceptions {
		plans.addArrivalException(exc)
	}

	pickupExceptions, err := s.deps.PickupExceptions.FindByStudentIDsAndDateRange(ctx, studentIDs, from, to)
	if err != nil {
		return &ScheduleError{Op: opResolveCareDay, Err: err}
	}
	for _, exc := range pickupExceptions {
		plans.addPickupException(exc)
	}
	return nil
}

func (p *carePlans) addArrivalException(row *schedule.StudentArrivalException) {
	if row == nil {
		return
	}
	if p.arrivalExceptions[row.StudentID] == nil {
		p.arrivalExceptions[row.StudentID] = make(map[timezone.Date]*schedule.StudentArrivalException)
	}
	p.arrivalExceptions[row.StudentID][row.ExceptionDate] = row
}

func (p *carePlans) addPickupException(row *schedule.StudentPickupException) {
	if row == nil {
		return
	}
	if p.pickupExceptions[row.StudentID] == nil {
		p.pickupExceptions[row.StudentID] = make(map[timezone.Date]*schedule.StudentPickupException)
	}
	p.pickupExceptions[row.StudentID][row.ExceptionDate] = row
}

func (p *carePlans) markHasPlan(studentID int64, date timezone.Date) {
	if p.hasPlan[studentID] == nil {
		p.hasPlan[studentID] = make(map[timezone.Date]bool)
	}
	p.hasPlan[studentID][date] = true
}

// statusFor derives one child's status on one date.
//
// The inputs deliberately carry only the care-plan facts: HasActualAttendance,
// the absence flags, and HasTimetable stay false. Attendance and reported
// absences are owned by the roster (visits and active.student_status_days), and
// feeding HasTimetable would make every assigned child trivially "comes today",
// which is the very question this derivation exists to answer.
//
// Consumers that DO resolve a timetable signal must gate it on this verdict
// instead of passing the raw assignment: api/students/day_planning.go drops a
// child's HasTimetable on a not_scheduled day for exactly that reason. Skipping
// it makes the student search report "kommt heute" for a child the timetable
// roster and the expected counts leave out.
func (p *carePlans) statusFor(studentID int64, date timezone.Date) CareDayStatus {
	// With authoritative bookings, the arrival projection's dated row is the
	// positive care-day signal. Pickup schedules and one-day exceptions may add
	// details to that day, but cannot create a day the child did not book.
	if p.bookingsAuthoritative && p.arrivalByStudentDate[studentID][date] == nil {
		return CareDayNotScheduled
	}

	// The whole-day cancellation rule (a timeless "Kommt heute nicht"
	// exception on either leg cancels the day, whatever the other leg says)
	// lives INSIDE ResolveDayPlanning, so the student search, the parent
	// portal, and this derivation can never disagree on the same child and
	// date. Do not re-implement any precedence here.
	decision := ResolveDayPlanning(DayPlanningInputs{
		Arrival: p.effectiveArrival(studentID, date),
		Pickup:  p.effectivePickup(studentID, date),
	})

	switch {
	case decision.ComesToday:
		return CareDayScheduled
	case decision.Reason == DayPlanningReasonArrivalException ||
		decision.Reason == DayPlanningReasonPickupException:
		// Not coming AND an exception reason: the only path to that pair is the
		// cancellation branch of ResolveDayPlanning. Somebody stated the child
		// is out today, which is an absence to record, not a missing booking.
		return CareDayCancelled
	case decision.Reason == DayPlanningReasonNoPlan && !p.hasPlan[studentID][date]:
		return CareDayUnknown
	case p.hasPlan[studentID][date] && p.hasArrivalSchedule(studentID, date):
		// A care day remains scheduled even when its own and its class arrival
		// time are both absent. The missing wall-clock time is not an absence of
		// the care-day booking.
		return CareDayScheduled
	default:
		// The plan covers other weekdays, just not this one.
		return CareDayNotScheduled
	}
}

func (p *carePlans) hasArrivalSchedule(studentID int64, date timezone.Date) bool {
	if p.arrivalByStudentDate[studentID][date] != nil {
		return true
	}
	return p.arrivalByStudentWeekday[studentID][isoWeekday(date)] != nil
}

// effectiveArrival mirrors the exception-beats-schedule merge of
// GetBulkEffectiveArrivalTimesForDate for one day, minus the note loading the
// derivation has no use for.
func (p *carePlans) effectiveArrival(studentID int64, date timezone.Date) *EffectiveArrivalTime {
	weekday := isoWeekday(date)
	result := &EffectiveArrivalTime{Date: date, WeekdayName: schedule.WeekdayNames[weekday]}

	if exc, ok := p.arrivalExceptions[studentID][date]; ok {
		result.IsException = true
		result.ArrivalTime = exc.ExpectedArrival
		if !exc.CreatedAt.IsZero() {
			recorded := exc.CreatedAt
			result.ChangedAt = &recorded
		}
		return result
	}
	// Weekends carry no weekly rows; only an exception can put a child there.
	if weekday > schedule.WeekdayFriday {
		return result
	}
	sched := p.arrivalByStudentDate[studentID][date]
	if sched == nil {
		sched = p.arrivalByStudentWeekday[studentID][weekday]
	}
	// A care day whose class carries no time has no arrival time. Copying the
	// zero value here would render as 00:00 everywhere (#2414).
	if sched != nil && !sched.ExpectedArrival.IsZero() {
		arrival := sched.ExpectedArrival
		result.ArrivalTime = &arrival
	}
	return result
}

// effectivePickup is the pickup-side mirror of effectiveArrival.
func (p *carePlans) effectivePickup(studentID int64, date timezone.Date) *EffectivePickupTime {
	weekday := isoWeekday(date)
	result := &EffectivePickupTime{Date: date, WeekdayName: schedule.WeekdayNames[weekday]}

	if exc, ok := p.pickupExceptions[studentID][date]; ok {
		result.IsException = true
		result.PickupTime = exc.PickupTime
		return result
	}
	if weekday > schedule.WeekdayFriday {
		return result
	}
	if sched, ok := p.pickupByStudentDate[studentID][date]; ok && sched != nil {
		pickup := sched.PickupTime
		result.PickupTime = &pickup
	}
	return result
}
