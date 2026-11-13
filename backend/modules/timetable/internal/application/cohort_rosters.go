package application

import (
	"context"
	"errors"
	"log/slog"
	"slices"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	activityContract "github.com/moto-ogs/project-phoenix/modules/timetable/activitycontract"
	"github.com/moto-ogs/project-phoenix/modules/timetable/internal/domain"
	schedule "github.com/moto-ogs/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-ogs/project-phoenix/modules/timetable/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// CohortRosters is the Timetable owner's timetable.CohortRosters (#6841): it
// moves children who changed Klasse or Gruppe between the already
// materialized occurrences of templates with a Klasse, Jahrgang or Gruppe
// target. It reuses the sourced-roster batch writes and the observation rule
// of the roster reconciler: only rows that are still purely a plan are
// removed.
type CohortRosters struct {
	deps    ports.CohortRosterDependencies
	rosters *RosterReconciler
}

var _ timetable.CohortRosters = (*CohortRosters)(nil)

// NewCohortRosters constructs the follow-up; CareBounds may be nil.
func NewCohortRosters(deps ports.CohortRosterDependencies) (*CohortRosters, error) {
	if deps.Groups == nil || deps.Students == nil || deps.Enrollments == nil || noInstanceRows(deps.Instances) ||
		noParticipantRows(deps.Participants) {
		return nil, errors.New("cohort rosters: groups, students, enrollments, instances and participants are required")
	}
	deps.Logger = OrDefaultLogger(deps.Logger)
	return &CohortRosters{
		deps:    deps,
		rosters: NewRosterReconciler(deps.Instances, deps.Participants, deps.Enrollments, deps.Directories, deps.Logger),
	}, nil
}

// cohortMove is one child joining (or leaving) one template's target.
type cohortMove struct {
	studentID int64
	joined    bool
}

// cohortRosterPass is the state one follow-up diffs against.
type cohortRosterPass struct {
	moves       map[int64][]cohortMove
	enrollments map[int64]map[int64][]*activityContract.StudentEnrollment
	careBounds  map[int64]calendar.Date
	existing    map[instanceStudentPair]*schedule.InstanceStudent
	creates     []timetable.InstanceStudentInput
	deletes     []int64
	touched     map[int64]calendar.Date
}

func (c *CohortRosters) FollowCohortChanges(ctx context.Context, changes []timetable.CohortChange, from calendar.Date) error {
	changes = relevantCohortChanges(changes)
	if len(changes) == 0 {
		return nil
	}
	moves, err := c.cohortMoves(ctx, changes, from)
	if err != nil || len(moves) == 0 {
		return err
	}
	templateIDs := cohortTemplateIDs(moves)
	fromText := from.String()
	instances, err := c.deps.Instances.Reads.ListLegacyInstances(ctx, ports.LegacyInstanceFilter{
		ActivityGroupIDs: templateIDs, FromDate: &fromText, MaterializedPlanned: true, OrderByDateAndTime: true,
	})
	if err != nil {
		return &ports.ScheduleError{Op: "follow cohort changes: load future instances", Err: err}
	}
	if len(instances) == 0 {
		return nil
	}
	pass, err := c.loadCohortRosterPass(ctx, moves, templateIDs, instances)
	if err != nil {
		return err
	}
	for _, inst := range instances {
		if err := pass.planInstance(ctx, inst); err != nil {
			return &ports.ScheduleError{Op: "follow cohort changes: resolve weekend plan", Err: err}
		}
	}
	if err := c.rosters.saveParticipantDiff(ctx, pass.creates, pass.deletes); err != nil {
		return &ports.ScheduleError{Op: "follow cohort changes: write rosters", Err: err}
	}
	if err := c.rosters.applySourcedRosterStatusDays(ctx, pass.touched); err != nil {
		return &ports.ScheduleError{Op: "follow cohort changes: apply student status days", Err: err}
	}
	if len(pass.creates) > 0 || len(pass.deletes) > 0 {
		c.deps.Logger.Info("reconciled materialized rosters after cohort change",
			slog.Int("students", len(changes)),
			slog.Int("templates", len(templateIDs)),
			slog.Int("rows_created", len(pass.creates)),
			slog.Int("rows_removed", len(pass.deletes)),
		)
	}
	return nil
}

// relevantCohortChanges keeps one change per child that names a write.
func relevantCohortChanges(changes []timetable.CohortChange) []timetable.CohortChange {
	seen := make(map[int64]bool, len(changes))
	result := make([]timetable.CohortChange, 0, len(changes))
	for _, change := range changes {
		if change.StudentID <= 0 || (!change.SchoolClassChanged && !change.GroupChanged) || seen[change.StudentID] {
			continue
		}
		seen[change.StudentID] = true
		result = append(result, change)
	}
	return result
}

// cohortMoves resolves, per template with a dynamic target, the children
// whose membership the change flips. A child the directory does not list as
// enrolled (or whose care ended before from) reaches no target before or
// after, so the write does not move them.
func (c *CohortRosters) cohortMoves(ctx context.Context, changes []timetable.CohortChange, from calendar.Date) (map[int64][]cohortMove, error) {
	targets, err := c.dynamicTargets(ctx)
	if err != nil || len(targets) == 0 {
		return nil, err
	}
	students, err := c.deps.Students.ListEnrolledStudents(ctx)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "follow cohort changes: load students", Err: err}
	}
	current := make(map[int64]domain.TargetStudent, len(changes))
	for _, student := range students {
		current[student.ID] = domain.TargetStudent{
			ID: student.ID, SchoolClass: student.SchoolClass, EducationGroupID: student.EducationGroupID, EnrolledUntil: student.EnrolledUntil,
		}
	}
	moves := make(map[int64][]cohortMove)
	for _, change := range changes {
		now, ok := current[change.StudentID]
		if !ok || careEndedBefore(now.EnrolledUntil, from.String()) {
			continue
		}
		before := previousTargetStudent(now, change)
		for templateID, rules := range targets {
			was, is := matchesAnyTarget(rules, before), matchesAnyTarget(rules, now)
			if was != is {
				moves[templateID] = append(moves[templateID], cohortMove{studentID: change.StudentID, joined: is})
			}
		}
	}
	return moves, nil
}

