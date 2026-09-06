package schedule

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
)

// Edited-field categories. These are the fields a "Nur diesen Termin" edit can
// change that a series re-plan (ReplanWeek / template split) does NOT preserve:
// the deviation snapshot only carries Vertretungsplan overrides + the
// required_staff pin (#1840/#1839), so anything below is silently discarded
// when the series is edited (#1875). Stable machine-readable strings — the
// frontend maps them to German labels.
const (
	EditedChangeTitle       = "title"
	EditedChangeDescription = "description"
	EditedChangeNotes       = "notes"
	EditedChangeRoom        = "room"
	EditedChangeTime        = "time"
	EditedChangeStaff       = "staff"
	EditedChangeStudents    = "students"
	// EditedChangeAttendance marks a manual or observed attendance state on a
	// still-planned occurrence (present, hand-set absent, not_scheduled, a
	// manual_status_at stamp). ReplanWeek deletes the instance and rematerializes
	// expected rows; it reapplies status-day and pickup-exception absences but
	// does not snapshot ordinary attendance, so this must be reported as a lost
	// edit. Distinct from EditedChangeStudents: the child can stay on the roster
	// while the attendance state itself would be discarded.
	EditedChangeAttendance = "attendance"
	// EditedChangeListKind marks a per-occurrence Listenart override (#1565): the
	// occurrence's list_kind diverges from what the template would materialize. A
	// series re-plan copies list_kind from the template, so this single-occurrence
	// classification is discarded exactly like a title or room edit — it must be
	// reported so the lost-edits warning covers it.
	EditedChangeListKind = "list_kind"
	// EditedChangeDeleted marks a date whose occurrence was individually deleted
	// (a cancelled exception). A same-template re-plan preserves it, but a
	// following-series split rematerializes it under the successor template, so
	// it is only reported when the caller asks for deletions (#1875 review).
	EditedChangeDeleted = "deleted"
)

// EditedOccurrence describes one planned, template-backed occurrence that was
// individually adjusted relative to its template — i.e. a single-occurrence
// edit a series re-plan would discard (#1875). Changes is the non-empty, sorted
// set of diverging field categories (see the EditedChange* constants).
type EditedOccurrence struct {
	InstanceID int64
	Date       timezone.Date
	StartTime  string // "15:04:05" wall clock, for display
	Title      string // the occurrence's current (possibly edited) title
	Changes    []string
}

