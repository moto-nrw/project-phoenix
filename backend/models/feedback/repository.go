package feedback

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// EntryRepository defines operations for managing feedback entries
type EntryRepository interface {
	base.CRUDRepository[*Entry]

	// Specialized query methods
	FindByStudentID(ctx context.Context, studentID int64) ([]*Entry, error)
	FindByDay(ctx context.Context, day timezone.Date) ([]*Entry, error)
	FindByDateRange(ctx context.Context, startDate, endDate timezone.Date) ([]*Entry, error)
	FindMensaFeedback(ctx context.Context, isMensaFeedback bool) ([]*Entry, error)
	FindByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*Entry, error)

	// Cleanup methods
	DeleteOlderThan(ctx context.Context, days int) (int, error)
}
