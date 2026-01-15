package feedback

import (
	"context"
	"time"

	domain "github.com/moto-nrw/project-phoenix/internal/core/domain/feedback"
)

type Entry = domain.Entry

// EntryRepository defines operations for managing feedback entries
type EntryRepository interface {
	// Core CRUD operations
	Create(ctx context.Context, entry *Entry) error
	FindByID(ctx context.Context, id interface{}) (*Entry, error)
	Update(ctx context.Context, entry *Entry) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*Entry, error)

	// Specialized query methods
	FindByStudentID(ctx context.Context, studentID int64) ([]*Entry, error)
	FindByDay(ctx context.Context, day time.Time) ([]*Entry, error)
	FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*Entry, error)
	FindMensaFeedback(ctx context.Context, isMensaFeedback bool) ([]*Entry, error)
	FindByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate time.Time) ([]*Entry, error)

	// Aggregation methods
	CountByDay(ctx context.Context, day time.Time) (int, error)
	CountByStudentID(ctx context.Context, studentID int64) (int, error)
	CountMensaFeedback(ctx context.Context, isMensaFeedback bool) (int, error)
}
