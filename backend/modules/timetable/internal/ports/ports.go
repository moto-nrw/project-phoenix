package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

type Store interface {
	FindCategory(context.Context, int64, string) (domain.Category, bool, domain.OperationStats, error)
	FindCategoryByName(context.Context, string, bool, string) (domain.Category, bool, domain.OperationStats, error)
	ListCategories(context.Context) ([]domain.Category, domain.OperationStats, error)
	CountCategoryUsage(context.Context) (map[int64]int, domain.OperationStats, error)
	CreateCategory(context.Context, domain.CategoryFields) (domain.Category, domain.OperationStats, error)
	UpdateCategoryIfActive(context.Context, int64, domain.CategoryFields) (domain.Category, bool, domain.OperationStats, error)
	SetCategoryArchivedAt(context.Context, int64, *time.Time) (domain.Category, bool, domain.OperationStats, error)
	SetCategoryShiftTypeLinks(context.Context, int64, []int64) (domain.OperationStats, error)
	LockStudentEnrollmentsForCareExit(context.Context, []int64, string) (domain.OperationStats, error)
	EndStudentEnrollmentsForCareExit(context.Context, []int64, string) (domain.CareExitEnrollmentChanges, domain.OperationStats, error)
	RestoreStudentEnrollmentsForCareExit(context.Context, []int64, []int64, []domain.CareExitEnrollmentRemoval) (int64, domain.OperationStats, error)
	FindGroup(context.Context, int64, string) (domain.Group, bool, domain.OperationStats, error)
	FindGroupByName(context.Context, string) (domain.Group, bool, domain.OperationStats, error)
	ListGroups(context.Context, domain.GroupFilter) ([]domain.Group, domain.OperationStats, error)
	CreateGroup(context.Context, domain.GroupFields) (domain.Group, domain.OperationStats, error)
	UpdateGroup(context.Context, int64, domain.GroupFields) (domain.Group, bool, domain.OperationStats, error)
	DeleteGroup(context.Context, int64) (domain.OperationStats, error)
	UpdateTemplate(context.Context, int64, domain.TemplateFields) (int64, domain.OperationStats, error)
	ArchiveTemplate(context.Context, int64) (int64, domain.OperationStats, error)
	UpdateGroupOfferingSource(context.Context, int64, domain.OfferingSourceFields) (domain.OperationStats, error)
	ListGroupTargets(context.Context, []int64) (map[int64][]domain.GroupTarget, domain.OperationStats, error)
	ReplaceGroupTargets(context.Context, int64, []domain.GroupTargetFields) (domain.OperationStats, error)
	FindSchedule(context.Context, int64) (domain.Schedule, bool, domain.OperationStats, error)
	ListSchedules(context.Context, domain.ScheduleFilter) ([]domain.Schedule, domain.OperationStats, error)
	FindTemplateStartTimes(context.Context, []int64) ([]domain.TemplateStartTime, domain.OperationStats, error)
	CreateSchedule(context.Context, domain.ScheduleFields) (domain.Schedule, domain.OperationStats, error)
	UpdateSchedule(context.Context, int64, domain.ScheduleFields) (domain.Schedule, bool, domain.OperationStats, error)
	DeleteSchedule(context.Context, int64) (domain.OperationStats, error)
	DeleteSchedulesByGroup(context.Context, int64) (domain.OperationStats, error)
	CapScheduleValidUntil(context.Context, int64, string) (int64, domain.OperationStats, error)
	FindPlannedSupervisor(context.Context, int64) (domain.PlannedSupervisor, bool, domain.OperationStats, error)
	ListPlannedSupervisors(context.Context, domain.PlannedSupervisorFilter) ([]domain.PlannedSupervisor, domain.OperationStats, error)
	ListPlannedSupervisionBlockers(context.Context, int64) ([]domain.PlannedSupervisionBlocker, domain.OperationStats, error)
	CreatePlannedSupervisor(context.Context, domain.PlannedSupervisorFields) (domain.PlannedSupervisor, domain.OperationStats, error)
	UpdatePlannedSupervisor(context.Context, int64, domain.PlannedSupervisorFields) (domain.PlannedSupervisor, bool, domain.OperationStats, error)
	DeletePlannedSupervisor(context.Context, int64) (domain.OperationStats, error)
	SetPrimaryPlannedSupervisor(context.Context, int64) (bool, domain.OperationStats, error)
	DeletePlannedSupervisorsByStaff(context.Context, int64) (int64, domain.OperationStats, error)
	CapActivePlannedSupervisors(context.Context, int64, string) (int64, domain.OperationStats, error)
	SetPlannedSupervisorValidUntil(context.Context, int64, string) (bool, domain.OperationStats, error)
	CloseOpenPlannedSupervisors(context.Context, int64, *int64, string) (domain.OperationStats, error)
	FindStudentEnrollment(context.Context, int64) (domain.StudentEnrollment, bool, domain.OperationStats, error)
	ListStudentEnrollments(context.Context, domain.StudentEnrollmentFilter) ([]domain.StudentEnrollment, domain.OperationStats, error)
	CreateStudentEnrollment(context.Context, domain.StudentEnrollmentFields) (domain.StudentEnrollment, domain.OperationStats, error)
	UpdateStudentEnrollment(context.Context, int64, domain.StudentEnrollmentFields) (domain.StudentEnrollment, bool, domain.OperationStats, error)
	DeleteStudentEnrollment(context.Context, int64) (domain.OperationStats, error)
	BackfillStudentEnrollmentSource(context.Context, int64, int64, []int64) (int64, domain.OperationStats, error)
	DeleteStudentEnrollmentsBySource(context.Context, int64, int64) (int64, domain.OperationStats, error)
	CapActiveStudentEnrollments(context.Context, int64, string) (int64, domain.OperationStats, error)
	SetStudentEnrollmentValidUntil(context.Context, int64, string) (bool, domain.OperationStats, error)
	CloseOpenStudentEnrollments(context.Context, int64, *int64, string) (domain.OperationStats, error)
	FindTimeframe(context.Context, int64) (domain.Timeframe, bool, domain.OperationStats, error)
	ListTimeframes(context.Context, domain.TimeframeFilter) ([]domain.Timeframe, domain.OperationStats, error)
	CreateTimeframe(context.Context, domain.TimeframeFields) (domain.Timeframe, domain.OperationStats, error)
	UpdateTimeframe(context.Context, int64, domain.TimeframeFields) (domain.Timeframe, bool, domain.OperationStats, error)
	DeleteTimeframe(context.Context, int64) (domain.OperationStats, error)
	FindPlanningTrack(context.Context, int64, string) (domain.PlanningTrack, bool, domain.OperationStats, error)
	ListPlanningTracks(context.Context, domain.PlanningTrackFilter) ([]domain.PlanningTrack, domain.OperationStats, error)
	CreatePlanningTrack(context.Context, domain.PlanningTrackFields) (domain.PlanningTrack, domain.OperationStats, error)
	UpdatePlanningTrack(context.Context, int64, domain.PlanningTrackFields, bool) (domain.PlanningTrack, bool, domain.OperationStats, error)
	DeletePlanningTrack(context.Context, int64) (domain.OperationStats, error)
	SetPlanningTrackArchivedAt(context.Context, int64, *time.Time) (domain.PlanningTrack, bool, domain.OperationStats, error)
	ReorderPlanningTracks(context.Context, []int64) (domain.OperationStats, error)
	RestorePlanningTrackAtEnd(context.Context, int64) (domain.PlanningTrack, bool, domain.OperationStats, error)
}

type Transaction interface {
	RunWrite(context.Context, bool, func(context.Context) error) error
}

type StudentDirectory interface {
	ListEnrolledStudents(context.Context) ([]domain.TargetStudent, domain.OperationStats, error)
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)
