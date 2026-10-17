package application

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// eligibleOn reports whether a student may be a candidate on the given date.
// Actual attendance wins before enrollment bounds are applied: a child who is
// still present must remain visible even after their planned care ended.
//
// Immediate activation (enrollment.default_activation_mode = "immediate") is the
// deliberate exception: the enrollment decision service creates an already
// 'active' student while keeping enrolled_from at the phase's future
// ServiceStartDate, so the child appears in lists/attendance from today. An
// active status therefore overrides the enrolled_from lower bound — but only
// from today onward: the override lets the child appear for the current and
// future dates before the phase officially starts, it must NOT make the child
// retroactively enrolled for every past date before enrolled_from. Otherwise a
// stale or manually created slot roster (or the /options counts) would show the
// child as planned/missing before their enrollment ever began (#1565 review).
// The enrollment part of that rule lives in userModel.EnrolledOn so the slot
// lists, the day-log rosters and the statistics report cannot drift apart
// (#1565, #2606); presence and alumnus status are decided here because they
// are candidate rules, not enrollment ones.
//
// today is the service clock's calendar day (s.todayDate()), threaded in so
// the immediate-activation cutoff uses the same clock as every other date
// guard in BuildList/ListOptions — deterministic simulations and
// time-controlled tests pin SlotListDependencies.Now, and reading the process clock
// here instead would decide eligibility against a different "today" (#1565
// review).
func eligibleOn(rules ports.Rules, student *peopledirectory.Student, date, today calendar.Date, actuallyPresent bool) (bool, error) {
	if student == nil {
		return false, nil
	}
	if actuallyPresent {
		return true, nil
	}
	if student.Status == peopledirectory.StudentStatusAlumnus {
		return false, nil
	}
	facts, err := studentFacts(student)
	if err != nil {
		return false, err
	}
	return rules.EnrolledOn(facts, date, today), nil
}

// listEligibleStudents returns the cohort candidates shared by the pickup
// builder and options aggregation. actual is supplied by the live pickup
// reader so an open or historical attendance row survives the upper boundary.
//
// The directory lists every non-alumni child of the tenant; the graduates the
// live reader still reports present are fetched by id so documented
// attendance keeps winning over the lifecycle status, exactly as before the
// projection read the owner seams.
func (s *service) listEligibleStudents(
	ctx context.Context, date calendar.Date, actual map[int64]struct{},
) ([]peopledirectory.Student, error) {
	all, err := s.students.ListEnrolledStudents(ctx)
	if err != nil {
		return nil, err
	}
	listed := make(map[int64]struct{}, len(all))
	for _, student := range all {
		listed[student.ID] = struct{}{}
	}
	missing := make([]int64, 0)
	for id := range actual {
		if _, ok := listed[id]; !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		sort.Slice(missing, func(i, j int) bool { return missing[i] < missing[j] })
		graduates, err := s.students.ListStudentsByID(ctx, missing)
		if err != nil {
			return nil, err
		}
		all = append(all, graduates...)
	}
	today := s.todayDate()
	eligible := make([]peopledirectory.Student, 0, len(all))
	for i := range all {
		student := &all[i]
		_, present := actual[student.ID]
		ok, err := eligibleOn(s.rules, student, date, today, present)
		if err != nil {
			return nil, err
		}
		if ok {
			eligible = append(eligible, *student)
		}
	}
	return eligible, nil
}

// cohortPickupTime returns the HH:MM used to place a student into a Ganztag
// cohort on a date, or "" when the child is not in care that day. A cleared
// same-day exception ("Kommt heute nicht") drops the effective pickup time; a
// cancelled care verdict then falls back to the regular weekly bucket so the
// signed-off child still appears in their cohort. Preview, export, and the
// options availability counts MUST all apply this identical fallback or they
// disagree on cancelled care days (#1565 review).
func (s *service) regularPickupBucket(
	ctx context.Context,
	studentIDs []int64,
	date calendar.Date,
) (map[int64]string, error) {
	out, err := s.pickupBaselines.RegularPickupTimes(ctx, studentIDs, date)
	if err != nil {
		return nil, fmt.Errorf("load regular pickup schedules: %w", err)
	}
	if out == nil {
		out = map[int64]string{}
	}
	return out, nil
}

func cohortPickupTime(cancelled bool, effective *time.Time, regular string) string {
	if effective != nil {
		return effective.Format(timeLayout)
	}
	if cancelled {
		return regular
	}
	return ""
}

func includePickupParticipant(careDay ports.CareDay, present bool) bool {
	return careDay != ports.CareDayNotScheduled || present
}