// DetectEditedInWindow returns the planned, template-backed occurrences of one
// template in [from, to] whose content diverges from what the current template
// would materialize — the single-occurrence edits a series re-plan (#1875)
// would silently discard. Read-only; runs under the caller's ambient tenant
// (RLS) transaction. Deviation-only rows (absences, substitutes, understaffed
// ack, required_staff pin) are intentionally NOT reported: ReplanWeek preserves
// those, so they are not "lost".
func (s *materializationService) DetectEditedInWindow(
	ctx context.Context,
	activityGroupID int64,
	from, to timezone.Date,
	includeDeletions bool,
) ([]EditedOccurrence, error) {
	if to.Before(from) {
		return nil, &ScheduleError{Op: "detect edited: validate window", Err: errors.New("to must not be before from")}
	}

	tmpl, err := s.groupRepo.FindByID(ctx, activityGroupID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			// Template not visible to this tenant (or deleted) — nothing to
			// compare against; report no edits rather than blocking the caller.
			return nil, nil
		}
		return nil, &ScheduleError{Op: "detect edited: load template", Err: err}
	}
	if tmpl == nil {
		return nil, nil
	}

	instances, err := s.instanceRepo.FindByActivityGroupAndDateRange(ctx, activityGroupID, schedule.Date(from), schedule.Date(to))
	if err != nil {
		return nil, &ScheduleError{Op: "detect edited: load instances", Err: err}
	}
	// Keep only the rows a re-plan would delete-and-regenerate: planned,
	// template-backed, non-spontaneous.
	planned := make([]*schedule.ActivityInstance, 0, len(instances))
	ids := make([]int64, 0, len(instances))
	for _, inst := range instances {
		if inst == nil || inst.IsSpontaneous || inst.ActivityGroupID == nil {
			continue
		}
		if inst.Status != schedule.InstanceStatusPlanned {
			continue
		}
		planned = append(planned, inst)
		ids = append(ids, inst.ID)
	}
	// No surviving instances to diff and (for a same-template re-plan) no
	// deletions to report either — short-circuit before the heavier loads.
	if len(planned) == 0 && !includeDeletions {
		return nil, nil
	}

	// Exceptions in the window: modified ones shape expectedSlotsOn; cancelled
	// ones are individually-deleted occurrences that a following-series split
	// would rematerialize under the successor template — reported only when the
	// caller asks for deletions (#1875 review).
	exceptions, err := s.exceptionRepo.FindByActivityGroupAndDateRange(ctx, activityGroupID, schedule.Date(from), schedule.Date(to))
	if err != nil {
		return nil, &ScheduleError{Op: "detect edited: load exceptions", Err: err}
	}
	exceptionIdx := buildExceptionIndex(exceptions)

	edited := make([]EditedOccurrence, 0)

	if len(planned) > 0 {
		// Load everything the materializer's per-date rules need. Expected
		// start/end/room depend on the date (schedule validity, A/B week pattern,
		// active period, modified/cancelled exception) — see expectedSlotsOn.
		schedules, err := s.scheduleRepo.FindByGroupID(ctx, activityGroupID)
		if err != nil {
			return nil, &ScheduleError{Op: "detect edited: load schedules", Err: err}
		}
		timeframes, err := s.timeframeRepo.ListAll(ctx)
		if err != nil {
			return nil, &ScheduleError{Op: "detect edited: load timeframes", Err: err}
		}
		timeframeByID := make(map[int64]*schedule.Timeframe, len(timeframes))
		for _, tf := range timeframes {
			timeframeByID[tf.ID] = tf
		}
		periods, err := s.periodRepo.FindActiveByTenantID(ctx)
		if err != nil {
			return nil, &ScheduleError{Op: "detect edited: load periods", Err: err}
		}
		enrollments, err := s.enrollmentRepo.FindByGroupID(ctx, activityGroupID)
		if err != nil {
			return nil, &ScheduleError{Op: "detect edited: load enrollments", Err: err}
		}
		targetStudentIDs := make([]int64, 0)
		if targetRepo, ok := s.groupRepo.(activities.GroupTargetRepository); ok {
			targetStudentIDs, err = targetRepo.FindTargetStudentIDs(ctx, activityGroupID)
			if err != nil {
				return nil, &ScheduleError{Op: "detect edited: load target students", Err: err}
			}
		}
		careBounds, err := s.loadCareBounds(ctx, targetStudentIDs, enrollments)
		if err != nil {
			return nil, &ScheduleError{Op: "detect edited: load care bounds", Err: err}
		}
		supervisors, err := s.supervisorRepo.FindByGroupID(ctx, activityGroupID)
		if err != nil {
			return nil, &ScheduleError{Op: "detect edited: load supervisors", Err: err}
		}
		staffByInstance, err := s.staffRosterByInstance(ctx, ids)
		if err != nil {
			return nil, err
		}
		studentsByInstance, err := s.studentRosterByInstance(ctx, ids)
		if err != nil {
			return nil, err
		}

		for _, inst := range planned {
			instanceDate := timezone.Date(inst.Date)
			expected := s.expectedSlotsOn(
				tmpl,
				schedules,
				timeframeByID,
				periods,
				exceptionIdx[exceptionKey{tmpl.ID, instanceDate}],
				instanceDate,
			)
			expectedStudentIDs := expectedStudentIDsOn(
				enrollments,
				targetStudentIDs,
				careBounds,
				instanceDate,
				calendarPeriodID(inst),
			)
			changes := diffOccurrenceWithExpectedStudents(
				inst,
				tmpl.Name,
				expectedStudentIDs,
				supervisors,
				staffByInstance[inst.ID],
				studentsByInstance[inst.ID],
				expected,
			)
			// Listenart is a template-level field materialization copies verbatim
			// onto every occurrence, so it is compared here (template vs occurrence)
			// rather than per-slot inside diffOccurrence (#1565 review).
			if !sameListKind(inst.ListKind, tmpl.ListKind) {
				changes = append(changes, EditedChangeListKind)
				sort.Strings(changes)
			}
			if len(changes) == 0 {
				continue
			}
			edited = append(edited, EditedOccurrence{
				InstanceID: inst.ID,
				Date:       instanceDate,
				StartTime:  formatTimeOfDay(inst.StartTime),
				Title:      inst.Title,
				Changes:    changes,
			})
		}
	}

	if includeDeletions {
		for _, exc := range exceptions {
			if exc == nil || exc.ExceptionType != schedule.ActivityExceptionCancelled {
				continue
			}
			// No instance row (it was deleted); InstanceID 0 signals a deletion.
			edited = append(edited, EditedOccurrence{
				InstanceID: 0,
				Date:       timezone.Date(exc.ExceptionDate),
				StartTime:  "",
				Title:      tmpl.Name,
				Changes:    []string{EditedChangeDeleted},
			})
		}
	}

	sort.Slice(edited, func(i, j int) bool {
		if edited[i].Date != edited[j].Date {
			return edited[i].Date.Before(edited[j].Date)
		}
		return edited[i].StartTime < edited[j].StartTime
	})
	return edited, nil
}

