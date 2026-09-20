package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

// opResolveCareDay names this derivation in careplan.ScheduleError.
const opResolveCareDay = "resolve care day"

// CareDayDependencies carries the read boundaries the derivation uses.
type CareDayDependencies struct {
	// ArrivalBaselines resolves the arrival plan the same way every other
	// reader sees it (#2414): with the booking mode on, a stale row on an
	// unbooked weekday plans nothing. Without it the resolver would keep
	// marking a deregistered child as expected — the 19.08. incident.
	ArrivalBaselines  careplan.ArrivalBaselineReader
	Records           CareDayRecords
	PickupBaselines   careplan.PickupBaselineReader
	CareParticipation CareParticipationResolver
}

type CareDayRecords = ports.CareDayRecords

type CareParticipationResolver = ports.CareParticipationResolver

type careDayService struct {
	deps CareDayDependencies
}

// NewCareDays builds the care-day derivation.
func NewCareDays(deps CareDayDependencies) careplan.CareDayQuery {
	if deps.Records == nil || deps.PickupBaselines == nil {
		panic("care plan care-day queries: records and pickup baselines are required")
	}
	return &careDayService{deps: deps}
}

// WireCareParticipation attaches the lifecycle after both services exist in
// the factory. Care-day construction happens earlier because CareLifecycle
// itself depends on schedule repositories.
func WireCareParticipation(service careplan.CareDayQuery, resolver CareParticipationResolver) {
	concrete, ok := service.(*careDayService)
	if !ok {
		panic("schedule care-day service does not support participation wiring")
	}
	concrete.deps.CareParticipation = resolver
}

func (s *careDayService) ResolveForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]careplan.CareDayStatus, error) {
	return NewCareDayQueries(s).ResolveForDate(ctx, studentIDs, date)
}

func (s *careDayService) ResolveForRange(ctx context.Context, studentIDs []int64, from, to timezone.Date) (map[int64]map[timezone.Date]careplan.CareDayStatus, error) {
	return NewCareDayQueries(s).ResolveForRange(ctx, studentIDs, from, to)
}

func (s *careDayService) LoadCareDayFacts(ctx context.Context, studentIDs []int64, from, to timezone.Date) (map[int64]map[timezone.Date]careplan.CareDayFacts, error) {
	plans, err := s.loadCarePlans(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]map[timezone.Date]careplan.CareDayFacts, len(studentIDs))
	for _, id := range studentIDs {
		days := make(map[timezone.Date]careplan.CareDayFacts)
		for date := from; !date.After(to); date = date.AddDays(1) {
			days[date] = plans.factsFor(id, date)
		}
		result[id] = days
	}
	return result, nil
}

func (s *careDayService) ParticipatingStudentIDsByDate(ctx context.Context, studentIDs []int64, from, to timezone.Date) (map[timezone.Date]map[int64]bool, error) {
	if s.deps.CareParticipation == nil {
		return nil, &careplan.ScheduleError{Op: opResolveCareDay, Err: errors.New("care participation resolver is not configured")}
	}
	result, err := s.deps.CareParticipation.ParticipatingStudentIDsByDate(ctx, studentIDs, from, to)
	if err != nil {
		return nil, &careplan.ScheduleError{Op: opResolveCareDay, Err: err}
	}
	return result, nil
}

