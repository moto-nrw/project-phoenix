package compose

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// Lost-edit detection (#1875): the planned, template-backed occurrences whose
// content diverges from what the current template would materialize. The
// deviation snapshot of a series re-plan only carries Vertretungsplan
// overrides and the required_staff pin (#1840/#1839), so every divergence
// reported here (timetable.EditedChange*) is silently discarded when the
// series is edited.

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
) ([]timetable.EditedOccurrence, error) {
	if to.Before(from) {
		return nil, &ScheduleError{Op: "detect edited: validate window", Err: errors.New("to must not be before from")}
	}
	tmpl, err := s.loadDetectionTemplate(ctx, activityGroupID)
	if err != nil || tmpl == nil {
		return nil, err
	}
	planned, ids, err := s.loadPlannedOccurrences(ctx, activityGroupID, from, to)
	if err != nil {
		return nil, err
	}
	// No surviving instances to diff and (for a same-template re-plan) no
	// deletions to report either — short-circuit before the heavier loads.
	if len(planned) == 0 && !includeDeletions {
		return nil, nil
	}

	// Exceptions in the window: modified ones shape the expected slots;
	// cancelled ones are individually-deleted occurrences that a
	// following-series split would rematerialize under the successor template
	// — reported only when the caller asks for deletions (#1875 review).
	exceptions, err := s.exceptionRepo.FindByActivityGroupAndDateRange(ctx, activityGroupID, schedule.Date(from), schedule.Date(to))
	if err != nil {
		return nil, &ScheduleError{Op: "detect edited: load exceptions", Err: err}
	}

	edited := make([]timetable.EditedOccurrence, 0)
	if len(planned) > 0 {
		projection, err := s.loadTemplateProjection(ctx, tmpl, ids)
		if err != nil {
			return nil, err
		}
		edited = append(edited, projection.editedOccurrences(tmpl, planned, buildExceptionIndex(exceptions))...)
	}
	if includeDeletions {
		edited = append(edited, deletedOccurrences(tmpl, exceptions)...)
	}

	sort.Slice(edited, func(i, j int) bool {
		if edited[i].Date != edited[j].Date {
			return edited[i].Date.Before(edited[j].Date)
		}
		return edited[i].StartTime < edited[j].StartTime
	})
	return edited, nil
}

// loadDetectionTemplate loads the template to compare against. A template
// not visible to this tenant (or deleted) yields nil: there is nothing to
// compare against, so no edits are reported rather than blocking the caller.
func (s *materializationService) loadDetectionTemplate(ctx context.Context, templateID int64) (*activities.Group, error) {
	tmpl, err := s.groupRepo.FindByID(ctx, templateID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, nil
		}
		return nil, &ScheduleError{Op: "detect edited: load template", Err: err}
	}
	return tmpl, nil
}

// loadPlannedOccurrences keeps only the rows a re-plan would delete and
// regenerate: planned, template-backed, non-spontaneous.
func (s *materializationService) loadPlannedOccurrences(
	ctx context.Context,
	templateID int64,
	from, to timezone.Date,
) ([]*schedule.ActivityInstance, []int64, error) {
	instances, err := s.instanceRepo.FindByActivityGroupAndDateRange(ctx, templateID, schedule.Date(from), schedule.Date(to))
	if err != nil {
		return nil, nil, &ScheduleError{Op: "detect edited: load instances", Err: err}
	}
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
	return planned, ids, nil
}

// templateProjection is everything the materializer's per-date rules need
// plus the occurrences' actual rosters. Expected start/end/room depend on the
// date (schedule validity, A/B week pattern, active period, modified or
// cancelled exception) — see expectedSlotsOn.
type templateProjection struct {
	roster             templateRoster
	timeframeByID      map[int64]*schedule.Timeframe
	periods            []*schedule.CalendarPeriod
	staffByInstance    map[int64][]*schedule.InstanceStaff
	studentsByInstance map[int64][]*schedule.InstanceStudent
	logger             *slog.Logger
}

func (s *materializationService) loadTemplateProjection(
	ctx context.Context,
	tmpl *activities.Group,
	ids []int64,
) (*templateProjection, error) {
	schedules, err := s.scheduleRepo.FindByGroupID(ctx, tmpl.ID)
	if err != nil {
		return nil, &ScheduleError{Op: "detect edited: load schedules", Err: err}
	}
	timeframeByID, err := s.loadTimeframes(ctx, "detect edited: load timeframes")
	if err != nil {
		return nil, err
	}
	periods, err := s.periodRepo.FindActiveByTenantID(ctx)
	if err != nil {
		return nil, &ScheduleError{Op: "detect edited: load periods", Err: err}
	}
	roster, err := s.loadTemplatePeople(ctx, tmpl.ID, "detect edited")
	if err != nil {
		return nil, err
	}
	roster.schedules = schedules
	staffByInstance, err := s.staffRosterByInstance(ctx, ids)
	if err != nil {
		return nil, err
	}
	studentsByInstance, err := s.studentRosterByInstance(ctx, ids)
	if err != nil {
		return nil, err
	}
	return &templateProjection{
		roster:             roster,
		timeframeByID:      timeframeByID,
		periods:            periods,
		staffByInstance:    staffByInstance,
		studentsByInstance: studentsByInstance,
		logger:             s.getLogger(),
	}, nil
}

