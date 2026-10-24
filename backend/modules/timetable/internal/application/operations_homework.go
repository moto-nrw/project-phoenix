package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Homework on the Lernzeit roster (#4796): a block of the list kind
// learning_time shows each child the homework its class has on the block's
// date and the feedback the OGS gave. Other blocks read nothing.

// isLearningTime reports a Lernzeit block.
func isLearningTime(inst *scheduleModels.ActivityInstance) bool {
	return inst != nil && inst.ListKind != nil && *inst.ListKind == timetable.ListKindLearningTime
}

// applyRosterHomework fills in each row's class homework and feedback with
// one read each.
func (s *operations) applyRosterHomework(ctx context.Context, inst *scheduleModels.ActivityInstance, rows []timetable.OperationRosterRow) error {
	if s.deps.Homework == nil || !isLearningTime(inst) || len(rows) == 0 {
		return nil
	}
	date := calendar.Date(inst.Date)
	classes := make([]string, 0, len(rows))
	studentIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		classes = append(classes, row.SchoolClass)
		studentIDs = append(studentIDs, row.StudentID)
	}
	entries, err := s.deps.Homework.ClassHomework(ctx, classes, date)
	if err != nil {
		return fmt.Errorf("roster homework: %w", err)
	}
	feedback, err := s.deps.Homework.HomeworkFeedbackOn(ctx, date, studentIDs)
	if err != nil {
		return fmt.Errorf("roster homework feedback: %w", err)
	}
	byClass := map[string][]timetable.OperationRosterHomework{}
	for _, entry := range entries {
		key := classArrivalKeys([]string{entry.SchoolClass})[0]
		byClass[key] = append(byClass[key], timetable.OperationRosterHomework{Subject: entry.Subject, Description: entry.Description})
	}
	byStudent := make(map[int64]*timetable.OperationHomeworkFeedback, len(feedback))
	for _, entry := range feedback {
		byStudent[entry.StudentID] = &timetable.OperationHomeworkFeedback{Status: entry.Status, Note: entry.Note}
	}
	for i := range rows {
		if keys := classArrivalKeys([]string{rows[i].SchoolClass}); len(keys) > 0 {
			rows[i].Homework = byClass[keys[0]]
		}
		rows[i].HomeworkFeedback = byStudent[rows[i].StudentID]
	}
	return nil
}

// canRecordHomeworkFeedback is the roster's verdict: a Lernzeit block that
// is neither cancelled nor a duty, for a caller who may edit its attendance.
func canRecordHomeworkFeedback(roster *timetable.OperationRoster) bool {
	instance := roster.Instance
	return roster.CanEditAttendance && instance.ListKind != nil && *instance.ListKind == timetable.ListKindLearningTime &&
		instance.Status != scheduleModels.InstanceStatusCancelled && !instance.IsDuty
}
