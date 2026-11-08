package application

import (
	"context"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// loadTemplatesPeople loads the people the occurrences of several templates
// receive, keyed by template: enrollments, dynamic target students,
// supervisors and the children's care bounds (#5814). Each read runs once for
// all templates. The recurrence gate is already held and materialization
// writes none of these rows, so every template sees the rows its own reads
// would see after the gate. op prefixes the error operations of the caller.
func (s *materializationService) loadTemplatesPeople(ctx context.Context, templateIDs []int64, op string) (map[int64]templateRoster, error) {
	rosters := make(map[int64]templateRoster, len(templateIDs))
	if len(templateIDs) == 0 {
		return rosters, nil
	}
	enrollments, err := groupsStudentEnrollments(ctx, s.enrollmentRepo, s.directories, templateIDs)
	if err != nil {
		return nil, &ports.ScheduleError{Op: op + ": load enrollments", Err: err}
	}
	targets, err := s.groupRepo.ListTargetStudentIDs(ctx, templateIDs)
	if err != nil {
		return nil, &ports.ScheduleError{Op: op + ": load target students", Err: err}
	}
	supervisors, err := namedPlannedSupervisors(ctx, s.supervisorRepo, s.directories, templateIDs, "find by group ID")
	if err != nil {
		return nil, &ports.ScheduleError{Op: op + ": load supervisors", Err: err}
	}
	var allTargets []int64
	for _, id := range templateIDs {
		allTargets = append(allTargets, targets[id]...)
	}
	careBounds, err := s.loadCareBounds(ctx, allTargets, enrollments)
	if err != nil {
		return nil, &ports.ScheduleError{Op: op + ": load care bounds", Err: err}
	}
	for _, id := range templateIDs {
		rosters[id] = templateRoster{
			enrollments:      []*activityContract.StudentEnrollment{},
			targetStudentIDs: targets[id],
			supervisors:      []*activityContract.SupervisorPlanned{},
			careBounds:       careBounds,
		}
	}
	for _, enrollment := range enrollments {
		roster := rosters[enrollment.ActivityGroupID]
		roster.enrollments = append(roster.enrollments, enrollment)
		rosters[enrollment.ActivityGroupID] = roster
	}
	for _, supervisor := range supervisors {
		roster := rosters[supervisor.GroupID]
		roster.supervisors = append(roster.supervisors, supervisor)
		rosters[supervisor.GroupID] = roster
	}
	return rosters, nil
}
