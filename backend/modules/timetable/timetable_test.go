package timetable_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingEngine struct {
	create     timetable.CreateCategory
	update     timetable.UpdateCategory
	group      timetable.GroupInput
	template   timetable.TemplateUpdate
	offering   timetable.OfferingSourceInput
	targets    []timetable.GroupTargetInput
	supervisor timetable.PlannedSupervisorInput
	timeframe  timetable.TimeframeInput
	track      timetable.PlanningTrackInput
	recurrence timetable.RecurrenceRuleInput
	calls      int
	rejections []string
}

func (e *recordingEngine) BindCarePlan(timetable.CarePlanDirectory) { e.calls++ }

func (e *recordingEngine) CountPlannedSupervisorsByCalendarPeriod(context.Context) (map[int64]int, error) {
	e.calls++
	return nil, nil
}

func (e *recordingEngine) LockInstanceStudentAssignments(context.Context, int64) error {
	e.calls++
	return nil
}
func (e *recordingEngine) RestoreInstanceStudentAttendance(context.Context, int64, []timetable.CompletionAttendance) error {
	e.calls++
	return nil
}

func (e *recordingEngine) CourseInstances(context.Context, string, string, string) ([]timetable.CourseInstanceRow, error) {
	e.calls++
	return nil, nil
}

func (e *recordingEngine) CourseParticipation(context.Context, string, string, string) ([]timetable.CourseParticipationRow, error) {
	e.calls++
	return nil, nil
}

func (e *recordingEngine) CountPlannedStudentAssignmentsAfter(ctx context.Context, studentIDs []int64, after string, removals []timetable.InstanceStudent) (map[int64]int, error) {
	e.calls++
	return nil, nil
}

func (e *recordingEngine) RemovePlannedStudentAssignmentsAfter(ctx context.Context, studentIDs []int64, after string) ([]timetable.InstanceStudent, error) {
	e.calls++
	return nil, nil
}

func (e *recordingEngine) RestoreCareExitStudentAssignments(ctx context.Context, studentIDs, roomIDs, statusDayIDs, pickupExceptionIDs []int64, removals []timetable.InstanceStudent) (int64, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) ListOpenStudentAssignments(ctx context.Context, studentIDs []int64) ([]int64, error) {
	e.calls++
	return nil, nil
}

func (e *recordingEngine) LatestStudentAssignmentAttendanceDate(ctx context.Context, studentID int64) (*string, error) {
	e.calls++
	return nil, nil
}

func (e *recordingEngine) CloseOpenStudentAssignments(ctx context.Context, studentIDs []int64, at time.Time) (int64, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) CountStudentAssignments(ctx context.Context, studentID int64) (int, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) DeleteStudentAssignments(ctx context.Context, studentID int64) (int64, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) LockOpenStudentAssignments(ctx context.Context, studentIDs []int64) error {
	e.calls++
	return nil
}

func (e *recordingEngine) LockPlannedStudentAssignmentsAfter(ctx context.Context, studentIDs []int64, after string) error {
	e.calls++
	return nil
}

func (e *recordingEngine) ReconnectCareExitAssignmentPickupExceptions(ctx context.Context, studentIDs, pickupExceptionIDs []int64, removals []timetable.InstanceStudent) error {
	e.calls++
	return nil
}

func (e *recordingEngine) ArchivePlannedInstanceStudents(context.Context, int64, []int64, string, time.Time) (int, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) RestoreArchivedInstanceStudents(context.Context, int64, []int64, string) (int, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) FindCategory(context.Context, int64) (timetable.Category, error) {
	e.calls++
	return timetable.Category{}, nil
}

func (e *recordingEngine) FindCategoryForAssignment(context.Context, int64) (timetable.Category, error) {
	e.calls++
	return timetable.Category{}, nil
}

func (e *recordingEngine) FindCategoryForShare(context.Context, int64) (timetable.Category, error) {
	e.calls++
	return timetable.Category{}, nil
}

func (e *recordingEngine) FindCategoryByName(context.Context, string) (timetable.Category, error) {
	e.calls++
	return timetable.Category{}, nil
}

func (e *recordingEngine) FindCategoryByNameForAssignment(context.Context, string) (timetable.Category, error) {
	e.calls++
	return timetable.Category{}, nil
}

func (e *recordingEngine) ListCategories(context.Context) ([]timetable.Category, error) {
	e.calls++
	return []timetable.Category{}, nil
}

func (e *recordingEngine) CountCategoryUsage(context.Context) (map[int64]int, error) {
	e.calls++
	return map[int64]int{}, nil
}

func (e *recordingEngine) FindGroup(context.Context, int64) (timetable.Group, error) {
	e.calls++
	return timetable.Group{}, nil
}

func (e *recordingEngine) FindGroupForUpdate(context.Context, int64) (timetable.Group, error) {
	e.calls++
	return timetable.Group{}, nil
}

func (e *recordingEngine) FindGroupByName(context.Context, string) (timetable.Group, error) {
	e.calls++
	return timetable.Group{}, nil
}

func (e *recordingEngine) ListGroups(context.Context, timetable.GroupFilter) ([]timetable.Group, error) {
	e.calls++
	return []timetable.Group{}, nil
}

func (e *recordingEngine) ListTemplateRows(context.Context, *int64) ([]timetable.TemplateListRow, error) {
	e.calls++
	return []timetable.TemplateListRow{}, nil
}

func (e *recordingEngine) ListTemplateRowsForTemplatePeriod(context.Context, int64, int64) ([]timetable.TemplateListRow, error) {
	e.calls++
	return []timetable.TemplateListRow{}, nil
}

