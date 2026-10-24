package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type Store interface {
	ListReplannableActivityInstanceIDs(context.Context, string, *string, *int64, bool) ([]int64, domain.OperationStats, error)
	DeleteActivityInstancesByID(context.Context, []int64) (int64, domain.OperationStats, error)
	ListRemovedWeekendActivityInstanceIDs(context.Context, int64, []int, string) ([]int64, domain.OperationStats, error)
	ListFutureTemplateActivityInstanceIDs(context.Context, int64, *string, string) ([]int64, domain.OperationStats, error)
	SetActivityInstanceListKind(context.Context, []int64, *string, time.Time) (int64, domain.OperationStats, error)
	ListPlannedInstanceStudents(context.Context, timetable.InstanceStudentFilter) ([]timetable.PlannedInstanceStudent, domain.OperationStats, error)
	EnsureInstanceStudent(context.Context, int64, int64) (timetable.InstanceStudent, bool, domain.OperationStats, error)
	LockInstanceStudentsByID(context.Context, []int64) (domain.OperationStats, error)
	ArchivePlannedInstanceStudents(context.Context, int64, []timetable.RosterArchiveEntry) (int64, domain.OperationStats, error)
	InsertRestoredInstanceStudents(context.Context, []timetable.InstanceStudentInput) ([]timetable.InstanceStudent, domain.OperationStats, error)
	ListPlannedRosterForCareExit(context.Context, []int64, string) ([]timetable.CareExitRosterRow, domain.OperationStats, error)
	RemovePlannedRosterForCareExit(context.Context, []int64) ([]timetable.CareExitRosterRow, domain.OperationStats, error)
	RestoreRosterForCareExit(context.Context, []int64, []timetable.CareExitRosterRow, []int64) ([]timetable.CareExitRosterRow, domain.OperationStats, error)
	ListRestorableRosterForCareExit(context.Context, []int64, string, []timetable.CareExitRosterRow) ([]timetable.CareExitRosterRow, domain.OperationStats, error)
	PreviewStaffOffboarding(context.Context, int64, string) ([]domain.OffboardingInstance, []timetable.PlannedSupervisor, domain.OperationStats, error)
	CountCalendarPeriodReferences(context.Context) (map[int64]timetable.CalendarPeriodReferences, domain.OperationStats, error)
	LockInstanceStudentAssignments(context.Context, int64) (domain.OperationStats, error)
	CountStudentAssignments(context.Context, int64) (int, domain.OperationStats, error)
	CountStudentRosterRemovals(context.Context, int64) (int, domain.OperationStats, error)
	DeleteStudentAssignments(context.Context, int64) (domain.OperationStats, error)
	CountRunningEnrollmentsForCareExit(context.Context, []int64, string, []timetable.CareExitEnrollmentRemoval) (map[int64]int, domain.OperationStats, error)
	CountRestorableEnrollmentsForCareExit(context.Context, []int64, string, []timetable.CareExitEnrollmentRemoval) (map[int64]int, domain.OperationStats, error)
	FindCategory(context.Context, int64, string) (timetable.Category, bool, domain.OperationStats, error)
	FindCategoryByName(context.Context, string, bool, string) (timetable.Category, bool, domain.OperationStats, error)
	ListCategories(context.Context) ([]timetable.Category, domain.OperationStats, error)
	CountCategoryUsage(context.Context) (map[int64]int, domain.OperationStats, error)
	CreateCategory(context.Context, timetable.CreateCategory) (timetable.Category, domain.OperationStats, error)
	UpdateCategoryIfActive(context.Context, int64, timetable.CreateCategory) (timetable.Category, bool, domain.OperationStats, error)
	SetCategoryArchivedAt(context.Context, int64, *time.Time) (timetable.Category, bool, domain.OperationStats, error)
	SetCategoryShiftTypeLinks(context.Context, int64, []int64) (domain.OperationStats, error)
	SetCategoryShiftTypeID(context.Context, int64, *int64) (domain.OperationStats, error)
	DeleteCategory(context.Context, int64) (domain.OperationStats, error)
	LockStudentEnrollmentsForCareExit(context.Context, []int64, string) (domain.OperationStats, error)
	EndStudentEnrollmentsForCareExit(context.Context, []int64, string) (timetable.CareExitEnrollmentChanges, domain.OperationStats, error)
	FindGroup(context.Context, int64, string) (timetable.Group, bool, domain.OperationStats, error)
	FindGroupByName(context.Context, string) (timetable.Group, bool, domain.OperationStats, error)
	ListGroups(context.Context, timetable.GroupFilter) ([]timetable.Group, domain.OperationStats, error)
	ListCourseGroups(context.Context, timetable.CourseGroupFilter) ([]timetable.CourseGroup, domain.OperationStats, error)
	ListTemplateRows(context.Context, *int64, string) ([]timetable.TemplateListRow, domain.OperationStats, error)
	ListTemplateRowsForTemplatePeriod(context.Context, int64, int64, string) ([]timetable.TemplateListRow, domain.OperationStats, error)
	ListTemplateRowsForPeriod(context.Context, *int64, string) ([]timetable.TemplateListRow, domain.OperationStats, error)
	ListTemplateWeekdayRoster(context.Context, *int64, *int64, string) ([]timetable.TemplateWeekdayRosterRow, domain.OperationStats, error)
	ListTemplateCapacityOccurrences(context.Context, *int64, []int64, []timetable.TemplateCapacityPeriod, map[int64][]int64) ([]timetable.TemplateCapacityOccurrence, domain.OperationStats, error)
	CreateGroup(context.Context, timetable.GroupInput) (timetable.Group, domain.OperationStats, error)
	UpdateGroup(context.Context, int64, timetable.GroupInput) (timetable.Group, bool, domain.OperationStats, error)
	DeleteGroup(context.Context, int64) (domain.OperationStats, error)
	UpdateTemplate(context.Context, int64, timetable.TemplateUpdate) (int64, domain.OperationStats, error)
	ArchiveTemplate(context.Context, int64) (int64, domain.OperationStats, error)
	UpdateGroupOfferingSource(context.Context, int64, timetable.OfferingSourceInput) (domain.OperationStats, error)
	ListGroupTargets(context.Context, []int64) (map[int64][]timetable.GroupTarget, domain.OperationStats, error)
	ReplaceGroupTargets(context.Context, int64, []timetable.GroupTargetInput) (domain.OperationStats, error)
	FindSchedule(context.Context, int64) (timetable.Schedule, bool, domain.OperationStats, error)
	ListSchedules(context.Context, timetable.ScheduleFilter) ([]timetable.Schedule, domain.OperationStats, error)
	FindTemplateStartTimes(context.Context, []int64) ([]timetable.TemplateStartTime, domain.OperationStats, error)
	CreateSchedule(context.Context, timetable.ScheduleInput) (timetable.Schedule, domain.OperationStats, error)
	UpdateSchedule(context.Context, int64, timetable.ScheduleInput) (timetable.Schedule, bool, domain.OperationStats, error)
	DeleteSchedule(context.Context, int64) (domain.OperationStats, error)
	DeleteSchedulesByGroup(context.Context, int64) (domain.OperationStats, error)
	CapScheduleValidUntil(context.Context, int64, string) (int64, domain.OperationStats, error)
	FindPlannedSupervisor(context.Context, int64) (timetable.PlannedSupervisor, bool, domain.OperationStats, error)
	ListPlannedSupervisors(context.Context, timetable.PlannedSupervisorFilter) ([]timetable.PlannedSupervisor, domain.OperationStats, error)
	ListPlannedSupervisionBlockers(context.Context, int64) ([]timetable.PlannedSupervisionBlocker, domain.OperationStats, error)
	CreatePlannedSupervisor(context.Context, timetable.PlannedSupervisorInput) (timetable.PlannedSupervisor, domain.OperationStats, error)
	UpdatePlannedSupervisor(context.Context, int64, timetable.PlannedSupervisorInput) (timetable.PlannedSupervisor, bool, domain.OperationStats, error)
	DeletePlannedSupervisor(context.Context, int64) (domain.OperationStats, error)
	SetPrimaryPlannedSupervisor(context.Context, int64) (bool, domain.OperationStats, error)
	DeletePlannedSupervisorsByStaff(context.Context, int64) (int64, domain.OperationStats, error)
	LockPlannedSupervisors(context.Context) (domain.OperationStats, error)
	CapActivePlannedSupervisors(context.Context, int64, string) (int64, domain.OperationStats, error)
	SetPlannedSupervisorValidUntil(context.Context, int64, string) (bool, domain.OperationStats, error)
	CloseOpenPlannedSupervisors(context.Context, int64, *int64, string) (domain.OperationStats, error)
	FindStudentEnrollment(context.Context, int64) (timetable.StudentEnrollment, bool, domain.OperationStats, error)
	ListStudentEnrollments(context.Context, timetable.StudentEnrollmentFilter) ([]timetable.StudentEnrollment, domain.OperationStats, error)
	CreateStudentEnrollment(context.Context, timetable.StudentEnrollmentInput) (timetable.StudentEnrollment, domain.OperationStats, error)
	UpdateStudentEnrollment(context.Context, int64, timetable.StudentEnrollmentInput) (timetable.StudentEnrollment, bool, domain.OperationStats, error)
	DeleteStudentEnrollment(context.Context, int64) (domain.OperationStats, error)
	DeleteStudentEnrollmentsBySource(context.Context, int64, int64) (int64, domain.OperationStats, error)
	CapActiveStudentEnrollments(context.Context, int64, string) (int64, domain.OperationStats, error)
	SetStudentEnrollmentValidUntil(context.Context, int64, string) (bool, domain.OperationStats, error)
	CloseOpenStudentEnrollments(context.Context, int64, *int64, string) (domain.OperationStats, error)
	FindTimeframe(context.Context, int64) (timetable.Timeframe, bool, domain.OperationStats, error)
	ListTimeframes(context.Context, timetable.TimeframeFilter) ([]timetable.Timeframe, domain.OperationStats, error)
	CreateTimeframe(context.Context, timetable.TimeframeInput) (timetable.Timeframe, domain.OperationStats, error)
	UpdateTimeframe(context.Context, int64, timetable.TimeframeInput) (timetable.Timeframe, bool, domain.OperationStats, error)
	FindPlanningTrack(context.Context, int64, string) (timetable.PlanningTrack, bool, domain.OperationStats, error)
	ListPlanningTracks(context.Context, timetable.PlanningTrackFilter) ([]timetable.PlanningTrack, domain.OperationStats, error)
	CreatePlanningTrack(context.Context, timetable.PlanningTrackInput) (timetable.PlanningTrack, domain.OperationStats, error)
	UpdatePlanningTrack(context.Context, int64, timetable.PlanningTrackInput, bool) (timetable.PlanningTrack, bool, domain.OperationStats, error)
	DeletePlanningTrack(context.Context, int64) (domain.OperationStats, error)
	SetPlanningTrackArchivedAt(context.Context, int64, *time.Time) (timetable.PlanningTrack, bool, domain.OperationStats, error)
	ReorderPlanningTracks(context.Context, []int64) (domain.OperationStats, error)
	RestorePlanningTrackAtEnd(context.Context, int64) (timetable.PlanningTrack, bool, domain.OperationStats, error)
	FindActivityException(context.Context, int64) (timetable.ActivityException, bool, domain.OperationStats, error)
	ListActivityExceptions(context.Context, timetable.ActivityExceptionFilter) ([]timetable.ActivityException, domain.OperationStats, error)
	CountActivityExceptions(context.Context, *string) (int, domain.OperationStats, error)
	OldestActivityExceptionBefore(context.Context, *string) (*string, domain.OperationStats, error)
	CreateActivityException(context.Context, timetable.ActivityExceptionInput) (timetable.ActivityException, domain.OperationStats, error)
	UpdateActivityException(context.Context, int64, timetable.ActivityExceptionInput) (timetable.ActivityException, bool, domain.OperationStats, error)
	DeleteActivityException(context.Context, int64) (domain.OperationStats, error)
	DeleteActivityExceptionsBefore(context.Context, string) (int64, domain.OperationStats, error)
	FindActivityInstance(context.Context, int64) (timetable.ActivityInstance, bool, domain.OperationStats, error)
	LockActivityInstance(context.Context, int64, bool) (timetable.ActivityInstance, bool, domain.OperationStats, error)
	ListActivityInstances(context.Context, timetable.ActivityInstanceFilter) ([]timetable.ActivityInstance, domain.OperationStats, error)
	MaxActivityInstanceID(context.Context) (int64, domain.OperationStats, error)
	CountActivityInstances(context.Context, *string) (int, domain.OperationStats, error)
	OldestActivityInstanceBefore(context.Context, *string) (*string, domain.OperationStats, error)
	CreateActivityInstance(context.Context, timetable.ActivityInstanceInput) (timetable.ActivityInstance, domain.OperationStats, error)
	CreateTemplateBackedActivityInstanceIfAbsent(context.Context, timetable.ActivityInstanceInput) (timetable.ActivityInstance, bool, domain.OperationStats, error)
	CreateIdempotentActivityInstance(context.Context, timetable.ActivityInstanceInput) (timetable.ActivityInstance, bool, domain.OperationStats, error)
	UpdateActivityInstance(context.Context, int64, timetable.ActivityInstanceInput) (timetable.ActivityInstance, bool, domain.OperationStats, error)
	PatchActivityInstance(context.Context, int64, timetable.ActivityInstanceInput, []string) (int64, domain.OperationStats, error)
	DeleteActivityInstance(context.Context, int64) (domain.OperationStats, error)
	DeleteActivityInstancesBefore(context.Context, string) (int64, domain.OperationStats, error)
	FindInstanceStaff(context.Context, int64) (timetable.InstanceStaff, bool, domain.OperationStats, error)
	ListInstanceStaff(context.Context, timetable.InstanceStaffFilter) ([]timetable.InstanceStaff, domain.OperationStats, error)
	CountNonAbsentInstanceStaff(context.Context, []int64) (map[int64]int, domain.OperationStats, error)
	CreateInstanceStaff(context.Context, timetable.InstanceStaffInput) (timetable.InstanceStaff, domain.OperationStats, error)
	UpdateInstanceStaff(context.Context, int64, timetable.InstanceStaffInput) (timetable.InstanceStaff, bool, domain.OperationStats, error)
	PatchInstanceStaff(context.Context, int64, timetable.InstanceStaffInput, []string) (int64, domain.OperationStats, error)
	DeleteInstanceStaff(context.Context, int64) (domain.OperationStats, error)
	DeleteInstanceStaffByInstance(context.Context, int64) (domain.OperationStats, error)
	DeleteInstanceStaffByStaff(context.Context, int64) (int64, domain.OperationStats, error)
	DeleteUpcomingInstanceStaff(context.Context, int64, string, []int64) (int64, domain.OperationStats, error)
	FindInstanceStudent(context.Context, int64) (timetable.InstanceStudent, bool, domain.OperationStats, error)
	ListInstanceStudents(context.Context, timetable.InstanceStudentFilter) ([]timetable.InstanceStudent, domain.OperationStats, error)
	ListStudentInstanceRefsBefore(context.Context, string) ([]timetable.StudentInstanceRef, domain.OperationStats, error)
	ListPlannedStudentIDs(context.Context, []int64, string) ([]int64, domain.OperationStats, error)
	CreateInstanceStudent(context.Context, timetable.InstanceStudentInput) (timetable.InstanceStudent, domain.OperationStats, error)
	UpdateInstanceStudent(context.Context, int64, timetable.InstanceStudentInput) (timetable.InstanceStudent, bool, domain.OperationStats, error)
	DeleteInstanceStudent(context.Context, int64) (domain.OperationStats, error)
	DeleteInstanceStudentsByInstance(context.Context, int64) (domain.OperationStats, error)
	ListConflictAckFingerprints(context.Context, int64) ([]string, domain.OperationStats, error)
	InsertConflictAck(context.Context, int64, string) (bool, domain.OperationStats, error)
	PruneConflictAcks(context.Context, int64, int) (int64, domain.OperationStats, error)
	DeleteConflictAck(context.Context, int64, string) (domain.OperationStats, error)
	UpsertPickupExtensionTask(context.Context, domain.PickupExtensionTask) (domain.OperationStats, error)
	DeletePickupDayExtensionTask(context.Context, int64, domain.Date) (domain.OperationStats, error)
	DeletePickupWeekdayExtensionTask(context.Context, int64, int) (domain.OperationStats, error)
	DeletePastPickupExtensionTasks(context.Context, domain.Date) (domain.OperationStats, error)
	ListPickupExtensionTasks(context.Context, int64, domain.Date) ([]domain.PickupExtensionTask, domain.OperationStats, error)
	FindPickupExtensionTask(context.Context, int64) (domain.PickupExtensionTask, bool, domain.OperationStats, error)
	FindPickupExtensionTaskForUpdate(context.Context, int64) (domain.PickupExtensionTask, bool, domain.OperationStats, error)
	DeletePickupExtensionTask(context.Context, int64) (domain.OperationStats, error)
	ListPickupExtensionDayBlocks(context.Context, []domain.PickupExtensionTask) ([]domain.PickupExtensionBlock, domain.OperationStats, error)
	ListPickupExtensionWeekdayBlocks(context.Context, []domain.PickupExtensionTask, domain.Date) ([]domain.PickupExtensionBlock, domain.OperationStats, error)
	ListPickupExtensionTemplateInstances(context.Context, int64, int64, int, domain.Date, *int64, *domain.Date) ([]domain.PickupExtensionInstance, domain.OperationStats, error)
	ConsumeRosterRemovals(context.Context, int64, []int64) ([]domain.RosterRemoval, domain.OperationStats, error)
	HomeworkStore
}

