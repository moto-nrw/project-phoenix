package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

func (s *operations) Roster(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (*timetable.OperationRoster, error) {
	if _, err := s.requireCanView(ctx, accountID, isAdmin, instanceID); err != nil {
		return nil, err
	}
	roster, err := s.buildRoster(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	return s.rosterWithActionAccess(ctx, accountID, isAdmin, instanceID, roster, nil)
}

func (s *operations) RosterByActiveGroup(ctx context.Context, accountID int64, isAdmin bool, activeGroupID int64) (*timetable.OperationRoster, error) {
	inst, err := s.deps.Instances.FindByActiveGroupID(ctx, activeGroupID)
	if err != nil {
		return nil, err
	}
	if inst == nil {
		return nil, timetable.ErrTimetableOperationNotFound
	}
	return s.Roster(ctx, accountID, isAdmin, inst.ID)
}

// rosterWithActionAccess adds the caller's action rights; reads and write
// responses carry the same flags. A failed absence-policy lookup hides that
// action without preventing an independently authorized check-in.
func (s *operations) rosterWithActionAccess(ctx context.Context, accountID int64, isAdmin bool, instanceID int64, roster *timetable.OperationRoster, err error) (*timetable.OperationRoster, error) {
	if err != nil || roster == nil {
		return roster, err
	}
	roster.CanOperate, err = s.canOperate(ctx, accountID, isAdmin, instanceID)
	if err != nil {
		return nil, err
	}
	staffID, err := s.requireCanEditAttendance(ctx, accountID, isAdmin, instanceID)
	if err != nil && !errors.Is(err, timetable.ErrTimetableOperationForbidden) {
		return nil, err
	}
	roster.CanEditAttendance = err == nil && staffID > 0
	roster.CanReportAbsence = s.requireCanReportAbsence(ctx, accountID, isAdmin, instanceID) == nil
	return roster, nil
}

func (s *operations) buildRoster(ctx context.Context, instanceID int64) (*timetable.OperationRoster, error) {
	return s.buildRosterWithCareDay(ctx, instanceID, nil)
}

// rosterSources is everything a roster is built from: the block, its
// planned rows and session visits, and the children with their persons and
// groups.
type rosterSources struct {
	inst       *scheduleModels.ActivityInstance
	planned    []*scheduleModels.InstanceStudent
	visits     []studentpresence.Visit
	studentIDs []int64
	students   map[int64]*usersModels.Student
	persons    map[int64]*usersModels.Person
	groupNames map[int64]string
	template   *rosterTemplateGroup
}

// buildRosterWithCareDay builds the roster, optionally reusing a care-day
// map the caller already resolved: PlannedNow resolves once for every block
// of the day, nil makes this method resolve for itself.
//
// Graduated children and children whose care ended drop off any roster a
// supervisor can still act on; frozen history (past-dated, completed or
// cancelled blocks) keeps every row (#405, #2487).
func (s *operations) buildRosterWithCareDay(ctx context.Context, instanceID int64, careDay map[int64]timetable.CareDayStatus) (*timetable.OperationRoster, error) {
	src, err := s.loadRosterSources(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	warnings := s.rosterWarnings(ctx, src)
	if careDay == nil {
		careDay, err = s.deps.CareDays.ResolveForDate(ctx, src.studentIDs, timezone.Date(src.inst.Date))
		if err != nil {
			return nil, err
		}
	}
	pickupTimes, pickupTimesLoaded := s.rosterPickupTimes(ctx, src.inst, src.studentIDs)
	parallelPresence, err := s.parallelPresenceByStudent(ctx, src.inst, src.studentIDs)
	if err != nil {
		return nil, err
	}
	rows := s.rosterRows(src, rosterRowContext{
		excluded: rosterExcludedAlumni(src.inst, src.students, s.today()),
		latest:   latestVisitsByStudent(src.visits),
		warnings: warnings,
		careDay:  careDay,
		pickups:  pickupTimes,
		parallel: parallelPresence,
	})
	s.sortRosterRows(rows)
	return s.rosterEnvelope(ctx, src.inst, rows, pickupTimesLoaded)
}

func (s *operations) loadRosterSources(ctx context.Context, instanceID int64) (rosterSources, error) {
	inst, err := s.loadInstance(ctx, instanceID)
	if err != nil {
		return rosterSources{}, err
	}
	src := rosterSources{inst: inst}
	if src.planned, err = s.deps.Participants.FindByInstanceID(ctx, instanceID); err != nil {
		return rosterSources{}, err
	}
	if inst.ActiveGroupID != nil {
		src.visits, err = s.deps.Visits.ListVisits(ctx, studentpresence.VisitFilter{ActiveGroupIDs: []int64{*inst.ActiveGroupID}})
		if err != nil {
			return rosterSources{}, err
		}
	}
	src.studentIDs = rosterStudentIDs(src.planned, src.visits)
	if src.students, err = s.deps.Students.FindByIDs(ctx, src.studentIDs); err != nil {
		return rosterSources{}, err
	}
	if src.template, err = s.loadRosterTemplateGroup(ctx, inst.ActivityGroupID); err != nil {
		return rosterSources{}, err
	}
	if err := s.loadRosterPeople(ctx, &src); err != nil {
		return rosterSources{}, err
	}
	return src, nil
}

// loadRosterPeople reads the children's persons and the education groups of
// the children and of the template in one read each.
func (s *operations) loadRosterPeople(ctx context.Context, src *rosterSources) error {
	groupIDs := make([]int64, 0, len(src.students))
	personIDs := make([]int64, 0, len(src.students))
	for _, st := range src.students {
		personIDs = append(personIDs, st.PersonID)
		if st.GroupID != nil {
			groupIDs = append(groupIDs, *st.GroupID)
		}
	}
	if src.template != nil && src.template.EducationGroupID != nil {
		groupIDs = append(groupIDs, *src.template.EducationGroupID)
	}
	var err error
	if src.persons, err = s.deps.People.GetByIDs(ctx, personIDs); err != nil {
		return err
	}
	src.groupNames, err = s.deps.EducationGroups.EducationGroupNames(ctx, groupIDs)
	return err
}

func rosterStudentIDs(planned []*scheduleModels.InstanceStudent, visits []studentpresence.Visit) []int64 {
	studentIDs := make([]int64, 0, len(planned)+len(visits))
	seen := map[int64]bool{}
	for _, row := range planned {
		if !seen[row.StudentID] {
			seen[row.StudentID] = true
			studentIDs = append(studentIDs, row.StudentID)
		}
	}
	for _, visit := range visits {
		if !seen[visit.StudentID] {
			seen[visit.StudentID] = true
			studentIDs = append(studentIDs, visit.StudentID)
		}
	}
	return studentIDs
}

func latestVisitsByStudent(visits []studentpresence.Visit) map[int64]*studentpresence.Visit {
	latest := map[int64]*studentpresence.Visit{}
	for _, visit := range visits {
		if current := latest[visit.StudentID]; current == nil || visit.EntryTime.After(current.EntryTime) {
			latest[visit.StudentID] = &visit
		}
	}
	return latest
}

// rosterRowContext is what every row of one roster is mapped against.
type rosterRowContext struct {
	excluded map[int64]bool
	latest   map[int64]*studentpresence.Visit
	warnings map[int64][]timetable.OperationRosterWarning
	careDay  map[int64]timetable.CareDayStatus
	pickups  map[int64]*time.Time
	parallel map[int64]*timetable.OperationParallelPresence
}

// rosterRows maps the planned children, then the walk-ins present in the
// session without a planned row.
func (s *operations) rosterRows(src rosterSources, rc rosterRowContext) []timetable.OperationRosterRow {
	rows := make([]timetable.OperationRosterRow, 0, len(src.studentIDs))
	for _, planned := range src.planned {
		if rc.excluded[planned.StudentID] {
			continue
		}
		rows = append(rows, s.rosterRow(src, planned.StudentID, planned, rc.latest[planned.StudentID], rc.warnings[planned.StudentID], rc))
	}
	for _, visit := range rc.latest {
		if rc.excluded[visit.StudentID] {
			continue
		}
		if _, planned := findPlanned(src.planned, visit.StudentID); planned {
			continue
		}
		rows = append(rows, s.rosterRow(src, visit.StudentID, nil, visit, nil, rc))
	}
	return rows
}

func (s *operations) rosterRow(src rosterSources, studentID int64, planned *scheduleModels.InstanceStudent, visit *studentpresence.Visit, warnings []timetable.OperationRosterWarning, rc rosterRowContext) timetable.OperationRosterRow {
	row := timetable.OperationRosterRow{
		StudentID:        studentID,
		Planned:          planned != nil && !planned.IsUnplanned,
		IsUnplanned:      (planned != nil && planned.IsUnplanned) || (planned == nil && visit != nil),
		CurrentlyPresent: visit != nil && visit.ExitTime == nil,
		Status:           scheduleModels.AttendanceStatusPresent,
		Warnings:         warnings,
		CareDayStatus:    s.rosterCareDayStatus(src.inst, studentID, planned, visit, rc.careDay),
	}
	applyPlannedRosterAttendance(&row, planned)
	if visit != nil {
		id := visit.ID
		row.VisitID = &id
		entry := visit.EntryTime.UTC().Format(time.RFC3339)
		row.VisitEntryTime = &entry
	}
	applyRosterStudentIdentity(&row, studentID, src)
	row.PickupTime = formatRosterPickupTime(rc.pickups[studentID])
	row.ParallelPresentIn = rc.parallel[studentID]
	return row
}

// sortRosterRows puts present children first, then the ones the care plan
// expects here today, then planned before walk-ins, then by name.
func (s *operations) sortRosterRows(rows []timetable.OperationRosterRow) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CurrentlyPresent != rows[j].CurrentlyPresent {
			return rows[i].CurrentlyPresent && !rows[j].CurrentlyPresent
		}
		if expectedI, expectedJ := s.deps.CareDays.Expected(rows[i].CareDayStatus), s.deps.CareDays.Expected(rows[j].CareDayStatus); expectedI != expectedJ {
			return expectedI && !expectedJ
		}
		if rows[i].Planned != rows[j].Planned {
			return rows[i].Planned && !rows[j].Planned
		}
		return rows[i].StudentName < rows[j].StudentName
	})
}