func (e *recordingEngine) ListTemplateRowsForPeriod(context.Context, *int64) ([]timetable.TemplateListRow, error) {
	e.calls++
	return []timetable.TemplateListRow{}, nil
}

func (e *recordingEngine) ListTemplateWeekdayRoster(context.Context, *int64, *int64) ([]timetable.TemplateWeekdayRosterRow, error) {
	e.calls++
	return []timetable.TemplateWeekdayRosterRow{}, nil
}

func (e *recordingEngine) ListTemplateCapacityOccurrences(context.Context, *int64, []int64, []timetable.TemplateCapacityPeriod) ([]timetable.TemplateCapacityOccurrence, error) {
	e.calls++
	return []timetable.TemplateCapacityOccurrence{}, nil
}

func (e *recordingEngine) ListGroupTargets(context.Context, []int64) (map[int64][]timetable.GroupTarget, error) {
	e.calls++
	return map[int64][]timetable.GroupTarget{}, nil
}

func (e *recordingEngine) ListTargetStudentIDs(context.Context, []int64) (map[int64][]int64, error) {
	e.calls++
	return map[int64][]int64{}, nil
}

func (e *recordingEngine) FindSchedule(context.Context, int64) (timetable.Schedule, error) {
	e.calls++
	return timetable.Schedule{}, nil
}

func (e *recordingEngine) ListSchedules(context.Context, timetable.ScheduleFilter) ([]timetable.Schedule, error) {
	e.calls++
	return []timetable.Schedule{}, nil
}

func (e *recordingEngine) FindTemplateStartTimes(context.Context, []int64) ([]timetable.TemplateStartTime, error) {
	e.calls++
	return []timetable.TemplateStartTime{}, nil
}

func (e *recordingEngine) CreateSchedule(context.Context, timetable.ScheduleInput) (timetable.Schedule, error) {
	e.calls++
	return timetable.Schedule{}, nil
}

func (e *recordingEngine) UpdateSchedule(context.Context, int64, timetable.ScheduleInput) (timetable.Schedule, error) {
	e.calls++
	return timetable.Schedule{}, nil
}

func (e *recordingEngine) DeleteSchedule(context.Context, int64) error {
	e.calls++
	return nil
}

func (e *recordingEngine) DeleteSchedulesByGroup(context.Context, int64) error {
	e.calls++
	return nil
}

func (e *recordingEngine) CapScheduleValidUntil(context.Context, int64, string) (int64, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) FindPlannedSupervisor(context.Context, int64) (timetable.PlannedSupervisor, error) {
	e.calls++
	return timetable.PlannedSupervisor{}, nil
}

func (e *recordingEngine) ListPlannedSupervisors(context.Context, timetable.PlannedSupervisorFilter) ([]timetable.PlannedSupervisor, error) {
	e.calls++
	return []timetable.PlannedSupervisor{}, nil
}

func (e *recordingEngine) ListPlannedSupervisionBlockers(context.Context, int64, int64) ([]timetable.PlannedSupervisionBlocker, error) {
	e.calls++
	return []timetable.PlannedSupervisionBlocker{}, nil
}

func (e *recordingEngine) CreatePlannedSupervisor(_ context.Context, input timetable.PlannedSupervisorInput) (timetable.PlannedSupervisor, error) {
	e.calls++
	e.supervisor = input
	return timetable.PlannedSupervisor{}, nil
}

func (e *recordingEngine) UpdatePlannedSupervisor(context.Context, int64, timetable.PlannedSupervisorInput) (timetable.PlannedSupervisor, error) {
	e.calls++
	return timetable.PlannedSupervisor{}, nil
}

func (e *recordingEngine) DeletePlannedSupervisor(context.Context, int64) error {
	e.calls++
	return nil
}

func (e *recordingEngine) SetPrimaryPlannedSupervisor(context.Context, int64) error {
	e.calls++
	return nil
}

func (e *recordingEngine) DeletePlannedSupervisorsByStaff(context.Context, int64) (int64, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) CapActivePlannedSupervisors(context.Context, int64, string) (int64, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) SetPlannedSupervisorValidUntil(context.Context, int64, string) error {
	e.calls++
	return nil
}

func (e *recordingEngine) CloseOpenPlannedSupervisors(context.Context, int64, *int64, string) error {
	e.calls++
	return nil
}

func (e *recordingEngine) FindStudentEnrollment(context.Context, int64) (timetable.StudentEnrollment, error) {
	e.calls++
	return timetable.StudentEnrollment{}, nil
}

func (e *recordingEngine) ListStudentEnrollments(context.Context, timetable.StudentEnrollmentFilter) ([]timetable.StudentEnrollment, error) {
	e.calls++
	return []timetable.StudentEnrollment{}, nil
}

func (e *recordingEngine) CreateStudentEnrollment(context.Context, timetable.StudentEnrollmentInput) (timetable.StudentEnrollment, error) {
	e.calls++
	return timetable.StudentEnrollment{}, nil
}

func (e *recordingEngine) UpdateStudentEnrollment(context.Context, int64, timetable.StudentEnrollmentInput) (timetable.StudentEnrollment, error) {
	e.calls++
	return timetable.StudentEnrollment{}, nil
}

func (e *recordingEngine) DeleteStudentEnrollment(context.Context, int64) error {
	e.calls++
	return nil
}

func (e *recordingEngine) BackfillStudentEnrollmentSource(context.Context, int64, int64, []int64) (int64, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) DeleteStudentEnrollmentsBySource(context.Context, int64, int64) (int64, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) CapActiveStudentEnrollments(context.Context, int64, string) (int64, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) SetStudentEnrollmentValidUntil(context.Context, int64, string) error {
	e.calls++
	return nil
}

