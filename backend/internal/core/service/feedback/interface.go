package feedback

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/feedback"
)

// EntryReader defines read-only feedback queries.
type EntryReader interface {
	GetEntryByID(ctx context.Context, id int64) (*feedback.Entry, error)
	ListEntries(ctx context.Context, filters map[string]interface{}) ([]*feedback.Entry, error)
	GetEntriesByStudent(ctx context.Context, studentID int64) ([]*feedback.Entry, error)
	GetEntriesByDay(ctx context.Context, day time.Time) ([]*feedback.Entry, error)
	GetEntriesByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*feedback.Entry, error)
	GetMensaFeedback(ctx context.Context, isMensaFeedback bool) ([]*feedback.Entry, error)
	GetEntriesByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate time.Time) ([]*feedback.Entry, error)
}

// EntryWriter defines write operations for feedback entries.
type EntryWriter interface {
	CreateEntry(ctx context.Context, entry *feedback.Entry) error
	UpdateEntry(ctx context.Context, entry *feedback.Entry) error
	DeleteEntry(ctx context.Context, id int64) error
	CreateEntries(ctx context.Context, entries []*feedback.Entry) ([]error, error)
}

// EntryStats defines aggregate reporting operations.
type EntryStats interface {
	CountByDay(ctx context.Context, day time.Time) (int, error)
	CountByStudent(ctx context.Context, studentID int64) (int, error)
	CountMensaFeedback(ctx context.Context, isMensaFeedback bool) (int, error)
}

// EntryReadWriter composes read and write operations for handlers.
type EntryReadWriter interface {
	EntryReader
	EntryWriter
}

// Service defines the full feedback service operations.
type Service interface {
	base.TransactionalService
	EntryReader
	EntryWriter
	EntryStats
}