// rosterEnvelope wraps the rows with the block and its complete lifecycle.
func (s *operations) rosterEnvelope(ctx context.Context, inst *scheduleModels.ActivityInstance, rows []timetable.OperationRosterRow, pickupTimesLoaded bool) (*timetable.OperationRoster, error) {
	roomNames, err := s.roomNameMap(ctx)
	if err != nil {
		return nil, err
	}
	enforcePlannedEnd, err := s.deps.Settings.EnforcePlannedEnd(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve planned end policy: %v", timetable.ErrLifecycleSettings, err)
	}
	availability := timetable.EvaluateLifecycleAvailability(lifecycleWindow(inst), s.now(), 15, enforcePlannedEnd)
	return &timetable.OperationRoster{
		Instance: timetable.OperationRosterInstance{
			ID:                  inst.ID,
			Title:               inst.Title,
			Status:              inst.Status,
			IsSpontaneous:       inst.IsSpontaneous,
			ActiveGroupID:       inst.ActiveGroupID,
			RoomID:              inst.RoomID,
			RoomName:            roomNames[inst.RoomID],
			Date:                inst.Date.String(),
			StartTime:           inst.StartTime.Format("15:04"),
			EndTime:             inst.EndTime.Format("15:04"),
			CanComplete:         availability.CanComplete,
			CompleteAvailableAt: availability.CompleteAvailableAt.Format(time.RFC3339),
		},
		Rows:              rows,
		PickupTimesLoaded: pickupTimesLoaded,
	}, nil
}