// carePlans is the in-memory projection every date lookup reads from.
type carePlans struct {
	// The arrival plan is undated. Pickup baselines can change within the
	// range when booking validity changes, so they are keyed by date.
	arrivalByStudentWeekday map[int64]map[int]*careplan.ArrivalSchedule
	arrivalByStudentDate    map[int64]map[timezone.Date]*careplan.ArrivalSchedule
	pickupByStudentDate     map[int64]map[timezone.Date]*careplan.PickupSchedule
	hasPlan                 map[int64]map[timezone.Date]bool
	bookingsAuthoritative   bool

	arrivalExceptions map[int64]map[timezone.Date]*careplan.ArrivalException
	pickupExceptions  map[int64]map[timezone.Date]*careplan.PickupException
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
		arrivalByStudentWeekday: map[int64]map[int]*careplan.ArrivalSchedule{},
		arrivalByStudentDate:    map[int64]map[timezone.Date]*careplan.ArrivalSchedule{},
		pickupByStudentDate:     map[int64]map[timezone.Date]*careplan.PickupSchedule{},
		hasPlan:                 map[int64]map[timezone.Date]bool{},
		arrivalExceptions:       map[int64]map[timezone.Date]*careplan.ArrivalException{},
		pickupExceptions:        map[int64]map[timezone.Date]*careplan.PickupException{},
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
		return &careplan.ScheduleError{Op: opResolveCareDay, Err: err}
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
				plans.arrivalByStudentDate[studentID] = make(map[timezone.Date]*careplan.ArrivalSchedule)
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
	arrivals, err := s.deps.Records.ListArrivalSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: studentIDs})
	if err != nil {
		return &careplan.ScheduleError{Op: opResolveCareDay, Err: err}
	}
	for i := range arrivals {
		row := &arrivals[i]
		byWeekday, ok := plans.arrivalByStudentWeekday[row.StudentID]
		if !ok {
			byWeekday = map[int]*careplan.ArrivalSchedule{}
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
		return &careplan.ScheduleError{Op: opResolveCareDay, Err: err}
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
				plans.pickupByStudentDate[studentID] = make(map[timezone.Date]*careplan.PickupSchedule)
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
	arrivalExceptions, err := s.deps.Records.ListArrivalExceptions(ctx, careplan.StudentScheduleFilter{StudentIDs: studentIDs, From: careplan.Date(from), To: careplan.Date(to)})
	if err != nil {
		return &careplan.ScheduleError{Op: opResolveCareDay, Err: err}
	}
	for i := range arrivalExceptions {
		plans.addArrivalException(&arrivalExceptions[i])
	}

	pickupExceptions, err := s.deps.Records.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{StudentIDs: studentIDs, From: careplan.Date(from), To: careplan.Date(to)})
	if err != nil {
		return &careplan.ScheduleError{Op: opResolveCareDay, Err: err}
	}
	for i := range pickupExceptions {
		plans.addPickupException(&pickupExceptions[i])
	}
	return nil
}

func (p *carePlans) addArrivalException(row *careplan.ArrivalException) {
	if row == nil {
		return
	}
	if p.arrivalExceptions[row.StudentID] == nil {
		p.arrivalExceptions[row.StudentID] = make(map[timezone.Date]*careplan.ArrivalException)
	}
	p.arrivalExceptions[row.StudentID][timezone.Date(row.ExceptionDate)] = row
}

func (p *carePlans) addPickupException(row *careplan.PickupException) {
	if row == nil {
		return
	}
	if p.pickupExceptions[row.StudentID] == nil {
		p.pickupExceptions[row.StudentID] = make(map[timezone.Date]*careplan.PickupException)
	}
	p.pickupExceptions[row.StudentID][timezone.Date(row.ExceptionDate)] = row
}

func (p *carePlans) markHasPlan(studentID int64, date timezone.Date) {
	if p.hasPlan[studentID] == nil {
		p.hasPlan[studentID] = make(map[timezone.Date]bool)
	}
	p.hasPlan[studentID][date] = true
}

func (p *carePlans) hasArrivalSchedule(studentID int64, date timezone.Date) bool {
	if p.arrivalByStudentDate[studentID][date] != nil {
		return true
	}
	return p.arrivalByStudentWeekday[studentID][domain.ISOWeekday(date)] != nil
}

// effectiveArrival mirrors the exception-beats-schedule merge of
// GetBulkEffectiveArrivalTimesForDate for one day, minus the note loading the
// derivation has no use for.
func (p *carePlans) effectiveArrival(studentID int64, date timezone.Date) *careplan.EffectiveArrivalTime {
	weekday := domain.ISOWeekday(date)
	result := &careplan.EffectiveArrivalTime{Date: date, WeekdayName: domain.WeekdayName(weekday)}

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
	if weekday > 5 {
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
func (p *carePlans) effectivePickup(studentID int64, date timezone.Date) *careplan.EffectivePickupTime {
	weekday := domain.ISOWeekday(date)
	result := &careplan.EffectivePickupTime{Date: date, WeekdayName: domain.WeekdayName(weekday)}

	if exc, ok := p.pickupExceptions[studentID][date]; ok {
		result.IsException = true
		result.PickupTime = exc.PickupTime
		return result
	}
	if weekday > 5 {
		return result
	}
	if sched, ok := p.pickupByStudentDate[studentID][date]; ok && sched != nil {
		pickup := sched.PickupTime
		result.PickupTime = &pickup
	}
	return result
}

func (p *carePlans) factsFor(studentID int64, date timezone.Date) careplan.CareDayFacts {
	return careplan.CareDayFacts{
		BookingsAuthoritative: p.bookingsAuthoritative,
		HasBookedCareDay:      p.arrivalByStudentDate[studentID][date] != nil,
		HasPlan:               p.hasPlan[studentID][date],
		HasArrivalSchedule:    p.hasArrivalSchedule(studentID, date),
		Arrival:               p.effectiveArrival(studentID, date),
		Pickup:                p.effectivePickup(studentID, date),
	}
}