// dynamicTargets lists the target rules of every template that has one.
func (c *CohortRosters) dynamicTargets(ctx context.Context) (map[int64][]timetable.GroupTarget, error) {
	isTemplate := true
	templates, err := c.deps.Groups.ListGroups(ctx, timetable.GroupFilter{IsTemplate: &isTemplate})
	if err != nil {
		return nil, &ports.ScheduleError{Op: "follow cohort changes: list templates", Err: err}
	}
	if len(templates) == 0 {
		return nil, nil
	}
	ids := make([]int64, 0, len(templates))
	for _, template := range templates {
		ids = append(ids, template.ID)
	}
	targets, err := c.deps.Groups.ListGroupTargets(ctx, ids)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "follow cohort changes: list targets", Err: err}
	}
	for id, rules := range targets {
		if len(rules) == 0 {
			delete(targets, id)
		}
	}
	return targets, nil
}

func previousTargetStudent(now domain.TargetStudent, change timetable.CohortChange) domain.TargetStudent {
	before := now
	if change.SchoolClassChanged {
		before.SchoolClass = change.PreviousSchoolClass
	}
	if change.GroupChanged {
		before.EducationGroupID = change.PreviousGroupID
	}
	return before
}

func (c *CohortRosters) loadCohortRosterPass(
	ctx context.Context, moves map[int64][]cohortMove, templateIDs []int64, instances []*schedule.ActivityInstance,
) (*cohortRosterPass, error) {
	rows, err := groupsStudentEnrollments(ctx, c.deps.Enrollments, c.deps.Directories, templateIDs)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "follow cohort changes: load enrollments", Err: err}
	}
	enrollments := make(map[int64]map[int64][]*activityContract.StudentEnrollment, len(templateIDs))
	for _, row := range rows {
		if enrollments[row.ActivityGroupID] == nil {
			enrollments[row.ActivityGroupID] = make(map[int64][]*activityContract.StudentEnrollment)
		}
		enrollments[row.ActivityGroupID][row.StudentID] = append(enrollments[row.ActivityGroupID][row.StudentID], row)
	}
	var careBounds map[int64]calendar.Date
	if c.deps.CareBounds != nil {
		if careBounds, err = c.deps.CareBounds.FindCareBoundsByIDs(ctx, movedStudentIDs(moves)); err != nil {
			return nil, &ports.ScheduleError{Op: "follow cohort changes: load care bounds", Err: err}
		}
	}
	instanceIDs := make([]int64, 0, len(instances))
	for _, inst := range instances {
		instanceIDs = append(instanceIDs, inst.ID)
	}
	existing, err := c.rosters.existingStudentRows(ctx, instanceIDs, "follow cohort changes: load existing rows")
	if err != nil {
		return nil, err
	}
	return &cohortRosterPass{
		moves: moves, enrollments: enrollments, careBounds: careBounds, existing: existing, touched: make(map[int64]calendar.Date),
	}, nil
}

// planInstance diffs one occurrence for the children moved on its template.
// A child an enrollment of the template plans on the date stays as is: the
// target never decided that row.
func (p *cohortRosterPass) planInstance(ctx context.Context, inst *schedule.ActivityInstance) error {
	if inst == nil || inst.ActivityGroupID == nil {
		return nil
	}
	templateID := *inst.ActivityGroupID
	date := calendar.Date(inst.Date)
	weekday, err := calendar.PlanWeekday(ctx, date)
	if err != nil {
		return err
	}
	for _, move := range p.moves[templateID] {
		if careEndedOnDate(p.careBounds, move.studentID, date) ||
			enrollmentsPlanStudentOn(p.enrollments[templateID][move.studentID], date, calendarPeriodID(inst), weekday) {
			continue
		}
		key := instanceStudentPair{instanceID: inst.ID, studentID: move.studentID}
		row, exists := p.existing[key]
		switch {
		case move.joined && !exists:
			p.creates = append(p.creates, timetable.InstanceStudentInput{InstanceID: inst.ID, StudentID: move.studentID})
			p.existing[key] = &schedule.InstanceStudent{InstanceID: inst.ID, StudentID: move.studentID, Status: schedule.AttendanceStatusExpected}
			p.touched[inst.ID] = date
		case !move.joined && exists && instanceRowIsStillPlanned(row):
			p.deletes = append(p.deletes, row.ID)
			delete(p.existing, key)
		}
	}
	return nil
}

func movedStudentIDs(moves map[int64][]cohortMove) []int64 {
	ids := make([]int64, 0)
	for _, list := range moves {
		for _, move := range list {
			ids = append(ids, move.studentID)
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

func cohortTemplateIDs(values map[int64][]cohortMove) []int64 {
	keys := make([]int64, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
