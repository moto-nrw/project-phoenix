// Package timetable is the public Timetable & Activities capability. It owns
// activity categories, templates, target groups, schedules, rosters, and
// materialized activity instances.
package timetable

import (
	"context"
	"time"
)

func (m *Module) FindSchedule(ctx context.Context, id int64) (Schedule, error) {
	if id <= 0 {
		return Schedule{}, m.reject("find_schedule", ErrInvalidScheduleQuery)
	}
	return m.engine.FindSchedule(ctx, id)
}

func (m *Module) ListSchedules(ctx context.Context, filter ScheduleFilter) ([]Schedule, error) {
	return m.engine.ListSchedules(ctx, filter)
}

func (m *Module) FindTemplateStartTimes(ctx context.Context, groupIDs []int64) ([]TemplateStartTime, error) {
	return m.engine.FindTemplateStartTimes(ctx, groupIDs)
}

func (m *Module) CreateSchedule(ctx context.Context, input ScheduleInput) (Schedule, error) {
	if !normalizeSchedule(&input) {
		return Schedule{}, m.reject("create_schedule", ErrInvalidSchedule)
	}
	return m.engine.CreateSchedule(ctx, input)
}

func (m *Module) UpdateSchedule(ctx context.Context, id int64, input ScheduleInput) (Schedule, error) {
	if id <= 0 || !normalizeSchedule(&input) {
		return Schedule{}, m.reject("update_schedule", ErrInvalidSchedule)
	}
	return m.engine.UpdateSchedule(ctx, id, input)
}

func (m *Module) DeleteSchedule(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("delete_schedule", ErrInvalidSchedule)
	}
	return m.engine.DeleteSchedule(ctx, id)
}

func (m *Module) DeleteSchedulesByGroup(ctx context.Context, groupID int64) error {
	if groupID <= 0 {
		return m.reject("delete_schedules_by_group", ErrInvalidSchedule)
	}
	return m.engine.DeleteSchedulesByGroup(ctx, groupID)
}

func (m *Module) CapScheduleValidUntil(ctx context.Context, groupID int64, validUntil string) (int64, error) {
	if groupID <= 0 || !validDate(validUntil) {
		return 0, m.reject("cap_schedule_valid_until", ErrInvalidSchedule)
	}
	return m.engine.CapScheduleValidUntil(ctx, groupID, validUntil)
}

func (m *Module) FindPlannedSupervisor(ctx context.Context, id int64) (PlannedSupervisor, error) {
	if id <= 0 {
		return PlannedSupervisor{}, m.reject("find_planned_supervisor", ErrInvalidPlannedSupervisorQuery)
	}
	return m.engine.FindPlannedSupervisor(ctx, id)
}

func (m *Module) ListPlannedSupervisors(ctx context.Context, filter PlannedSupervisorFilter) ([]PlannedSupervisor, error) {
	if hasInvalidID(filter.GroupIDs) || (filter.StaffID != nil && *filter.StaffID <= 0) {
		return nil, m.reject("list_planned_supervisors", ErrInvalidPlannedSupervisorQuery)
	}
	return m.engine.ListPlannedSupervisors(ctx, filter)
}

func (m *Module) ListPlannedSupervisionBlockers(ctx context.Context, staffID, tenantID int64) ([]PlannedSupervisionBlocker, error) {
	if staffID <= 0 || tenantID <= 0 {
		return nil, m.reject("list_planned_supervision_blockers", ErrInvalidPlannedSupervisorQuery)
	}
	return m.engine.ListPlannedSupervisionBlockers(ctx, staffID, tenantID)
}

func (m *Module) CreatePlannedSupervisor(ctx context.Context, input PlannedSupervisorInput) (PlannedSupervisor, error) {
	if !normalizePlannedSupervisor(&input) {
		return PlannedSupervisor{}, m.reject("create_planned_supervisor", ErrInvalidPlannedSupervisor)
	}
	return m.engine.CreatePlannedSupervisor(ctx, input)
}