func (e *recordingEngine) CloseOpenStudentEnrollments(context.Context, int64, *int64, string) error {
	e.calls++
	return nil
}

func (e *recordingEngine) FindTimeframe(context.Context, int64) (timetable.Timeframe, error) {
	e.calls++
	return timetable.Timeframe{}, nil
}

func (e *recordingEngine) ListTimeframes(context.Context, timetable.TimeframeFilter) ([]timetable.Timeframe, error) {
	e.calls++
	return []timetable.Timeframe{}, nil
}

func (e *recordingEngine) CreateTimeframe(_ context.Context, input timetable.TimeframeInput) (timetable.Timeframe, error) {
	e.calls++
	e.timeframe = input
	return timetable.Timeframe{StartTime: input.StartTime, EndTime: input.EndTime}, nil
}

func (e *recordingEngine) UpdateTimeframe(context.Context, int64, timetable.TimeframeInput) (timetable.Timeframe, error) {
	e.calls++
	return timetable.Timeframe{}, nil
}

func (e *recordingEngine) DeleteTimeframe(context.Context, int64) error {
	e.calls++
	return nil
}

func (e *recordingEngine) FindPlanningTrack(context.Context, int64) (timetable.PlanningTrack, error) {
	e.calls++
	return timetable.PlanningTrack{}, nil
}

func (e *recordingEngine) FindPlanningTrackForShare(context.Context, int64) (timetable.PlanningTrack, error) {
	e.calls++
	return timetable.PlanningTrack{}, nil
}

func (e *recordingEngine) ListPlanningTracks(context.Context, timetable.PlanningTrackFilter) ([]timetable.PlanningTrack, error) {
	e.calls++
	return []timetable.PlanningTrack{}, nil
}

func (e *recordingEngine) CreatePlanningTrack(_ context.Context, input timetable.PlanningTrackInput) (timetable.PlanningTrack, error) {
	e.calls++
	e.track = input
	return timetable.PlanningTrack{Name: input.Name, Color: input.Color, SortOrder: input.SortOrder}, nil
}

func (e *recordingEngine) UpdatePlanningTrack(context.Context, int64, timetable.PlanningTrackInput) (timetable.PlanningTrack, error) {
	e.calls++
	return timetable.PlanningTrack{}, nil
}

func (e *recordingEngine) UpdateActivePlanningTrack(context.Context, int64, timetable.PlanningTrackInput) (timetable.PlanningTrack, bool, error) {
	e.calls++
	return timetable.PlanningTrack{}, true, nil
}

func (e *recordingEngine) DeletePlanningTrack(context.Context, int64) error {
	e.calls++
	return nil
}

func (e *recordingEngine) SetPlanningTrackArchivedAt(context.Context, int64, *time.Time) (timetable.PlanningTrack, bool, error) {
	e.calls++
	return timetable.PlanningTrack{}, true, nil
}

func (e *recordingEngine) ReorderPlanningTracks(context.Context, []int64) error {
	e.calls++
	return nil
}

func (e *recordingEngine) RestorePlanningTrackAtEnd(context.Context, int64) (timetable.PlanningTrack, bool, error) {
	e.calls++
	return timetable.PlanningTrack{}, true, nil
}

func (e *recordingEngine) FindRecurrenceRule(context.Context, int64) (timetable.RecurrenceRule, error) {
	e.calls++
	return timetable.RecurrenceRule{}, nil
}

func (e *recordingEngine) ListRecurrenceRules(context.Context, timetable.RecurrenceRuleFilter) ([]timetable.RecurrenceRule, error) {
	e.calls++
	return []timetable.RecurrenceRule{}, nil
}

func (e *recordingEngine) CreateRecurrenceRule(_ context.Context, input timetable.RecurrenceRuleInput) (timetable.RecurrenceRule, error) {
	e.calls++
	e.recurrence = input
	return timetable.RecurrenceRule{Frequency: input.Frequency, Weekdays: input.Weekdays}, nil
}

func (e *recordingEngine) UpdateRecurrenceRule(context.Context, int64, timetable.RecurrenceRuleInput) (timetable.RecurrenceRule, error) {
	e.calls++
	return timetable.RecurrenceRule{}, nil
}

func (e *recordingEngine) DeleteRecurrenceRule(context.Context, int64) error {
	e.calls++
	return nil
}

func (e *recordingEngine) FindActivityException(context.Context, int64) (timetable.ActivityException, error) {
	e.calls++
	return timetable.ActivityException{}, nil
}

func (e *recordingEngine) ListActivityExceptions(context.Context, timetable.ActivityExceptionFilter) ([]timetable.ActivityException, error) {
	e.calls++
	return []timetable.ActivityException{}, nil
}

func (e *recordingEngine) CountActivityExceptions(context.Context, *string) (int, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) OldestActivityExceptionBefore(context.Context, *string) (*string, error) {
	e.calls++
	return nil, nil
}

func (e *recordingEngine) CreateActivityException(context.Context, timetable.ActivityExceptionInput) (timetable.ActivityException, error) {
	e.calls++
	return timetable.ActivityException{}, nil
}

func (e *recordingEngine) UpdateActivityException(context.Context, int64, timetable.ActivityExceptionInput) (timetable.ActivityException, error) {
	e.calls++
	return timetable.ActivityException{}, nil
}

func (e *recordingEngine) DeleteActivityException(context.Context, int64) error {
	e.calls++
	return nil
}