// rosterExcludedAlumni returns the children to drop from a current or future
// roster: graduates and children whose care ended before the block. Frozen
// history — a past-dated, completed or cancelled block — excludes nobody
// (#405, #2487).
func rosterExcludedAlumni(inst *scheduleModels.ActivityInstance, students map[int64]*usersModels.Student, today timezone.Date) map[int64]bool {
	excluded := map[int64]bool{}
	if inst == nil {
		return excluded
	}
	if inst.Status == scheduleModels.InstanceStatusCompleted || inst.Status == scheduleModels.InstanceStatusCancelled {
		return excluded
	}
	if inst.Date.Before(today) {
		return excluded
	}
	for id, st := range students {
		if st != nil && (st.Status == usersModels.StudentStatusAlumnus || st.CareEndedOn(timezone.Date(inst.Date))) {
			excluded[id] = true
		}
	}
	return excluded
}

// rosterPickupTimes reads the children's effective pickup times; a failure
// degrades to a roster without them.
func (s *operations) rosterPickupTimes(ctx context.Context, inst *scheduleModels.ActivityInstance, studentIDs []int64) (map[int64]*time.Time, bool) {
	if len(studentIDs) == 0 {
		return map[int64]*time.Time{}, true
	}
	pickups, err := s.deps.Pickups.EffectivePickups(ctx, studentIDs, timezone.Date(inst.Date))
	if err != nil {
		s.logger().WarnContext(
			ctx,
			"could not load pickup times for timetable roster",
			slog.String("error", err.Error()),
			slog.Int64("instance_id", inst.ID),
		)
		return map[int64]*time.Time{}, false
	}
	return pickups, true
}