func (m *Module) UpdatePlannedSupervisor(ctx context.Context, id int64, input PlannedSupervisorInput) (PlannedSupervisor, error) {
	if id <= 0 || !normalizePlannedSupervisor(&input) {
		return PlannedSupervisor{}, m.reject("update_planned_supervisor", ErrInvalidPlannedSupervisor)
	}
	return m.engine.UpdatePlannedSupervisor(ctx, id, input)
}

func (m *Module) DeletePlannedSupervisor(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("delete_planned_supervisor", ErrInvalidPlannedSupervisor)
	}
	return m.engine.DeletePlannedSupervisor(ctx, id)
}

func (m *Module) SetPrimaryPlannedSupervisor(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("set_primary_planned_supervisor", ErrInvalidPlannedSupervisor)
	}
	return m.engine.SetPrimaryPlannedSupervisor(ctx, id)
}

func (m *Module) LockPlannedSupervisors(ctx context.Context) error {
	return m.engine.LockPlannedSupervisors(ctx)
}

func (m *Module) DeletePlannedSupervisorsByStaff(ctx context.Context, staffID int64) (int64, error) {
	if staffID <= 0 {
		return 0, m.reject("delete_planned_supervisors_by_staff", ErrInvalidPlannedSupervisor)
	}
	return m.engine.DeletePlannedSupervisorsByStaff(ctx, staffID)
}

func (m *Module) CapActivePlannedSupervisors(ctx context.Context, groupID int64, validUntil string) (int64, error) {
	if groupID <= 0 || !validDate(validUntil) {
		return 0, m.reject("cap_active_planned_supervisors", ErrInvalidPlannedSupervisor)
	}
	return m.engine.CapActivePlannedSupervisors(ctx, groupID, validUntil)
}

func (m *Module) SetPlannedSupervisorValidUntil(ctx context.Context, id int64, validUntil string) error {
	if id <= 0 || !validDate(validUntil) {
		return m.reject("set_planned_supervisor_valid_until", ErrInvalidPlannedSupervisor)
	}
	return m.engine.SetPlannedSupervisorValidUntil(ctx, id, validUntil)
}

func (m *Module) CloseOpenPlannedSupervisors(ctx context.Context, groupID int64, periodID *int64, validUntil string) error {
	if groupID <= 0 || (periodID != nil && *periodID <= 0) || !validDate(validUntil) {
		return m.reject("close_open_planned_supervisors", ErrInvalidPlannedSupervisor)
	}
	return m.engine.CloseOpenPlannedSupervisors(ctx, groupID, periodID, validUntil)
}

func (m *Module) FindStudentEnrollment(ctx context.Context, id int64) (StudentEnrollment, error) {
	if id <= 0 {
		return StudentEnrollment{}, m.reject("find_student_enrollment", ErrInvalidStudentEnrollmentQuery)
	}
	return m.engine.FindStudentEnrollment(ctx, id)
}

func (m *Module) ListStudentEnrollments(ctx context.Context, filter StudentEnrollmentFilter) ([]StudentEnrollment, error) {
	if hasInvalidID(filter.StudentIDs) || hasInvalidID(filter.ActivityGroupIDs) || filter.Limit < 0 || filter.Offset < 0 ||
		(filter.ActiveOn != nil && !validDate(*filter.ActiveOn)) {
		return nil, m.reject("list_student_enrollments", ErrInvalidStudentEnrollmentQuery)
	}
	return m.engine.ListStudentEnrollments(ctx, filter)
}

func (m *Module) CreateStudentEnrollment(ctx context.Context, input StudentEnrollmentInput) (StudentEnrollment, error) {
	if !validStudentEnrollment(input) {
		return StudentEnrollment{}, m.reject("create_student_enrollment", ErrInvalidStudentEnrollment)
	}
	return m.engine.CreateStudentEnrollment(ctx, input)
}

func (m *Module) UpdateStudentEnrollment(ctx context.Context, id int64, input StudentEnrollmentInput) (StudentEnrollment, error) {
	if id <= 0 || !validStudentEnrollment(input) {
		return StudentEnrollment{}, m.reject("update_student_enrollment", ErrInvalidStudentEnrollment)
	}
	return m.engine.UpdateStudentEnrollment(ctx, id, input)
}

