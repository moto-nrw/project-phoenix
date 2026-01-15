package feedback

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/feedback"
)

// Service defines the feedback service operations
type Service interface {
	base.TransactionalService
	// Core operations
	CreateEntry(ctx context.Context, entry *feedback.Entry) error
	GetEntryByID(ctx context.Context, id int64) (*feedback.Entry, error)
	UpdateEntry(ctx context.Context, entry *feedback.Entry) error
	DeleteEntry(ctx context.Context, id int64) error
	ListEntries(ctx context.Context, filters map[string]interface{}) ([]*feedback.Entry, error)

	// Specialized query methods
	GetEntriesByStudent(ctx context.Context, studentID int64) ([]*feedback.Entry, error)
	GetEntriesByDay(ctx context.Context, day time.Time) ([]*feedback.Entry, error)
	GetEntriesByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*feedback.Entry, error)
	GetMensaFeedback(ctx context.Context, isMensaFeedback bool) ([]*feedback.Entry, error)
	GetEntriesByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate time.Time) ([]*feedback.Entry, error)

	// Statistics and reporting
	CountByDay(ctx context.Context, day time.Time) (int, error)
	CountByStudent(ctx context.Context, studentID int64) (int, error)
	CountMensaFeedback(ctx context.Context, isMensaFeedback bool) (int, error)

	// Batch operations
	CreateEntries(ctx context.Context, entries []*feedback.Entry) ([]error, error)

	// Transaction support is provided by base.TransactionalService
}
