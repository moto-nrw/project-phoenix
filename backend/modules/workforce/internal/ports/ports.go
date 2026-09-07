package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// Store is the persistence port over config.work_time_models,
// config.work_time_model_entries and config.staff_work_schedules. Reads and
// writes honour the tenant in context when one is bound.
type Store interface {
	ListWorkTimeModels(context.Context) ([]domain.WorkTimeModel, domain.OperationStats, error)
	FindWorkTimeModel(context.Context, int64) (domain.WorkTimeModel, bool, domain.OperationStats, error)
	ListWorkTimeModelsByIDs(context.Context, []int64) ([]domain.WorkTimeModel, domain.OperationStats, error)
	CreateWorkTimeModel(context.Context, domain.WorkTimeModelFields) (domain.WorkTimeModel, domain.OperationStats, error)
	// UpdateWorkTimeModel replaces the template metadata and every entry.
	// A missing template reports found=false.
	UpdateWorkTimeModel(context.Context, int64, domain.WorkTimeModelFields) (domain.WorkTimeModel, bool, domain.OperationStats, error)
	DeleteWorkTimeModel(context.Context, int64) (bool, domain.OperationStats, error)

	// CloseStaffSchedules sets an exclusive valid_until on every running
	// version of the given staff members.
	CloseStaffSchedules(ctx context.Context, staffIDs []int64, until string) (domain.OperationStats, error)
	// InsertStaffSchedules writes the given rows as new current versions in
	// one statement.
	InsertStaffSchedules(ctx context.Context, rows []domain.StaffWorkSchedule) (domain.OperationStats, error)

	CurrentStaffSchedule(context.Context, int64) ([]domain.StaffWorkSchedule, domain.OperationStats, error)
	StaffScheduleOn(ctx context.Context, staffID int64, date string) ([]domain.StaffWorkSchedule, domain.OperationStats, error)
	StaffSchedulesInRange(ctx context.Context, staffIDs []int64, from, to string) ([]domain.StaffWorkSchedule, domain.OperationStats, error)
	HasStaffScheduleHistory(context.Context, int64) (bool, domain.OperationStats, error)
	StaffIDsWithScheduleHistory(context.Context, []int64) (map[int64]bool, domain.OperationStats, error)
}

// StaffAssignments is the consumer-owned port over the School Membership rows
// that bind staff to a template. Workforce never joins users.staff itself.
type StaffAssignments interface {
	// AssignedStaffIDs returns the live staff members assigned to a template.
	AssignedStaffIDs(ctx context.Context, workTimeModelID int64) ([]int64, error)
	// RebaseAnchor stamps the template's rotation anchor onto every live
	// assignee and returns their IDs.
	RebaseAnchor(ctx context.Context, workTimeModelID int64, anchorDate string) ([]int64, error)
}

// Transaction runs a unit of work on the caller's ambient transaction or, when
// there is none, opens one.
type Transaction interface {
	RunWrite(context.Context, func(context.Context) error) error
	// LockStaffBalance serializes writes that change a staff member's Soll.
	LockStaffBalance(ctx context.Context, staffID int64) error
}

// Clock supplies the calendar day new schedule versions start on.
type Clock interface {
	Today() string
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)