func (m *Module) DeleteStudentEnrollment(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("delete_student_enrollment", ErrInvalidStudentEnrollment)
	}
	return m.engine.DeleteStudentEnrollment(ctx, id)
}

func (m *Module) BackfillStudentEnrollmentSource(ctx context.Context, studentID, requestChildID int64, groupIDs []int64) (int64, error) {
	if studentID <= 0 || requestChildID <= 0 || hasInvalidID(groupIDs) {
		return 0, m.reject("backfill_student_enrollment_source", ErrInvalidStudentEnrollment)
	}
	return m.engine.BackfillStudentEnrollmentSource(ctx, studentID, requestChildID, groupIDs)
}

func (m *Module) DeleteStudentEnrollmentsBySource(ctx context.Context, studentID, requestChildID int64) (int64, error) {
	if studentID <= 0 || requestChildID <= 0 {
		return 0, m.reject("delete_student_enrollments_by_source", ErrInvalidStudentEnrollment)
	}
	return m.engine.DeleteStudentEnrollmentsBySource(ctx, studentID, requestChildID)
}

func (m *Module) CapActiveStudentEnrollments(ctx context.Context, groupID int64, validUntil string) (int64, error) {
	if groupID <= 0 || !validDate(validUntil) {
		return 0, m.reject("cap_active_student_enrollments", ErrInvalidStudentEnrollment)
	}
	return m.engine.CapActiveStudentEnrollments(ctx, groupID, validUntil)
}

func (m *Module) SetStudentEnrollmentValidUntil(ctx context.Context, id int64, validUntil string) error {
	if id <= 0 || !validDate(validUntil) {
		return m.reject("set_student_enrollment_valid_until", ErrInvalidStudentEnrollment)
	}
	return m.engine.SetStudentEnrollmentValidUntil(ctx, id, validUntil)
}

func (m *Module) CloseOpenStudentEnrollments(ctx context.Context, groupID int64, periodID *int64, validUntil string) error {
	if groupID <= 0 || (periodID != nil && *periodID <= 0) || !validDate(validUntil) {
		return m.reject("close_open_student_enrollments", ErrInvalidStudentEnrollment)
	}
	return m.engine.CloseOpenStudentEnrollments(ctx, groupID, periodID, validUntil)
}

func (m *Module) FindTimeframe(ctx context.Context, id int64) (Timeframe, error) {
	if id <= 0 {
		return Timeframe{}, m.reject("find_timeframe", ErrInvalidTimeframeQuery)
	}
	return m.engine.FindTimeframe(ctx, id)
}

func (m *Module) ListTimeframes(ctx context.Context, filter TimeframeFilter) ([]Timeframe, error) {
	if filter.Limit < 0 || filter.Offset < 0 ||
		(filter.OverlapsStart != nil && !validClock(*filter.OverlapsStart)) ||
		(filter.OverlapsEnd != nil && !validClock(*filter.OverlapsEnd)) {
		return nil, m.reject("list_timeframes", ErrInvalidTimeframeQuery)
	}
	return m.engine.ListTimeframes(ctx, filter)
}

func (m *Module) CreateTimeframe(ctx context.Context, input TimeframeInput) (Timeframe, error) {
	if !validTimeframe(input) {
		return Timeframe{}, m.reject("create_timeframe", ErrInvalidTimeframe)
	}
	return m.engine.CreateTimeframe(ctx, input)
}

func (m *Module) UpdateTimeframe(ctx context.Context, id int64, input TimeframeInput) (Timeframe, error) {
	if id <= 0 || !validTimeframe(input) {
		return Timeframe{}, m.reject("update_timeframe", ErrInvalidTimeframe)
	}
	return m.engine.UpdateTimeframe(ctx, id, input)
}

func (m *Module) DeleteTimeframe(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("delete_timeframe", ErrInvalidTimeframe)
	}
	return m.engine.DeleteTimeframe(ctx, id)
}