func (e *recordingEngine) DeleteActivityExceptionsBefore(context.Context, string) (int64, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) FindActivityInstance(context.Context, int64) (timetable.ActivityInstance, error) {
	e.calls++
	return timetable.ActivityInstance{}, nil
}

func (e *recordingEngine) ListActivityInstances(context.Context, timetable.ActivityInstanceFilter) ([]timetable.ActivityInstance, error) {
	e.calls++
	return []timetable.ActivityInstance{}, nil
}

func (e *recordingEngine) MaxActivityInstanceID(context.Context) (int64, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) CountActivityInstances(context.Context, *string) (int, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) OldestActivityInstanceBefore(context.Context, *string) (*string, error) {
	e.calls++
	return nil, nil
}

func (e *recordingEngine) CreateActivityInstance(context.Context, timetable.ActivityInstanceInput) (timetable.ActivityInstance, error) {
	e.calls++
	return timetable.ActivityInstance{}, nil
}

func (e *recordingEngine) CreateTemplateBackedActivityInstanceIfAbsent(context.Context, timetable.ActivityInstanceInput) (timetable.ActivityInstance, bool, error) {
	e.calls++
	return timetable.ActivityInstance{}, false, nil
}

func (e *recordingEngine) CreateIdempotentActivityInstance(context.Context, timetable.ActivityInstanceInput) (timetable.ActivityInstance, bool, error) {
	e.calls++
	return timetable.ActivityInstance{}, false, nil
}

func (e *recordingEngine) UpdateActivityInstance(context.Context, int64, timetable.ActivityInstanceInput) (timetable.ActivityInstance, error) {
	e.calls++
	return timetable.ActivityInstance{}, nil
}

func (e *recordingEngine) PatchActivityInstance(context.Context, int64, timetable.ActivityInstanceInput, []string) (int64, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) DeleteActivityInstance(context.Context, int64) error {
	e.calls++
	return nil
}

func (e *recordingEngine) MarkActivityInstanceCompleted(context.Context, int64, time.Time) error {
	e.calls++
	return nil
}

func (e *recordingEngine) CompleteActiveActivityInstances(context.Context, []int64, time.Time) (int64, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) DeletePlannedActivityInstances(context.Context, string, *string, *int64, bool) (int64, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) DeleteRemovedWeekendActivityInstances(context.Context, int64, []int) (int64, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) PropagateActivityInstanceListKind(context.Context, int64, *string, *string, string) (int64, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) DeleteActivityInstancesBefore(context.Context, string) (int64, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) FindInstanceStaff(context.Context, int64) (timetable.InstanceStaff, error) {
	e.calls++
	return timetable.InstanceStaff{}, nil
}

func (e *recordingEngine) ListInstanceStaff(context.Context, timetable.InstanceStaffFilter) ([]timetable.InstanceStaff, error) {
	e.calls++
	return []timetable.InstanceStaff{}, nil
}

func (e *recordingEngine) CountNonAbsentInstanceStaff(context.Context, []int64) (map[int64]int, error) {
	e.calls++
	return map[int64]int{}, nil
}

func (e *recordingEngine) CreateInstanceStaff(context.Context, timetable.InstanceStaffInput) (timetable.InstanceStaff, error) {
	e.calls++
	return timetable.InstanceStaff{}, nil
}

func (e *recordingEngine) UpdateInstanceStaff(context.Context, int64, timetable.InstanceStaffInput) (timetable.InstanceStaff, error) {
	e.calls++
	return timetable.InstanceStaff{}, nil
}

func (e *recordingEngine) PatchInstanceStaff(context.Context, int64, timetable.InstanceStaffInput, []string) (int64, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) DeleteInstanceStaff(context.Context, int64) error {
	e.calls++
	return nil
}

func (e *recordingEngine) DeleteInstanceStaffByInstance(context.Context, int64) error {
	e.calls++
	return nil
}

func (e *recordingEngine) DeleteUpcomingInstanceStaff(context.Context, int64, string) (int64, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) FindInstanceStudent(context.Context, int64) (timetable.InstanceStudent, error) {
	e.calls++
	return timetable.InstanceStudent{}, nil
}

func (e *recordingEngine) ListInstanceStudents(context.Context, timetable.InstanceStudentFilter) ([]timetable.InstanceStudent, error) {
	e.calls++
	return []timetable.InstanceStudent{}, nil
}

func (e *recordingEngine) CountNonAbsentInstanceStudents(context.Context, []int64) (map[int64]int, error) {
	e.calls++
	return map[int64]int{}, nil
}

func (e *recordingEngine) ListParallelStudentPresence(context.Context, int64, string, []int64) ([]timetable.ParallelPresence, error) {
	e.calls++
	return []timetable.ParallelPresence{}, nil
}

func (e *recordingEngine) CreateInstanceStudent(context.Context, timetable.InstanceStudentInput) (timetable.InstanceStudent, error) {
	e.calls++
	return timetable.InstanceStudent{}, nil
}

func (e *recordingEngine) UpdateInstanceStudent(context.Context, int64, timetable.InstanceStudentInput) (timetable.InstanceStudent, error) {
	e.calls++
	return timetable.InstanceStudent{}, nil
}

func (e *recordingEngine) DeleteInstanceStudent(context.Context, int64) error {
	e.calls++
	return nil
}

func (e *recordingEngine) DeleteInstanceStudentsByInstance(context.Context, int64) error {
	e.calls++
	return nil
}

func (e *recordingEngine) UpdateAttendanceFromCheckin(context.Context, int64, int64, time.Time) (bool, error) {
	e.calls++
	return true, nil
}

