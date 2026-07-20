package schedule

import (
	"context"

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
// reports unknown — with one exception: an absence a broad day status (sick /
// excused / class trip) wrote and still owns. ApplyStatusDay stamps every
// expected row of the day, including days the child was never booked into care,
// and keeps its provenance in student_status_day_id (a check-in and a manual
// PATCH both clear that column). Until the block ends and MarkNotScheduled
// undoes it, such a row is a false absence, so it keeps the non-booking
// verdict. A cancelled care day WAS booked, so its absence is real and stays
// one, and a manual absence is never relabelled.
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
		if row.StudentStatusDayID != nil && planVerdict == CareDayNotScheduled {
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
	// Costs the same four queries as ResolveForDate regardless of window
	// length: weekly plans are recurring, so they are loaded once and combined
	// with the window's exceptions in memory.
	ResolveForRange(ctx context.Context, studentIDs []int64, from, to timezone.Date) (map[int64]map[timezone.Date]CareDayStatus, error)
}

// CareDayDependencies carries the four repositories the derivation reads.
type CareDayDependencies struct {
	ArrivalSchedules  schedule.StudentArrivalScheduleRepository
	ArrivalExceptions schedule.StudentArrivalExceptionRepository
	PickupSchedules   schedule.StudentPickupScheduleRepository
	PickupExceptions  schedule.StudentPickupExceptionRepository
}

type careDayService struct {
	deps CareDayDependencies
}

// NewCareDayService builds the care-day derivation.
func NewCareDayService(deps CareDayDependencies) CareDayService {
	if deps.ArrivalSchedules == nil || deps.ArrivalExceptions == nil ||
		deps.PickupSchedules == nil || deps.PickupExceptions == nil {
		panic("schedule.NewCareDayService: required dependency is nil")
	}
	return &careDayService{deps: deps}
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
	return out, nil
}

// carePlans is the in-memory projection every date lookup reads from.
type carePlans struct {
	// arrivalByStudentWeekday / pickupByStudentWeekday hold the recurring
	// weekly plan; hasPlan records that a child has ANY plan row, which is what
	// separates "not booked today" from "no plan on file".
	arrivalByStudentWeekday map[int64]map[int]*schedule.StudentArrivalSchedule
	pickupByStudentWeekday  map[int64]map[int]*schedule.StudentPickupSchedule
	hasPlan                 map[int64]bool

	arrivalExceptions map[int64]map[timezone.Date]*schedule.StudentArrivalException
	pickupExceptions  map[int64]map[timezone.Date]*schedule.StudentPickupException
}

func (s *careDayService) loadCarePlans(
	ctx context.Context, studentIDs []int64, from, to timezone.Date,
) (*carePlans, error) {
	plans := &carePlans{
		arrivalByStudentWeekday: map[int64]map[int]*schedule.StudentArrivalSchedule{},
		pickupByStudentWeekday:  map[int64]map[int]*schedule.StudentPickupSchedule{},
		hasPlan:                 map[int64]bool{},
		arrivalExceptions:       map[int64]map[timezone.Date]*schedule.StudentArrivalException{},
		pickupExceptions:        map[int64]map[timezone.Date]*schedule.StudentPickupException{},
	}

	arrivals, err := s.deps.ArrivalSchedules.FindByStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, &ScheduleError{Op: opResolveCareDay, Err: err}
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
		plans.hasPlan[row.StudentID] = true
	}

	pickups, err := s.deps.PickupSchedules.FindByStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, &ScheduleError{Op: opResolveCareDay, Err: err}
	}
	for _, row := range pickups {
		if row == nil {
			continue
		}
		byWeekday, ok := plans.pickupByStudentWeekday[row.StudentID]
		if !ok {
			byWeekday = map[int]*schedule.StudentPickupSchedule{}
			plans.pickupByStudentWeekday[row.StudentID] = byWeekday
		}
		byWeekday[row.Weekday] = row
		plans.hasPlan[row.StudentID] = true
	}

	arrivalExceptions, err := s.deps.ArrivalExceptions.FindByStudentIDsAndDateRange(ctx, studentIDs, from, to)
	if err != nil {
		return nil, &ScheduleError{Op: opResolveCareDay, Err: err}
	}
	for _, exc := range arrivalExceptions {
		if exc == nil {
			continue
		}
		byDate, ok := plans.arrivalExceptions[exc.StudentID]
		if !ok {
			byDate = map[timezone.Date]*schedule.StudentArrivalException{}
			plans.arrivalExceptions[exc.StudentID] = byDate
		}
		byDate[exc.ExceptionDate] = exc
	}

	pickupExceptions, err := s.deps.PickupExceptions.FindByStudentIDsAndDateRange(ctx, studentIDs, from, to)
	if err != nil {
		return nil, &ScheduleError{Op: opResolveCareDay, Err: err}
	}
	for _, exc := range pickupExceptions {
		if exc == nil {
			continue
		}
		byDate, ok := plans.pickupExceptions[exc.StudentID]
		if !ok {
			byDate = map[timezone.Date]*schedule.StudentPickupException{}
			plans.pickupExceptions[exc.StudentID] = byDate
		}
		byDate[exc.ExceptionDate] = exc
	}

	return plans, nil
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
	case decision.Reason == DayPlanningReasonNoPlan && !p.hasPlan[studentID]:
		return CareDayUnknown
	default:
		// The plan covers other weekdays, just not this one.
		return CareDayNotScheduled
	}
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
		return result
	}
	// Weekends carry no weekly rows; only an exception can put a child there.
	if weekday > schedule.WeekdayFriday {
		return result
	}
	if sched, ok := p.arrivalByStudentWeekday[studentID][weekday]; ok && sched != nil {
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
	if sched, ok := p.pickupByStudentWeekday[studentID][weekday]; ok && sched != nil {
		pickup := sched.PickupTime
		result.PickupTime = &pickup
	}
	return result
}