func (m *Module) FindPlanningTrack(ctx context.Context, id int64) (PlanningTrack, error) {
	return m.findPlanningTrack(ctx, id, false)
}

func (m *Module) FindPlanningTrackForShare(ctx context.Context, id int64) (PlanningTrack, error) {
	return m.findPlanningTrack(ctx, id, true)
}

func (m *Module) findPlanningTrack(ctx context.Context, id int64, forShare bool) (PlanningTrack, error) {
	if id <= 0 {
		return PlanningTrack{}, m.reject("find_planning_track", ErrInvalidPlanningTrackQuery)
	}
	if forShare {
		return m.engine.FindPlanningTrackForShare(ctx, id)
	}
	return m.engine.FindPlanningTrack(ctx, id)
}

func (m *Module) ListPlanningTracks(ctx context.Context, filter PlanningTrackFilter) ([]PlanningTrack, error) {
	if hasInvalidID(filter.IDs) {
		return nil, m.reject("list_planning_tracks", ErrInvalidPlanningTrackQuery)
	}
	return m.engine.ListPlanningTracks(ctx, filter)
}

func (m *Module) CreatePlanningTrack(ctx context.Context, input PlanningTrackInput) (PlanningTrack, error) {
	input, ok := normalizePlanningTrack(input)
	if !ok {
		return PlanningTrack{}, m.reject("create_planning_track", ErrInvalidPlanningTrack)
	}
	return m.engine.CreatePlanningTrack(ctx, input)
}

func (m *Module) UpdatePlanningTrack(ctx context.Context, id int64, input PlanningTrackInput) (PlanningTrack, error) {
	input, ok := normalizePlanningTrack(input)
	if id <= 0 || !ok {
		return PlanningTrack{}, m.reject("update_planning_track", ErrInvalidPlanningTrack)
	}
	return m.engine.UpdatePlanningTrack(ctx, id, input)
}

func (m *Module) UpdateActivePlanningTrack(ctx context.Context, id int64, input PlanningTrackInput) (PlanningTrack, bool, error) {
	input, ok := normalizePlanningTrack(input)
	if id <= 0 || !ok {
		return PlanningTrack{}, false, m.reject("update_planning_track", ErrInvalidPlanningTrack)
	}
	return m.engine.UpdateActivePlanningTrack(ctx, id, input)
}

func (m *Module) DeletePlanningTrack(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("delete_planning_track", ErrInvalidPlanningTrack)
	}
	return m.engine.DeletePlanningTrack(ctx, id)
}

func (m *Module) SetPlanningTrackArchivedAt(ctx context.Context, id int64, value *time.Time) (PlanningTrack, bool, error) {
	if id <= 0 {
		return PlanningTrack{}, false, m.reject("archive_planning_track", ErrInvalidPlanningTrack)
	}
	return m.engine.SetPlanningTrackArchivedAt(ctx, id, value)
}

func (m *Module) ReorderPlanningTracks(ctx context.Context, ids []int64) error {
	if hasInvalidID(ids) || hasDuplicateID(ids) {
		return m.reject("reorder_planning_tracks", ErrInvalidPlanningTrack)
	}
	return m.engine.ReorderPlanningTracks(ctx, ids)
}

func (m *Module) RestorePlanningTrackAtEnd(ctx context.Context, id int64) (PlanningTrack, bool, error) {
	if id <= 0 {
		return PlanningTrack{}, false, m.reject("restore_planning_track", ErrInvalidPlanningTrack)
	}
	return m.engine.RestorePlanningTrackAtEnd(ctx, id)
}

func (m *Module) FindRecurrenceRule(ctx context.Context, id int64) (RecurrenceRule, error) {
	if id <= 0 {
		return RecurrenceRule{}, m.reject("find_recurrence_rule", ErrInvalidRecurrenceRuleQuery)
	}
	return m.engine.FindRecurrenceRule(ctx, id)
}