func formatRosterPickupTime(pickup *time.Time) *string {
	if pickup == nil {
		return nil
	}
	formatted := pickup.Format("15:04")
	return &formatted
}

// parallelPresenceByStudent resolves, for a running block's roster, which
// children are recorded present in another running block right now
// (#2265). Rows come newest block first, so the first row per child wins.
func (s *operations) parallelPresenceByStudent(ctx context.Context, inst *scheduleModels.ActivityInstance, studentIDs []int64) (map[int64]*timetable.OperationParallelPresence, error) {
	if inst.Status != scheduleModels.InstanceStatusActive || len(studentIDs) == 0 {
		return nil, nil
	}
	found, err := s.deps.Participants.FindPresentInOtherActiveInstances(ctx, inst.ID, inst.Date, studentIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*timetable.OperationParallelPresence, len(found))
	for _, row := range found {
		if _, taken := out[row.StudentID]; taken {
			continue
		}
		out[row.StudentID] = &timetable.OperationParallelPresence{
			InstanceID: row.InstanceID,
			Title:      row.Title,
			StartTime:  row.StartTime.Format("15:04"),
			EndTime:    row.EndTime.Format("15:04"),
		}
	}
	return out, nil
}

// rosterCareDayStatus resolves the care-day verdict of one roster row. A
// child who is actually here outranks any plan; a walk-in without a planned
// row is unknown; everything else is Care Plan's row rule, so this roster,
// the planner list and the planned-now cards never disagree (#1747).
func (s *operations) rosterCareDayStatus(inst *scheduleModels.ActivityInstance, studentID int64, planned *scheduleModels.InstanceStudent, visit *studentpresence.Visit, careDay map[int64]timetable.CareDayStatus) timetable.CareDayStatus {
	if visit != nil || (planned != nil && planned.Status == scheduleModels.AttendanceStatusPresent) {
		return timetable.CareDayScheduled
	}
	if planned == nil {
		return timetable.CareDayUnknown
	}
	completed := inst != nil && inst.Status == scheduleModels.InstanceStatusCompleted
	return s.deps.CareDays.AttendanceRowCareDay(completed, careDayAttendance(planned), careDay[studentID])
}

func applyPlannedRosterAttendance(row *timetable.OperationRosterRow, planned *scheduleModels.InstanceStudent) {
	if planned == nil {
		return
	}
	row.Status = planned.Status
	row.Substatus = planned.Substatus
	row.Note = planned.Note
	if planned.CheckedInAt != nil {
		checkedIn := planned.CheckedInAt.UTC().Format(time.RFC3339)
		row.CheckedInAt = &checkedIn
	}
	if planned.CheckedOutAt != nil {
		checkedOut := planned.CheckedOutAt.UTC().Format(time.RFC3339)
		row.CheckedOutAt = &checkedOut
	}
	if planned.Status == scheduleModels.AttendanceStatusPresent && planned.CheckedInAt != nil && planned.CheckedOutAt == nil {
		row.CurrentlyPresent = true
	}
}

func applyRosterStudentIdentity(row *timetable.OperationRosterRow, studentID int64, src rosterSources) {
	st := src.students[studentID]
	if st == nil {
		return
	}
	row.SchoolClass = st.SchoolClass
	if st.GroupID != nil {
		if name, ok := src.groupNames[*st.GroupID]; ok {
			row.GroupName = name
		}
	}
	if person := src.persons[st.PersonID]; person != nil {
		row.StudentName = person.GetFullName()
	}
}

// rosterTemplateGroup is the block's template with its dynamic targets.
type rosterTemplateGroup struct {
	*activitiesModels.Group
	Targets []*activitiesModels.GroupTarget
}

func (s *operations) loadRosterTemplateGroup(ctx context.Context, activityGroupID *int64) (*rosterTemplateGroup, error) {
	if activityGroupID == nil || *activityGroupID <= 0 {
		return nil, nil
	}
	group, err := s.deps.Templates.FindByID(ctx, *activityGroupID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	targetsByGroup, err := s.deps.Templates.FindTargetsByGroupIDs(ctx, []int64{*activityGroupID})
	if err != nil {
		return nil, err
	}
	return &rosterTemplateGroup{Group: group, Targets: targetsByGroup[*activityGroupID]}, nil
}