// expectedSlotsOn returns the start/end/room parameters the template would
// materialize on `date`, replaying materializeTemplate's exact per-date rules:
// weekday match, schedule valid_from/valid_until, active-period selection, the
// A/B week_pattern predicate, timeframe resolution, and finally the per-date
// modified/cancelled exception. A cancelled date (or an off-cycle / out-of-range
// date) yields no slots — an occurrence there has no template match and so is
// reported as a lost `time` edit, exactly as ReplanWeek would drop it. This is
// the date-aware, exception-aware replacement for the old (weekday, start) map:
// it neither misses off-cycle moves nor falsely flags exception-shifted starts.
func (s *materializationService) expectedSlotsOn(
	tmpl *activities.Group,
	schedules []*activities.Schedule,
	timeframeByID map[int64]*schedule.Timeframe,
	periods []*schedule.CalendarPeriod,
	exc *schedule.ActivityException,
	date timezone.Date,
) []materialParams {
	// Keep this projection aligned with materializeTemplate: legacy weekend
	// schedules remain administrable, but are never materialized into new
	// instances. Treating them as expected here would hide the warning that a
	// re-plan will remove a retained weekend instance without recreating it.
	if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
		return nil
	}

	isoWd := isoWeekday(date)
	out := make([]materialParams, 0, 1)
	for _, sch := range schedules {
		if sch.Weekday != isoWd {
			continue
		}
		if scheduleEndedOn(sch, date) || scheduleNotStartedOn(sch, date) {
			continue
		}
		period := selectPeriod(tmpl, sch, date, periods, s.getLogger())
		if period == nil {
			continue
		}
		if !s.calendarService.ShouldMaterialize(sch.WeekPattern, date, period) {
			continue
		}
		tfID := int64(0)
		if sch.TimeframeID != nil {
			tfID = *sch.TimeframeID
		}
		tf, ok := timeframeByID[tfID]
		if !ok || tf.EndTime == nil {
			continue
		}
		base := materialParams{
			StartTime: extractTimeOfDay(tf.StartTime),
			EndTime:   extractTimeOfDay(*tf.EndTime),
			RoomID:    0,
		}
		if tmpl.PlannedRoomID != nil {
			base.RoomID = *tmpl.PlannedRoomID
		}
		effective, skip := applyException(base, exc)
		if skip {
			continue // cancelled exception → nothing materializes on this date
		}
		if effective.RoomID <= 0 {
			continue // no room the materializer could satisfy the NOT NULL with
		}
		out = append(out, effective)
	}
	return out
}