// editedOccurrences diffs every planned occurrence against its template
// projection.
func (p *templateProjection) editedOccurrences(
	tmpl *activities.Group,
	planned []*schedule.ActivityInstance,
	exceptionIdx map[exceptionKey]*schedule.ActivityException,
) []timetable.EditedOccurrence {
	edited := make([]timetable.EditedOccurrence, 0)
	for _, inst := range planned {
		instanceDate := timezone.Date(inst.Date)
		expected := p.expectedSlotsOn(tmpl, exceptionIdx[exceptionKey{tmpl.ID, instanceDate}], instanceDate)
		expectedStudentIDs := expectedStudentIDsOn(
			p.roster.enrollments,
			p.roster.targetStudentIDs,
			p.roster.careBounds,
			instanceDate,
			calendarPeriodID(inst),
		)
		changes := diffOccurrenceWithExpectedStudents(
			inst,
			tmpl.Name,
			expectedStudentIDs,
			p.roster.supervisors,
			p.staffByInstance[inst.ID],
			p.studentsByInstance[inst.ID],
			expected,
		)
		// Listenart is a template-level field materialization copies verbatim
		// onto every occurrence, so it is compared here (template vs occurrence)
		// rather than per-slot inside diffOccurrence (#1565 review).
		if !sameListKind(inst.ListKind, tmpl.ListKind) {
			changes = append(changes, timetable.EditedChangeListKind)
			sort.Strings(changes)
		}
		if len(changes) == 0 {
			continue
		}
		edited = append(edited, timetable.EditedOccurrence{
			InstanceID: inst.ID,
			Date:       instanceDate,
			StartTime:  formatTimeOfDay(inst.StartTime),
			Title:      inst.Title,
			Changes:    changes,
		})
	}
	return edited
}

// deletedOccurrences reports the individually deleted occurrences (cancelled
// exceptions). There is no instance row, so InstanceID 0 signals a deletion.
func deletedOccurrences(tmpl *activities.Group, exceptions []*schedule.ActivityException) []timetable.EditedOccurrence {
	deleted := make([]timetable.EditedOccurrence, 0)
	for _, exc := range exceptions {
		if exc == nil || exc.ExceptionType != schedule.ActivityExceptionCancelled {
			continue
		}
		deleted = append(deleted, timetable.EditedOccurrence{
			InstanceID: 0,
			Date:       timezone.Date(exc.ExceptionDate),
			StartTime:  "",
			Title:      tmpl.Name,
			Changes:    []string{timetable.EditedChangeDeleted},
		})
	}
	return deleted
}

// expectedSlotsOn returns the start/end/room parameters the template would
// materialize on `date`, replaying materializeTemplate's exact per-date rules
// (candidateSlot). A cancelled date (or an off-cycle / out-of-range date)
// yields no slots — an occurrence there has no template match and so is
// reported as a lost `time` edit, exactly as ReplanWeek would drop it. Legacy
// weekend schedules remain administrable but are never materialized, so they
// are not expected either: treating them as expected would hide the warning
// that a re-plan removes a retained weekend instance without recreating it.
func (p *templateProjection) expectedSlotsOn(
	tmpl *activities.Group,
	exc *schedule.ActivityException,
	date timezone.Date,
) []materialParams {
	if isWeekend(date) {
		return nil
	}
	isoWd := isoWeekday(date)
	out := make([]materialParams, 0, 1)
	for _, sch := range p.roster.schedules {
		if sch.Weekday != isoWd {
			continue
		}
		// Occurrences on closing days and holidays are compared with the series
		// pattern as if the day were open: an untouched occurrence planned before
		// the closure is not an edit (#3594), and a re-plan drops it anyway.
		effective, _, skip := candidateSlot(tmpl, sch, date, p.periods, timetable.NonWorkingDays{}, exc, p.timeframeByID, p.logger)
		if skip == candidateKept {
			out = append(out, effective)
		}
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
		changes = append(changes, timetable.EditedChangeStaff)
	}
	changes = append(changes, diffOccurrenceStudents(expectedStudentIDs, studentRows)...)
	sort.Strings(changes)
	return changes
}

func diffOccurrenceText(inst *schedule.ActivityInstance, templateTitle string) []string {
	var changes []string
	if inst.Title != templateTitle {
		changes = append(changes, timetable.EditedChangeTitle)
	}
	if inst.Description != nil && strings.TrimSpace(*inst.Description) != "" {
		changes = append(changes, timetable.EditedChangeDescription)
	}
	if inst.Notes != nil && strings.TrimSpace(*inst.Notes) != "" {
		changes = append(changes, timetable.EditedChangeNotes)
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
		return []string{timetable.EditedChangeTime}
	}
	var changes []string
	if match.RoomID > 0 && inst.RoomID != match.RoomID {
		changes = append(changes, timetable.EditedChangeRoom)
	}
	if !sameClock(inst.EndTime, match.EndTime) {
		changes = append(changes, timetable.EditedChangeTime)
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
		changes = append(changes, timetable.EditedChangeStudents)
	}
	if attendanceLost {
		changes = append(changes, timetable.EditedChangeAttendance)
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
