package feedback

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/feedback"
)

// Service defines the feedback service operations
type Service interface {
	// Core operations
	CreateEntry(ctx context.Context, entry *feedback.Entry) error
	GetEntryByID(ctx context.Context, id int64) (*feedback.Entry, error)
	DeleteEntry(ctx context.Context, id int64) error
	ListEntries(ctx context.Context, filters map[string]interface{}) ([]*feedback.Entry, error)

	// Specialized query methods
	GetEntriesByStudent(ctx context.Context, studentID int64) ([]*feedback.Entry, error)
	GetEntriesByDay(ctx context.Context, day timezone.Date) ([]*feedback.Entry, error)
	GetEntriesByDateRange(ctx context.Context, startDate, endDate timezone.Date) ([]*feedback.Entry, error)
	GetMensaFeedback(ctx context.Context, isMensaFeedback bool) ([]*feedback.Entry, error)
	GetEntriesByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*feedback.Entry, error)

	// Statistics and reporting

	// Batch operations
	CreateEntries(ctx context.Context, entries []*feedback.Entry) ([]error, error)

	// Cleanup operations
	DeleteEntriesOlderThan(ctx context.Context, days int) (int, error)
}
