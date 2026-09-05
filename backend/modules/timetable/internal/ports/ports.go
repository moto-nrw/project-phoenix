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
}

type Transaction interface {
	RunWrite(context.Context, bool, func(context.Context) error) error
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)