func (m *Module) ListRecurrenceRules(ctx context.Context, filter RecurrenceRuleFilter) ([]RecurrenceRule, error) {
	if filter.Limit < 0 || filter.Offset < 0 || !validRecurrenceRuleSort(filter.SortBy) {
		return nil, m.reject("list_recurrence_rules", ErrInvalidRecurrenceRuleQuery)
	}
	return m.engine.ListRecurrenceRules(ctx, filter)
}

func validRecurrenceRuleSort(value string) bool {
	return value == "" || value == "id" || value == "frequency" || value == "interval_count" ||
		value == "end_date" || value == "count" || value == "created_at" || value == "updated_at"
}

func (m *Module) CreateRecurrenceRule(ctx context.Context, input RecurrenceRuleInput) (RecurrenceRule, error) {
	input, ok := normalizeRecurrenceRule(input)
	if !ok {
		return RecurrenceRule{}, m.reject("create_recurrence_rule", ErrInvalidRecurrenceRule)
	}
	return m.engine.CreateRecurrenceRule(ctx, input)
}

func (m *Module) UpdateRecurrenceRule(ctx context.Context, id int64, input RecurrenceRuleInput) (RecurrenceRule, error) {
	input, ok := normalizeRecurrenceRule(input)
	if id <= 0 || !ok {
		return RecurrenceRule{}, m.reject("update_recurrence_rule", ErrInvalidRecurrenceRule)
	}
	return m.engine.UpdateRecurrenceRule(ctx, id, input)
}

func (m *Module) DeleteRecurrenceRule(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("delete_recurrence_rule", ErrInvalidRecurrenceRule)
	}
	return m.engine.DeleteRecurrenceRule(ctx, id)
}

func (m *Module) FindActivityException(ctx context.Context, id int64) (ActivityException, error) {
	if id <= 0 {
		return ActivityException{}, m.reject("find_activity_exception", ErrInvalidActivityExceptionQuery)
	}
	return m.engine.FindActivityException(ctx, id)
}

func (m *Module) ListActivityExceptions(ctx context.Context, filter ActivityExceptionFilter) ([]ActivityException, error) {
	if !validActivityExceptionFilter(filter) {
		return nil, m.reject("list_activity_exceptions", ErrInvalidActivityExceptionQuery)
	}
	return m.engine.ListActivityExceptions(ctx, filter)
}

func (m *Module) CountActivityExceptions(ctx context.Context, before *string) (int, error) {
	if !validOptionalDate(before) {
		return 0, m.reject("count_activity_exceptions", ErrInvalidActivityExceptionQuery)
	}
	return m.engine.CountActivityExceptions(ctx, before)
}

func (m *Module) OldestActivityExceptionBefore(ctx context.Context, before *string) (*string, error) {
	if !validOptionalDate(before) {
		return nil, m.reject("oldest_activity_exception", ErrInvalidActivityExceptionQuery)
	}
	return m.engine.OldestActivityExceptionBefore(ctx, before)
}

func (m *Module) CreateActivityException(ctx context.Context, input ActivityExceptionInput) (ActivityException, error) {
	if !validActivityException(input) {
		return ActivityException{}, m.reject("create_activity_exception", ErrInvalidActivityException)
	}
	return m.engine.CreateActivityException(ctx, input)
}

func (m *Module) UpdateActivityException(ctx context.Context, id int64, input ActivityExceptionInput) (ActivityException, error) {
	if id <= 0 || !validActivityException(input) {
		return ActivityException{}, m.reject("update_activity_exception", ErrInvalidActivityException)
	}
	return m.engine.UpdateActivityException(ctx, id, input)
}

func (m *Module) DeleteActivityException(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("delete_activity_exception", ErrInvalidActivityException)
	}
	return m.engine.DeleteActivityException(ctx, id)
}

func (m *Module) DeleteActivityExceptionsBefore(ctx context.Context, before string) (int64, error) {
	if !validDate(before) {
		return 0, m.reject("delete_activity_exceptions_before", ErrInvalidActivityException)
	}
	return m.engine.DeleteActivityExceptionsBefore(ctx, before)
}