func (e *recordingEngine) UpdateAttendanceFromCheckinBatch(context.Context, []timetable.InstanceStudentKey, time.Time) error {
	e.calls++
	return nil
}

func (e *recordingEngine) UpdateAttendanceCheckout(context.Context, int64, int64, time.Time) error {
	e.calls++
	return nil
}

func (e *recordingEngine) UpdateAttendanceCheckoutBatch(context.Context, []timetable.InstanceStudentKey, time.Time) error {
	e.calls++
	return nil
}

func (e *recordingEngine) CreateUnplannedPresentIfAbsent(context.Context, int64, int64, time.Time) (timetable.InstanceStudent, error) {
	e.calls++
	return timetable.InstanceStudent{}, nil
}

func (e *recordingEngine) ReconcileAttendanceInterval(context.Context, int64, int64, time.Time, *time.Time, time.Time, *time.Time) (bool, error) {
	e.calls++
	return true, nil
}

func (e *recordingEngine) ListStudentInstanceRefsBefore(context.Context, string) ([]timetable.StudentInstanceRef, error) {
	e.calls++
	return []timetable.StudentInstanceRef{}, nil
}

func (e *recordingEngine) ListScheduledInstancesForStudent(context.Context, int64, string, string) ([]timetable.ScheduledInstanceRow, error) {
	e.calls++
	return []timetable.ScheduledInstanceRow{}, nil
}

func (e *recordingEngine) HasPlannedStudentSlots(context.Context, string, string) (bool, error) {
	e.calls++
	return false, nil
}

func (e *recordingEngine) ListPlannedStudentIDs(context.Context, []int64, string) ([]int64, error) {
	e.calls++
	return []int64{}, nil
}

func (e *recordingEngine) ListPartialAbsenceBlocks(context.Context, int64, string, time.Time) ([]timetable.PartialAbsenceBlock, error) {
	e.calls++
	return []timetable.PartialAbsenceBlock{}, nil
}

func (e *recordingEngine) UpdateAttendanceFields(context.Context, int64, timetable.AttendanceFieldPatch) error {
	e.calls++
	return nil
}

func (e *recordingEngine) BulkUpdateStatus(context.Context, int64, string, string, []int64) (int, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) MarkNotScheduled(context.Context, []timetable.StudentInstanceRef) error {
	e.calls++
	return nil
}

func (e *recordingEngine) MarkExpectedAbsentByActiveGroupIDs(context.Context, []int64, time.Time, []timetable.StudentInstanceRef) error {
	e.calls++
	return nil
}

func (e *recordingEngine) CloseOpenCheckoutsByActiveGroupIDs(context.Context, []int64, time.Time) (int, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) ApplyStatusDay(context.Context, int64, string, int64, string) (int, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) ReleaseStatusDay(context.Context, int64) (int, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) ApplyActiveStatusDaysForInstance(context.Context, int64, string) (int, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) ApplyPartialAbsence(context.Context, int64) (int, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) ReleasePartialAbsence(context.Context, int64) (int, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) ApplyActivePartialAbsencesForInstance(context.Context, int64, string) (int, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) ReplaceGroupTargets(_ context.Context, _ int64, targets []timetable.GroupTargetInput) error {
	e.calls++
	e.targets = targets
	return nil
}

func (e *recordingEngine) CreateGroup(_ context.Context, input timetable.GroupInput) (timetable.Group, error) {
	e.calls++
	e.group = input
	return timetable.Group{Name: input.Name, Type: input.Type, TargetGroupType: input.TargetGroupType}, nil
}

func (e *recordingEngine) UpdateGroup(_ context.Context, _ int64, input timetable.GroupInput) (timetable.Group, error) {
	e.calls++
	e.group = input
	return timetable.Group{Name: input.Name, Type: input.Type, TargetGroupType: input.TargetGroupType}, nil
}

func (e *recordingEngine) DeleteGroup(context.Context, int64) error {
	e.calls++
	return nil
}

func (e *recordingEngine) UpdateTemplate(_ context.Context, _ int64, input timetable.TemplateUpdate) (int64, error) {
	e.calls++
	e.template = input
	return 1, nil
}

func (e *recordingEngine) ArchiveTemplate(context.Context, int64) (int64, error) {
	e.calls++
	return 1, nil
}

func (e *recordingEngine) UpdateGroupOfferingSource(_ context.Context, _ int64, input timetable.OfferingSourceInput) error {
	e.calls++
	e.offering = input
	return nil
}

func (e *recordingEngine) CreateCategory(_ context.Context, input timetable.CreateCategory) (timetable.Category, error) {
	e.calls++
	e.create = input
	return timetable.Category{Name: input.Name, Description: input.Description, Color: input.Color}, nil
}

func (e *recordingEngine) UpdateCategory(_ context.Context, input timetable.UpdateCategory) (timetable.Category, error) {
	e.calls++
	e.update = input
	return timetable.Category{ID: input.ID, Name: input.Name, Description: input.Description, Color: input.Color}, nil
}

func (e *recordingEngine) ArchiveCategory(context.Context, int64) (timetable.Category, error) {
	e.calls++
	return timetable.Category{}, nil
}

func (e *recordingEngine) RestoreCategory(context.Context, int64) (timetable.Category, error) {
	e.calls++
	return timetable.Category{}, nil
}

func (e *recordingEngine) DeleteCategory(context.Context, int64) error {
	e.calls++
	return nil
}

func (e *recordingEngine) SetCategoryShiftTypeID(context.Context, int64, *int64) error {
	e.calls++
	return nil
}

