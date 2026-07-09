package feedback

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/feedback"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// feedbackService implements the Service interface
type feedbackService struct {
	entryRepo feedback.EntryRepository
}

// NewService creates a new feedback service
func NewService(entryRepo feedback.EntryRepository) Service {
	return &feedbackService{
		entryRepo: entryRepo,
	}
}

// CreateEntry creates a new feedback entry
func (s *feedbackService) CreateEntry(ctx context.Context, entry *feedback.Entry) error {
	if entry == nil {
		return &InvalidEntryDataError{Err: ErrInvalidParameters}
	}

	// Validate entry
	if err := entry.Validate(); err != nil {
		return &InvalidEntryDataError{Err: err}
	}

	// Set tenant ID from context
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		entry.SetTenantID(tenantID)
	}

	// Create entry
	if err := s.entryRepo.Create(ctx, entry); err != nil {
		return err
	}

	return nil
}

// GetEntryByID retrieves a feedback entry by ID
func (s *feedbackService) GetEntryByID(ctx context.Context, id int64) (*feedback.Entry, error) {
	if id <= 0 {
		return nil, &InvalidEntryDataError{Err: ErrInvalidParameters}
	}

	entry, err := s.entryRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, &EntryNotFoundError{EntryID: id}
	}

	return entry, nil
}

// DeleteEntry deletes a feedback entry by ID
func (s *feedbackService) DeleteEntry(ctx context.Context, id int64) error {
	if id <= 0 {
		return &InvalidEntryDataError{Err: ErrInvalidParameters}
	}

	// Check if entry exists
	existing, err := s.entryRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if existing == nil {
		return &EntryNotFoundError{EntryID: id}
	}

	// Delete entry
	if err := s.entryRepo.Delete(ctx, id); err != nil {
		return err
	}

	return nil
}

// ListEntries lists all feedback entries based on filters
func (s *feedbackService) ListEntries(ctx context.Context, filters map[string]interface{}) ([]*feedback.Entry, error) {
	return s.entryRepo.List(ctx, filters)
}

// GetEntriesByStudent retrieves all feedback entries for a student
func (s *feedbackService) GetEntriesByStudent(ctx context.Context, studentID int64) ([]*feedback.Entry, error) {
	if studentID <= 0 {
		return nil, &InvalidEntryDataError{Err: ErrInvalidParameters}
	}

	return s.entryRepo.FindByStudentID(ctx, studentID)
}

// GetEntriesByDay retrieves all feedback entries for a specific day
func (s *feedbackService) GetEntriesByDay(ctx context.Context, day timezone.Date) ([]*feedback.Entry, error) {
	if day.IsZero() {
		return nil, &InvalidEntryDataError{Err: ErrInvalidParameters}
	}

	return s.entryRepo.FindByDay(ctx, day)
}

// GetEntriesByDateRange retrieves all feedback entries within a date range
func (s *feedbackService) GetEntriesByDateRange(ctx context.Context, startDate, endDate timezone.Date) ([]*feedback.Entry, error) {
	if startDate.IsZero() || endDate.IsZero() {
		return nil, &InvalidEntryDataError{Err: ErrInvalidParameters}
	}

	if startDate.After(endDate) {
		return nil, &InvalidDateRangeError{
			StartDate: startDate,
			EndDate:   endDate,
		}
	}

	return s.entryRepo.FindByDateRange(ctx, startDate, endDate)
}

// GetMensaFeedback retrieves all feedback entries related to the cafeteria
func (s *feedbackService) GetMensaFeedback(ctx context.Context, isMensaFeedback bool) ([]*feedback.Entry, error) {
	return s.entryRepo.FindMensaFeedback(ctx, isMensaFeedback)
}

// GetEntriesByStudentAndDateRange retrieves all feedback entries for a student within a date range
func (s *feedbackService) GetEntriesByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*feedback.Entry, error) {
	if studentID <= 0 {
		return nil, &InvalidEntryDataError{Err: ErrInvalidParameters}
	}

	if startDate.IsZero() || endDate.IsZero() {
		return nil, &InvalidEntryDataError{Err: ErrInvalidParameters}
	}

	if startDate.After(endDate) {
		return nil, &InvalidDateRangeError{
			StartDate: startDate,
			EndDate:   endDate,
		}
	}

	return s.entryRepo.FindByStudentAndDateRange(ctx, studentID, startDate, endDate)
}

// CreateEntries creates multiple feedback entries in a batch operation
func (s *feedbackService) CreateEntries(ctx context.Context, entries []*feedback.Entry) ([]error, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	var errors []error

	for _, entry := range entries {
		if err := s.CreateEntry(ctx, entry); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return errors, &BatchOperationError{Errors: errors}
	}

	return nil, nil
}

// DeleteEntriesOlderThan deletes feedback entries older than the specified number of days.
func (s *feedbackService) DeleteEntriesOlderThan(ctx context.Context, days int) (int, error) {
	if days <= 0 {
		return 0, ErrInvalidParameters
	}
	return s.entryRepo.DeleteOlderThan(ctx, days)
}