// staffRosterByInstance batch-loads instance_staff rows grouped by instance.
func (s *materializationService) staffRosterByInstance(ctx context.Context, ids []int64) (map[int64][]*schedule.InstanceStaff, error) {
	rows, err := s.staffRepo.FindByInstanceIDs(ctx, ids)
	if err != nil {
		return nil, &ScheduleError{Op: "detect edited: load staff rosters", Err: err}
	}
	byInstance := make(map[int64][]*schedule.InstanceStaff, len(ids))
	for _, row := range rows {
		byInstance[row.InstanceID] = append(byInstance[row.InstanceID], row)
	}
	return byInstance, nil
}

// studentRosterByInstance batch-loads instance_students rows grouped by
// instance. Membership is derived from the row's existence; attendance state
// is kept so unrestored present/absent can be reported separately (#2225).
func (s *materializationService) studentRosterByInstance(ctx context.Context, ids []int64) (map[int64][]*schedule.InstanceStudent, error) {
	rows, err := s.studentRepo.FindByInstanceIDs(ctx, ids)
	if err != nil {
		return nil, &ScheduleError{Op: "detect edited: load student rosters", Err: err}
	}
	byInstance := make(map[int64][]*schedule.InstanceStudent, len(ids))
	for _, row := range rows {
		byInstance[row.InstanceID] = append(byInstance[row.InstanceID], row)
	}
	return byInstance, nil
}

// diffOccurrenceWithExpectedStudents returns the field categories on which
// one planned occurrence diverges from its template projection. Both slot and
// student inputs already contain the exact values materialization would write.
func diffOccurrenceWithExpectedStudents(
	inst *schedule.ActivityInstance,
	templateTitle string,
	expectedStudentIDs []int64,
	supervisors []*activities.SupervisorPlanned,
	staffRows []*schedule.InstanceStaff,
	studentRows []*schedule.InstanceStudent,
	expected []materialParams,
) []string {
	changes := diffOccurrenceText(inst, templateTitle)
	changes = append(changes, diffOccurrenceSlot(inst, expected)...)
	if staffRosterChanged(inst, supervisors, staffRows) {
		changes = append(changes, EditedChangeStaff)
	}
	changes = append(changes, diffOccurrenceStudents(expectedStudentIDs, studentRows)...)
	sort.Strings(changes)
	return changes
}

func diffOccurrenceText(inst *schedule.ActivityInstance, templateTitle string) []string {
	var changes []string
	if inst.Title != templateTitle {
		changes = append(changes, EditedChangeTitle)
	}
	if inst.Description != nil && strings.TrimSpace(*inst.Description) != "" {
		changes = append(changes, EditedChangeDescription)
	}
	if inst.Notes != nil && strings.TrimSpace(*inst.Notes) != "" {
		changes = append(changes, EditedChangeNotes)
	}
	return changes
}

func diffOccurrenceSlot(inst *schedule.ActivityInstance, expected []materialParams) []string {
	var match *materialParams
	instStart := formatTimeOfDay(inst.StartTime)
	for i := range expected {
		if formatTimeOfDay(expected[i].StartTime) == instStart {
			match = &expected[i]
			break
		}
	}
	if match == nil {
		return []string{EditedChangeTime}
	}
	var changes []string
	if match.RoomID > 0 && inst.RoomID != match.RoomID {
		changes = append(changes, EditedChangeRoom)
	}
	if !sameClock(inst.EndTime, match.EndTime) {
		changes = append(changes, EditedChangeTime)
	}
	return changes
}