func (s *service) loadPickupCandidates(
	ctx context.Context, date calendar.Date,
) ([]peopledirectory.Student, []int64, map[int64]struct{}, error) {
	attendanceRows, err := s.presence.ListAttendance(ctx, studentpresence.AttendanceFilter{
		FromDate: date.String(), UntilDate: date.String(), StudentOrder: true,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	present := make(map[int64]struct{}, len(attendanceRows))
	for _, row := range attendanceRows {
		present[row.StudentID] = struct{}{}
	}
	students, err := s.listEligibleStudents(ctx, date, present)
	if err != nil {
		return nil, nil, nil, err
	}
	studentIDs := make([]int64, 0, len(students))
	for _, student := range students {
		studentIDs = append(studentIDs, student.ID)
	}
	return students, studentIDs, present, nil
}

type pickupEntryInputs struct {
	pickupTimes    map[int64]*time.Time
	arrivalTimes   map[int64]*time.Time
	statusByID     map[int64]string
	partialByID    map[int64]string
	careDays       map[int64]ports.CareDay
	regularByID    map[int64]string
	presentStudent map[int64]struct{}
}

// collectPickupEntries derives the cohort from effective times, dated care
// participation and attendance. Closed attendance rows count historically.
func (s *service) collectPickupEntries(
	ctx context.Context, params listRequest, buckets pickupBucketConfig, result *classday.Result,
) ([]mergedEntry, error) {
	students, studentIDs, present, err := s.loadPickupCandidates(ctx, params.date)
	if err != nil {
		return nil, err
	}
	inputs, err := s.loadPickupEntryInputs(ctx, params, studentIDs, present)
	if err != nil {
		return nil, err
	}
	entries, cohort := s.buildPickupCohortEntries(params, buckets, result.ListLabel, students, inputs)
	return appendUnplannedPickupEntries(entries, cohort, params, buckets, result.ListLabel, inputs), nil
}

func (s *service) loadPickupEntryInputs(
	ctx context.Context, params listRequest, studentIDs []int64, present map[int64]struct{},
) (pickupEntryInputs, error) {
	pickups, arrivals, err := s.loadPickupTiming(ctx, params, studentIDs)
	if err != nil {
		return pickupEntryInputs{}, err
	}
	statuses, partial, err := s.loadPickupAbsences(ctx, studentIDs, params.date)
	if err != nil {
		return pickupEntryInputs{}, err
	}
	careDays, regular, err := s.loadPickupCareEvidence(ctx, studentIDs, params.date)
	if err != nil {
		return pickupEntryInputs{}, err
	}
	return pickupEntryInputs{pickups, arrivals, statuses, partial, careDays, regular, present}, nil
}

func (s *service) loadPickupTiming(
	ctx context.Context, params listRequest, studentIDs []int64,
) (map[int64]*time.Time, map[int64]*time.Time, error) {
	pickups := map[int64]*time.Time{}
	arrivals := map[int64]*time.Time{}
	if len(studentIDs) == 0 {
		return pickups, arrivals, nil
	}
	var err error
	pickups, err = s.effectiveTimes.PickupTimes(ctx, studentIDs, params.date)
	if err != nil {
		return nil, nil, err
	}
	if pickups == nil {
		pickups = map[int64]*time.Time{}
	}
	if params.Source == classday.SourceReconciliation {
		arrivals, err = s.effectiveTimes.ArrivalTimes(ctx, studentIDs, params.date)
		if err != nil {
			return nil, nil, fmt.Errorf("load effective arrival times: %w", err)
		}
		if arrivals == nil {
			arrivals = map[int64]*time.Time{}
		}
	}
	return pickups, arrivals, nil
}

func (s *service) loadPickupAbsences(
	ctx context.Context, studentIDs []int64, date calendar.Date,
) (map[int64]string, map[int64]string, error) {
	statuses := map[int64]string{}
	partial := map[int64]string{}
	if len(studentIDs) == 0 {
		return statuses, partial, nil
	}
	// Active sign-offs plus the scheduler's end-of-day archived rows (source =
	// "end_of_day"), so a child signed off sick/excused all day still renders
	// "Abgemeldet" after the configured status-clear time rather than an
	// unexplained "Fehlt" (#1565 review pass 1).
	statusDays, err := s.carePlan.ListStudentStatusDays(ctx, careplan.StudentStatusDayFilter{
		StudentIDs: studentIDs, Date: careplan.Date(date.String()), IncludeEndOfDay: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("load student status days: %w", err)
	}
	for _, day := range statusDays {
		statuses[day.StudentID] = day.Status
	}
	exceptions, err := s.carePlan.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{
		StudentIDs: studentIDs, Date: careplan.Date(date.String()),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("load pickup exceptions for partial absences: %w", err)
	}
	for _, exc := range exceptions {
		if exc.ExcusedFrom != nil {
			partial[exc.StudentID] = calendar.NormalizeWallClock(*exc.ExcusedFrom).Format(timeLayout)
		}
	}
	return statuses, partial, nil
}

func (s *service) loadPickupCareEvidence(
	ctx context.Context, studentIDs []int64, date calendar.Date,
) (map[int64]ports.CareDay, map[int64]string, error) {
	careDays, err := s.careDays.ResolveForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve care days: %w", err)
	}
	regular, err := s.regularPickupBucket(ctx, studentIDs, date)
	if err != nil {
		return nil, nil, err
	}
	return careDays, regular, nil
}

func (s *service) buildPickupCohortEntries(
	params listRequest, buckets pickupBucketConfig, label string,
	students []peopledirectory.Student, inputs pickupEntryInputs,
) ([]mergedEntry, map[int64]struct{}) {
	entries := []mergedEntry{}
	cohort := map[int64]struct{}{}
	for _, student := range students {
		entry, belongs := s.pickupEntryForStudent(params, buckets, label, student.ID, inputs)
		if belongs {
			cohort[student.ID] = struct{}{}
		}
		if entry != nil {
			entries = append(entries, *entry)
		}
	}
	return entries, cohort
}

func (s *service) pickupEntryForStudent(
	params listRequest, buckets pickupBucketConfig, label string, studentID int64, inputs pickupEntryInputs,
) (*mergedEntry, bool) {
	careDay := inputs.careDays[studentID]
	cancelled := careDay == ports.CareDayCancelled
	hhmm := cohortPickupTime(cancelled, inputs.pickupTimes[studentID], inputs.regularByID[studentID])
	if hhmm == "" || !pickupMatchesCohort(params.PickupCohort, hhmm, buckets) {
		return nil, false
	}
	_, present := inputs.presentStudent[studentID]
	if !includePickupParticipant(careDay, present) {
		return nil, false
	}
	if careDay == ports.CareDayNotScheduled || cancelled && present {
		return &mergedEntry{StudentID: studentID, SlotLabel: label, PickupTime: hhmm, Present: true}, true
	}
	status, hasStatus := inputs.statusByID[studentID]
	partial := partialCoversPickup(hhmm, inputs.partialByID[studentID])
	if params.Source == classday.SourceReconciliation && !present && !cancelled && !hasStatus && !partial &&
		s.beforeEffectiveArrival(params.date, inputs.arrivalTimes[studentID]) {
		return nil, true
	}
	return plannedPickupEntry(studentID, label, hhmm, present, cancelled, partial, status, hasStatus), true
}

func plannedPickupEntry(
	studentID int64, label, hhmm string, present, cancelled, partial bool, status string, hasStatus bool,
) *mergedEntry {
	entry := &mergedEntry{StudentID: studentID, SlotLabel: label, PickupTime: hhmm, Planned: true, Present: present}
	if present {
		return entry
	}
	entry.PlannedStatus = ports.AttendanceAbsent
	switch {
	case hasStatus:
		entry.PlannedSubstatus = &status
	case cancelled:
		substatus := string(ports.CareDayCancelled)
		entry.PlannedSubstatus = &substatus
	case partial:
		substatus := substatusExcused
		entry.PlannedSubstatus = &substatus
	default:
		entry.PlannedStatus = ""
	}
	return entry
}

func appendUnplannedPickupEntries(
	entries []mergedEntry, cohort map[int64]struct{}, params listRequest,
	buckets pickupBucketConfig, label string, inputs pickupEntryInputs,
) []mergedEntry {
	if params.Source != classday.SourceReconciliation {
		return entries
	}
	for id := range inputs.presentStudent {
		if _, belongs := cohort[id]; belongs {
			continue
		}
		cancelled := inputs.careDays[id] == ports.CareDayCancelled
		resolved := cohortPickupTime(cancelled, inputs.pickupTimes[id], inputs.regularByID[id])
		if pickupInAnyCohort(resolved, buckets) {
			continue
		}
		pickupLabel := ""
		if pickup := inputs.pickupTimes[id]; pickup != nil {
			pickupLabel = pickup.Format(timeLayout)
		}
		entries = append(entries, mergedEntry{
			StudentID: id, SlotLabel: label, PickupTime: pickupLabel, Present: true,
		})
	}
	return entries
}

func pickupMatchesCohort(cohort classday.PickupCohort, hhmm string, buckets pickupBucketConfig) bool {
	switch cohort {
	case classday.PickupCohortShortDay:
		return hhmm <= buckets.ShortCutoff
	case classday.PickupCohortLongDay:
		return hhmm > buckets.ShortCutoff && hhmm <= buckets.LongCutoff
	default:
		return false
	}
}

// pickupInAnyCohort reports whether hhmm places a child into any valid pickup
// cohort (short or long day). A child with such a time is planned in exactly one
// cohort, so a reconciliation for a DIFFERENT cohort must not treat their
// presence as unplanned. Returns false for an empty time (not in care) or a
// pickup past the long-day cutoff (no cohort claims them).
func pickupInAnyCohort(hhmm string, buckets pickupBucketConfig) bool {
	if hhmm == "" {
		return false
	}
	return pickupMatchesCohort(classday.PickupCohortShortDay, hhmm, buckets) ||
		pickupMatchesCohort(classday.PickupCohortLongDay, hhmm, buckets)
}