// HomeworkStore persists the homework of the classes and the OGS feedback on
// it (#4796) in education.class_homework and education.homework_feedback.
// Class keys are trimmed and lowercased; an entry is changed or deleted only
// by its author, so a foreign entry reads as not found.
type HomeworkStore interface {
	ListClassHomework(ctx context.Context, classKeys []string, date calendar.Date) ([]timetable.HomeworkEntry, domain.OperationStats, error)
	InsertClassHomework(ctx context.Context, in timetable.HomeworkInput) (timetable.HomeworkEntry, domain.OperationStats, error)
	UpdateOwnClassHomework(ctx context.Context, in timetable.HomeworkChange, classKey string) (timetable.HomeworkEntry, bool, domain.OperationStats, error)
	DeleteOwnClassHomework(ctx context.Context, ref timetable.HomeworkRef, classKey string) (bool, domain.OperationStats, error)
	ListHomeworkFeedback(ctx context.Context, date calendar.Date, studentIDs []int64) ([]timetable.HomeworkFeedback, domain.OperationStats, error)
	UpsertHomeworkFeedback(ctx context.Context, in timetable.HomeworkFeedbackInput) (timetable.HomeworkFeedback, domain.OperationStats, error)
	// DeleteHomeworkBefore deletes the tenant's entries and feedback dated
	// before cutoff and returns how many rows went.
	DeleteHomeworkBefore(ctx context.Context, cutoff calendar.Date) (int, domain.OperationStats, error)
}