func staffRosterChanged(
	inst *schedule.ActivityInstance,
	supervisors []*activities.SupervisorPlanned,
	staffRows []*schedule.InstanceStaff,
) bool {
	periodID := calendarPeriodID(inst)
	instanceDate := timezone.Date(inst.Date)
	primaryStaffID, hasPrimary := effectivePrimarySupervisor(supervisors, instanceDate, periodID)
	expectedStaff := make(map[int64]bool)
	for _, sup := range supervisors {
		if isSupervisorValidOn(sup, instanceDate, periodID) {
			expectedStaff[sup.StaffID] = hasPrimary && sup.StaffID == primaryStaffID
		}
	}
	actualStaff := make(map[int64]bool)
	for _, row := range staffRows {
		if !row.IsSubstitute {
			actualStaff[row.StaffID] = row.IsPrimary
		}
	}
	return !sameStaffSet(expectedStaff, actualStaff)
}

func diffOccurrenceStudents(
	expectedStudentIDs []int64,
	studentRows []*schedule.InstanceStudent,
) []string {
	expectedStudents := make(map[int64]struct{}, len(expectedStudentIDs))
	for _, studentID := range expectedStudentIDs {
		expectedStudents[studentID] = struct{}{}
	}
	studentSet := make(map[int64]struct{}, len(studentRows))
	attendanceLost := false
	for _, row := range studentRows {
		if row == nil {
			continue
		}
		studentSet[row.StudentID] = struct{}{}
		if attendanceWouldBeLost(row) {
			attendanceLost = true
		}
	}
	var changes []string
	if !sameIDSet(expectedStudents, studentSet) {
		changes = append(changes, EditedChangeStudents)
	}
	if attendanceLost {
		changes = append(changes, EditedChangeAttendance)
	}

	return changes
}

func calendarPeriodID(inst *schedule.ActivityInstance) int64 {
	if inst == nil || inst.CalendarPeriodID == nil {
		return 0
	}
	return *inst.CalendarPeriodID
}

// sameListKind reports whether two optional list_kind values are equivalent,
// treating nil and the empty string as the same "no classification" so an
// occurrence that merely omits the field is not flagged against a template that
// stores "".
func sameListKind(a, b *string) bool {
	av, bv := "", ""
	if a != nil {
		av = *a
	}
	if b != nil {
		bv = *b
	}
	return av == bv
}

// sameClock compares two times on their wall-clock components only.
func sameClock(a, b time.Time) bool {
	return a.Hour() == b.Hour() && a.Minute() == b.Minute() && a.Second() == b.Second()
}

// sameStaffSet reports whether two staffID→isPrimary maps are identical in both
// membership and primary flag.
func sameStaffSet(a, b map[int64]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for id, primary := range a {
		if bp, ok := b[id]; !ok || bp != primary {
			return false
		}
	}
	return true
}

// sameIDSet reports whether two ID sets have identical membership.
func sameIDSet(a, b map[int64]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for id := range a {
		if _, ok := b[id]; !ok {
			return false
		}
	}
	return true
}

// attendanceWouldBeLost reports whether a series re-plan would discard this
// row's attendance state. ReplanWeek deletes the planned instance (cascading
// the row) and rematerializes it as expected, then reapplies active status
// days and pickup-exception absences. Ordinary present/absent, not_scheduled,
// and hand-set stamps have no restore path.
func attendanceWouldBeLost(row *schedule.InstanceStudent) bool {
	if row == nil {
		return false
	}
	// Status-day and pickup-exception absences are reapplied after
	// rematerialization. They are not lost unless a later manual or observed
	// overlay sits on top of them.
	statusOwned := row.StudentStatusDayID != nil || row.PickupExceptionID != nil
	if statusOwned &&
		row.Status == schedule.AttendanceStatusAbsent &&
		row.ManualStatusAt == nil &&
		!row.NotScheduled &&
		row.CheckedInAt == nil &&
		row.CheckedOutAt == nil {
		return false
	}
	if row.Status != schedule.AttendanceStatusExpected {
		return true
	}
	return row.NotScheduled || row.ManualStatusAt != nil ||
		row.CheckedInAt != nil || row.CheckedOutAt != nil
}
