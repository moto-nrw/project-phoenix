package application

import (
	"context"
	"errors"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	activityContract "github.com/moto-ogs/project-phoenix/modules/timetable/activitycontract"
	schedule "github.com/moto-ogs/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-ogs/project-phoenix/modules/timetable/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// sourcedInstanceBatch holds the shared roster snapshots and their diff.
type sourcedInstanceBatch struct {
	inputs    map[int64]timetable.SourcedTemplateRosterReconcile
	instances []*schedule.ActivityInstance
	passes    map[int64]*sourcedRosterPass
	existing  map[instanceStudentPair]*schedule.InstanceStudent
	creates   []timetable.InstanceStudentInput
	deletes   []int64
	touched   map[int64]calendar.Date
}

// ReconcileSourcedTemplateRosterBatch preserves the single-template coverage
// and observation rules while sharing snapshots and writes.
func (s *RosterReconciler) ReconcileSourcedTemplateRosterBatch(ctx context.Context, inputs []timetable.SourcedTemplateRosterReconcile) error {
	if len(inputs) == 0 {
		return nil
	}
	batch, err := s.loadSourcedInstanceBatch(ctx, inputs)
	if err != nil {
		return err
	}
	for _, inst := range batch.instances {
		if err := batch.planInstance(ctx, inst); err != nil {
			return err
		}
	}
	if err := s.saveSourcedInstanceBatch(ctx, batch); err != nil {
		return err
	}
	return s.applySourcedRosterStatusDays(ctx, batch.touched)
}

func (s *RosterReconciler) loadSourcedInstanceBatch(ctx context.Context, inputs []timetable.SourcedTemplateRosterReconcile) (*sourcedInstanceBatch, error) {
	batch := &sourcedInstanceBatch{inputs: map[int64]timetable.SourcedTemplateRosterReconcile{}, passes: map[int64]*sourcedRosterPass{}, touched: map[int64]calendar.Date{}}
	from := inputs[0].From
	groupIDs := make([]int64, 0, len(inputs))
	for _, in := range inputs {
		groupIDs = append(groupIDs, in.TemplateID)
		batch.inputs[in.TemplateID] = in
		if in.From.Before(from) {
			from = in.From
		}
	}
	if today := calendar.TodayDate(); from.Before(today) {
		from = today
	}
	fromText := from.String()
	instances, err := s.instanceRepo.Reads.ListLegacyInstances(ctx, ports.LegacyInstanceFilter{ActivityGroupIDs: groupIDs, FromDate: &fromText, MaterializedPlanned: true, OrderByDateAndTime: true})
	if err != nil {
		return nil, err
	}
	batch.instances = instances
	if len(instances) == 0 {
		return batch, nil
	}
	rows, err := groupsStudentEnrollments(ctx, s.enrollmentRepo, s.directories, groupIDs)
	if err != nil {
		return nil, err
	}
	instanceIDs := make([]int64, 0, len(instances))
	for _, inst := range instances {
		instanceIDs = append(instanceIDs, inst.ID)
	}
	batch.existing, err = s.existingStudentRows(ctx, instanceIDs, "reconcile sourced roster batch")
	if err != nil {
		return nil, err
	}
	byTemplate := map[int64][]*activityContract.StudentEnrollment{}
	for _, row := range rows {
		byTemplate[row.ActivityGroupID] = append(byTemplate[row.ActivityGroupID], row)
	}
	for _, in := range inputs {
		batch.passes[in.TemplateID] = &sourcedRosterPass{byStudent: enrollmentsByStudentID(byTemplate[in.TemplateID]), priorByStudent: enrollmentsByStudentID(rosterEnrollmentRows(in.Prior))}
	}
	return batch, nil
}

func (b *sourcedInstanceBatch) planInstance(ctx context.Context, inst *schedule.ActivityInstance) error {
	if inst.ActivityGroupID == nil {
		return nil
	}
	in := b.inputs[*inst.ActivityGroupID]
	date := calendar.Date(inst.Date)
	if date.Before(in.From) {
		return nil
	}
	weekday, err := calendar.PlanWeekday(ctx, date)
	if err != nil {
		return err
	}
	pass := b.passes[in.TemplateID]
	for _, sid := range in.StudentIDs {
		desired := enrollmentsPlanStudentOn(pass.byStudent[sid], date, calendarPeriodID(inst), weekday)
		key := instanceStudentPair{instanceID: inst.ID, studentID: sid}
		row, exists := b.existing[key]
		switch {
		case desired && !exists && !enrollmentsPlanStudentOn(pass.priorByStudent[sid], date, calendarPeriodID(inst), weekday):
			b.creates = append(b.creates, timetable.InstanceStudentInput{InstanceID: inst.ID, StudentID: sid})
			b.existing[key] = &schedule.InstanceStudent{InstanceID: inst.ID, StudentID: sid, Status: schedule.AttendanceStatusExpected}
			b.touched[inst.ID] = date
		case !desired && exists && instanceRowIsStillPlanned(row):
			b.deletes = append(b.deletes, row.ID)
			delete(b.existing, key)
		}
	}
	return nil
}

func (s *RosterReconciler) applySourcedRosterStatusDays(ctx context.Context, touched map[int64]calendar.Date) error {
	if len(touched) == 0 {
		return nil
	}
	attendance, ok := s.instanceStudentRepo.Presence.(studentpresence.SourcedRosterAttendance)
	if !ok {
		return errors.New("sourced roster batch: attendance batch commands are required")
	}
	dates := make(map[int64]string, len(touched))
	for id, date := range touched {
		dates[id] = date.String()
	}
	return attendance.ApplySourcedRosterAttendance(ctx, dates)
}

func (s *RosterReconciler) saveSourcedInstanceBatch(ctx context.Context, batch *sourcedInstanceBatch) error {
	return s.saveParticipantDiff(ctx, batch.creates, batch.deletes)
}

// saveParticipantDiff writes a planned-roster diff in one batch.
func (s *RosterReconciler) saveParticipantDiff(ctx context.Context, creates []timetable.InstanceStudentInput, deletes []int64) error {
	if len(creates)+len(deletes) == 0 {
		return nil
	}
	commands, ok := s.instanceStudentRepo.Plans.(timetable.SourcedInstanceStudents)
	if !ok {
		return errors.New("roster diff: participant diff commands are required")
	}
	return commands.SaveSourcedInstanceStudents(ctx, creates, deletes)
}