func (e *recordingEngine) SetCategoryShiftTypeLinks(context.Context, int64, []int64) error {
	e.calls++
	return nil
}

func (e *recordingEngine) LockStudentEnrollmentsForCareExit(context.Context, []int64, string) error {
	e.calls++
	return nil
}

func (e *recordingEngine) EndStudentEnrollmentsForCareExit(context.Context, []int64, string) (timetable.CareExitEnrollmentChanges, error) {
	e.calls++
	return timetable.CareExitEnrollmentChanges{}, nil
}

func (e *recordingEngine) RestoreStudentEnrollmentsForCareExit(context.Context, []int64, []int64, []timetable.CareExitEnrollmentRemoval) (int, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) ObserveRejection(operation string, _ time.Duration, _ error) {
	e.rejections = append(e.rejections, operation)
}

func TestActivityExceptionRejectsInvalidWrites(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := timetable.NewModule(engine)
	override := "13:00:00"
	cases := []timetable.ActivityExceptionInput{
		{},
		{ActivityGroupID: 1, ExceptionDate: "not-a-date", ExceptionType: timetable.ActivityExceptionCancelled},
		{ActivityGroupID: 1, ExceptionDate: "2027-01-01", ExceptionType: timetable.ActivityExceptionCancelled, StartTime: &override},
		{ActivityGroupID: 1, ExceptionDate: "2027-01-01", ExceptionType: timetable.ActivityExceptionModified},
	}
	for _, input := range cases {
		_, err := module.CreateActivityException(context.Background(), input)
		require.ErrorIs(t, err, timetable.ErrInvalidActivityException)
	}
	_, err := module.DeleteActivityExceptionsBefore(context.Background(), "not-a-date")
	require.ErrorIs(t, err, timetable.ErrInvalidActivityException)
	assert.Zero(t, engine.calls)
}

func TestActivityExceptionRejectsInvalidQueries(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := timetable.NewModule(engine)
	_, err := module.FindActivityException(context.Background(), 0)
	require.ErrorIs(t, err, timetable.ErrInvalidActivityExceptionQuery)
	badGroupID := int64(-1)
	_, err = module.ListActivityExceptions(context.Background(), timetable.ActivityExceptionFilter{ActivityGroupID: &badGroupID})
	require.ErrorIs(t, err, timetable.ErrInvalidActivityExceptionQuery)
	badDate := "not-a-date"
	_, err = module.CountActivityExceptions(context.Background(), &badDate)
	require.ErrorIs(t, err, timetable.ErrInvalidActivityExceptionQuery)
	assert.Zero(t, engine.calls)
}