type Transaction interface {
	RunWrite(context.Context, bool, func(context.Context) error) error
}

type StudentDirectory interface {
	ListEnrolledStudents(context.Context) ([]domain.TargetStudent, domain.OperationStats, error)
}

type RoomDirectory interface {
	LockRoomsByID(context.Context, []int64) ([]timetable.RoomRef, domain.OperationStats, error)
}

// SessionFacts is the consumer-owned port to Student Presence: which planned
// rows already carry an execution or observed presence (#2762).
type SessionFacts interface {
	StartedInstanceIDs(context.Context, []int64) ([]int64, error)
	CompletedInstanceIDs(context.Context, []int64) ([]int64, error)
	ObservedParticipantIDs(context.Context, []int64) ([]int64, error)
	NotScheduledParticipantIDs(context.Context, []int64) ([]int64, error)
	ExecutionFacts(ctx context.Context, instanceIDs, participantIDs []int64) (completed, notScheduled []int64, err error)
	RunningSessionsByInstance(ctx context.Context, sessionIDs []int64) (map[int64]int64, error)
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)

// EnrollmentProvenanceStore writes the enrollment links whose eligibility the
// enrollment provenance projection decides (ADR 0058).
type EnrollmentProvenanceStore interface {
	BackfillStudentEnrollmentSource(context.Context, int64, int64, []int64) (int64, domain.OperationStats, error)
	RestoreStudentEnrollmentsForCareExit(context.Context, []int64, []int64, []timetable.CareExitEnrollmentRemoval) (int64, domain.OperationStats, error)
}