func TestModuleNormalizesCategoryWrites(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := timetable.NewModule(engine)

	created, err := module.CreateCategory(context.Background(), timetable.CreateCategory{
		Name: "  Werken  ", Description: "  Holz  ", Color: "abc",
	})
	require.NoError(t, err)
	assert.Equal(t, "Werken", created.Name)
	assert.Equal(t, "Holz", created.Description)
	assert.Equal(t, "#abc", created.Color)
	assert.Equal(t, engine.create.Name, created.Name)

	updated, err := module.UpdateCategory(context.Background(), timetable.UpdateCategory{
		ID: 17, Name: "  Sport  ", Color: "#123456",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(17), updated.ID)
	assert.Equal(t, "Sport", engine.update.Name)
}

func TestModuleValidatesPlannedSupervisorBoundary(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := timetable.NewModule(engine)
	ctx := context.Background()

	_, err := module.CreatePlannedSupervisor(ctx, timetable.PlannedSupervisorInput{StaffID: 1, GroupID: 2, ValidFrom: "not-a-date"})
	require.ErrorIs(t, err, timetable.ErrInvalidPlannedSupervisor)
	assert.Zero(t, engine.calls)

	weekday := 7
	_, err = module.CreatePlannedSupervisor(ctx, timetable.PlannedSupervisorInput{StaffID: 1, GroupID: 2, Weekday: &weekday})
	require.NoError(t, err)
	assert.Equal(t, &weekday, engine.supervisor.Weekday)
}

func TestModuleValidatesStudentEnrollmentBoundary(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := timetable.NewModule(engine)
	ctx := context.Background()

	_, err := module.CreateStudentEnrollment(ctx, timetable.StudentEnrollmentInput{
		StudentID: 1, ActivityGroupID: 2, ValidFrom: "not-a-date",
	})
	require.ErrorIs(t, err, timetable.ErrInvalidStudentEnrollment)
	_, err = module.CreateStudentEnrollment(ctx, timetable.StudentEnrollmentInput{
		StudentID: 1, ActivityGroupID: 2, ValidFrom: "2026-09-01", SelectedWeekdays: []int{2, 2},
	})
	require.ErrorIs(t, err, timetable.ErrInvalidStudentEnrollment)
	_, err = module.ListStudentEnrollments(ctx, timetable.StudentEnrollmentFilter{StudentIDs: []int64{0}})
	require.ErrorIs(t, err, timetable.ErrInvalidStudentEnrollmentQuery)
	assert.Zero(t, engine.calls)
	assert.Equal(t, []string{
		"create_student_enrollment", "create_student_enrollment", "list_student_enrollments",
	}, engine.rejections)
}

func TestModuleValidatesTimeframeBoundary(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := timetable.NewModule(engine)
	ctx := context.Background()

	_, err := module.CreateTimeframe(ctx, timetable.TimeframeInput{StartTime: "not-a-clock"})
	require.ErrorIs(t, err, timetable.ErrInvalidTimeframe)
	_, err = module.CreateTimeframe(ctx, timetable.TimeframeInput{StartTime: "12:00:00", EndTime: ptr("08:00:00")})
	require.ErrorIs(t, err, timetable.ErrInvalidTimeframe)
	_, err = module.ListTimeframes(ctx, timetable.TimeframeFilter{Limit: -1})
	require.ErrorIs(t, err, timetable.ErrInvalidTimeframeQuery)
	assert.Zero(t, engine.calls)

	created, err := module.CreateTimeframe(ctx, timetable.TimeframeInput{StartTime: "08:00:00", EndTime: ptr("12:00:00")})
	require.NoError(t, err)
	assert.Equal(t, "08:00:00", created.StartTime)
	assert.Equal(t, "08:00:00", engine.timeframe.StartTime)
}

func TestModuleNormalizesAndValidatesPlanningTracks(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := timetable.NewModule(engine)
	ctx := context.Background()

	_, err := module.CreatePlanningTrack(ctx, timetable.PlanningTrackInput{Name: " ", Color: "#5080D8"})
	require.ErrorIs(t, err, timetable.ErrInvalidPlanningTrack)
	_, err = module.CreatePlanningTrack(ctx, timetable.PlanningTrackInput{Name: "Spur", Color: "blue"})
	require.ErrorIs(t, err, timetable.ErrInvalidPlanningTrack)
	err = module.ReorderPlanningTracks(ctx, []int64{1, 1})
	require.ErrorIs(t, err, timetable.ErrInvalidPlanningTrack)
	assert.Zero(t, engine.calls)

	created, err := module.CreatePlanningTrack(ctx, timetable.PlanningTrackInput{Name: " Spur ", Color: "#5080D8"})
	require.NoError(t, err)
	assert.Equal(t, "Spur", created.Name)
	assert.Equal(t, "Spur", engine.track.Name)
}

func TestModuleNormalizesAndValidatesRecurrenceRules(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := timetable.NewModule(engine)
	ctx := context.Background()

	_, err := module.CreateRecurrenceRule(ctx, timetable.RecurrenceRuleInput{Frequency: "never", IntervalCount: 1})
	require.ErrorIs(t, err, timetable.ErrInvalidRecurrenceRule)
	_, err = module.CreateRecurrenceRule(ctx, timetable.RecurrenceRuleInput{Frequency: "weekly", IntervalCount: 1, Weekdays: []string{"NOPE"}})
	require.ErrorIs(t, err, timetable.ErrInvalidRecurrenceRule)
	_, err = module.ListRecurrenceRules(ctx, timetable.RecurrenceRuleFilter{Limit: -1})
	require.ErrorIs(t, err, timetable.ErrInvalidRecurrenceRuleQuery)
	assert.Zero(t, engine.calls)

	created, err := module.CreateRecurrenceRule(ctx, timetable.RecurrenceRuleInput{
		Frequency: "WEEKLY", IntervalCount: 1, Weekdays: []string{"mon"},
	})
	require.NoError(t, err)
	assert.Equal(t, "weekly", created.Frequency)
	assert.Equal(t, []string{"MON"}, engine.recurrence.Weekdays)
}

func ptr[T any](value T) *T { return &value }

func TestSystemActivityNamesAreExact(t *testing.T) {
	t.Parallel()
	assert.True(t, timetable.IsSystemActivityName(timetable.SchulhofActivityName))
	assert.True(t, timetable.IsSystemActivityName(timetable.WCActivityName))
	assert.False(t, timetable.IsSystemActivityName("wc"))
}

func TestModuleNormalizesAndValidatesGroupWrites(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := timetable.NewModule(engine)
	ctx := context.Background()

	created, err := module.CreateGroup(ctx, timetable.GroupInput{Name: "Werkstatt", CategoryID: 7})
	require.NoError(t, err)
	assert.Equal(t, timetable.GroupTypeActivity, created.Type)
	assert.Equal(t, timetable.TargetGroupTypeNone, created.TargetGroupType)

	negative := -1
	_, err = module.UpdateGroup(ctx, 12, timetable.GroupInput{
		Name: "Werkstatt", CategoryID: 7, RequiredStaff: &negative,
	})
	require.ErrorIs(t, err, timetable.ErrInvalidGroup)
	assert.Equal(t, 1, engine.calls, "invalid write must not reach the engine")
	assert.Contains(t, engine.rejections, "update_group")
}

func TestModuleNormalizesAndValidatesTemplateWrites(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := timetable.NewModule(engine)
	ctx := context.Background()
	class := " 2b "

	_, err := module.UpdateTemplate(ctx, 9, timetable.TemplateUpdate{
		Name: "Template", CategoryID: 7, TargetGroupType: timetable.TargetGroupTypeSchoolClass,
		TargetSchoolClass: &class,
	})
	require.NoError(t, err)
	assert.Equal(t, timetable.GroupTypeActivity, engine.template.Type)
	assert.Equal(t, "2b", *engine.template.TargetSchoolClass)
	err = module.UpdateGroupOfferingSource(ctx, 9, timetable.OfferingSourceInput{
		CareOfferingIDs: []int64{4}, SchoolClasses: []string{" 3A "},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"3A"}, engine.offering.SchoolClasses)

	_, err = module.UpdateTemplate(ctx, 9, timetable.TemplateUpdate{Name: "", CategoryID: 7})
	require.ErrorIs(t, err, timetable.ErrInvalidGroup)
	err = module.UpdateGroupOfferingSource(ctx, 9, timetable.OfferingSourceInput{GradeLevels: []int{2}})
	require.ErrorIs(t, err, timetable.ErrInvalidGroup)
	_, err = module.ArchiveTemplate(ctx, 0)
	require.ErrorIs(t, err, timetable.ErrInvalidGroup)
	assert.Equal(t, 2, engine.calls, "invalid writes must not reach the engine")
}

func TestModuleRejectsInvalidAndReservedCategoryWrites(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := timetable.NewModule(engine)
	ctx := context.Background()

	_, err := module.CreateCategory(ctx, timetable.CreateCategory{Name: " ", Color: "#abc"})
	require.ErrorIs(t, err, timetable.ErrInvalidCategory)
	_, err = module.CreateCategory(ctx, timetable.CreateCategory{Name: "wc", Color: "#abc"})
	require.ErrorIs(t, err, timetable.ErrSystemCategoryName)
	_, err = module.UpdateCategory(ctx, timetable.UpdateCategory{ID: 3, Name: "Schulhof", Color: "#abc"})
	require.ErrorIs(t, err, timetable.ErrSystemCategoryName)
	_, err = module.UpdateCategory(ctx, timetable.UpdateCategory{ID: 0, Name: "Sport", Color: "#abc"})
	require.ErrorIs(t, err, timetable.ErrInvalidCategory)
	require.ErrorIs(t, module.SetCategoryShiftTypeLinks(ctx, 4, []int64{2, 0}), timetable.ErrInvalidCategory)
	require.ErrorIs(t, module.LockStudentEnrollmentsForCareExit(ctx, nil, "2026-09-04"), timetable.ErrInvalidCareExitEnrollment)
	_, err = module.EndStudentEnrollmentsForCareExit(ctx, []int64{1}, "04.09.2026")
	require.ErrorIs(t, err, timetable.ErrInvalidCareExitEnrollment)
	_, err = module.RestoreStudentEnrollmentsForCareExit(ctx, []int64{1}, nil, []timetable.CareExitEnrollmentRemoval{{
		CareExitEnrollment: timetable.CareExitEnrollment{ID: 1, TenantID: 7, StudentID: 1, ActivityGroupID: 1},
		WasDeleted:         true,
	}})
	require.ErrorIs(t, err, timetable.ErrInvalidCareExitEnrollment)

	assert.Zero(t, engine.calls, "invalid input must never reach persistence")
	assert.Equal(t, []string{
		"create_category", "create_category", "update_category", "update_category", "set_category_shift_type_links",
		"lock_student_enrollments_for_care_exit", "end_student_enrollments_for_care_exit",
		"restore_student_enrollments_for_care_exit",
	}, engine.rejections)
}

func TestModuleRejectsInvalidGroupQueries(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := timetable.NewModule(engine)
	ctx := context.Background()

	_, err := module.FindGroup(ctx, 0)
	require.ErrorIs(t, err, timetable.ErrInvalidGroupQuery)
	_, err = module.ListGroups(ctx, timetable.GroupFilter{IDs: []int64{1, 0}})
	require.ErrorIs(t, err, timetable.ErrInvalidGroupQuery)
	_, err = module.ListGroups(ctx, timetable.GroupFilter{SourceOfferingIDs: []int64{-1}})
	require.ErrorIs(t, err, timetable.ErrInvalidGroupQuery)
	invalidSeriesID := int64(0)
	_, err = module.ListGroups(ctx, timetable.GroupFilter{SeriesForGroupID: &invalidSeriesID})
	require.ErrorIs(t, err, timetable.ErrInvalidGroupQuery)
	_, err = module.ListGroupTargets(ctx, []int64{-1})
	require.ErrorIs(t, err, timetable.ErrInvalidGroupQuery)

	assert.Zero(t, engine.calls)
	assert.Equal(t, []string{"find_group", "list_groups", "list_groups", "list_groups", "list_group_targets"}, engine.rejections)
}

func TestModuleNormalizesAndRejectsGroupTargets(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := timetable.NewModule(engine)
	ctx := context.Background()
	class := " 2b "

	require.NoError(t, module.ReplaceGroupTargets(ctx, 7, []timetable.GroupTargetInput{{
		TargetGroupType: "klasse", TargetSchoolClass: &class,
	}}))
	require.Equal(t, 1, engine.calls)
	require.Len(t, engine.targets, 1)
	assert.Equal(t, "2b", *engine.targets[0].TargetSchoolClass)

	grade := int16(14)
	require.ErrorIs(t, module.ReplaceGroupTargets(ctx, 7, []timetable.GroupTargetInput{{
		TargetGroupType: "jahrgang", TargetGradeLevel: &grade,
	}}), timetable.ErrInvalidGroupTarget)
	require.ErrorIs(t, module.ReplaceGroupTargets(ctx, 0, nil), timetable.ErrInvalidGroupTarget)
	assert.Equal(t, 1, engine.calls, "invalid targets must not reach persistence")
}

func TestSystemProvisionerMayCreateReservedCategory(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := timetable.NewModule(engine)

	_, err := module.CreateCategory(context.Background(), timetable.CreateCategory{
		Name: timetable.WCCategoryName, IsSystem: true,
	})
	require.NoError(t, err)
	assert.True(t, engine.create.IsSystem)
}

func TestErrorCodeIsStable(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "none", timetable.ErrorCode(nil))
	assert.Equal(t, "not_found", timetable.ErrorCode(timetable.ErrCategoryNotFound))
	assert.Equal(t, "category_name_exists", timetable.ErrorCode(timetable.ErrCategoryNameExists))
	assert.Equal(t, "internal_error", timetable.ErrorCode(errors.New("boom")))
}

func TestNewModulePanicsWithoutEngine(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { timetable.NewModule(nil) })
}
