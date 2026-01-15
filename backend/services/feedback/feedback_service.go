package feedback

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/feedback"
	"github.com/uptrace/bun"
)

// feedbackService implements the Service interface
type feedbackService struct {
	db        *bun.DB
	entryRepo feedback.EntryRepository
	txHandler *base.TxHandler
}

// NewService creates a new feedback service
func NewService(entryRepo feedback.EntryRepository, db *bun.DB) Service {
	return &feedbackService{
		entryRepo: entryRepo,
		db:        db,
		txHandler: base.NewTxHandler(db),
	}
}

// WithTx returns a new service that uses the provided transaction
func (s *feedbackService) WithTx(tx bun.Tx) any {
	// Get repositories with transaction if they implement the TransactionalRepository interface
	var entryRepo = s.entryRepo

	// Try to cast repository to TransactionalRepository and apply the transaction
	if txRepo, ok := s.entryRepo.(base.TransactionalRepository); ok {
		entryRepo = txRepo.WithTx(tx).(feedback.EntryRepository)
	}

	// Return a new service with the transaction
	return &feedbackService{
		db:        s.db,
		entryRepo: entryRepo,
		txHandler: s.txHandler.WithTx(tx),
	}
}

// No longer need the getTx method as we're using txHandler

// CreateEntry creates a new feedback entry
func (s *feedbackService) CreateEntry(ctx context.Context, entry *feedback.Entry) error {
	if entry == nil {
		return &InvalidEntryDataError{Err: ErrInvalidParameters}
	}

	// Validate entry
	if err := entry.Validate(); err != nil {
		return &InvalidEntryDataError{Err: err}
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

// UpdateEntry updates an existing feedback entry
func (s *feedbackService) UpdateEntry(ctx context.Context, entry *feedback.Entry) error {
	if entry == nil || entry.ID <= 0 {
		return &InvalidEntryDataError{Err: ErrInvalidParameters}
	}

	// Validate entry
	if err := entry.Validate(); err != nil {
		return &InvalidEntryDataError{Err: err}
	}

	// Check if entry exists
	existing, err := s.entryRepo.FindByID(ctx, entry.ID)
	if err != nil {
		return err
	}

	if existing == nil {
		return &EntryNotFoundError{EntryID: entry.ID}
	}

	// Update entry
	if err := s.entryRepo.Update(ctx, entry); err != nil {
		return err
	}

	return nil
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
func (s *feedbackService) GetEntriesByDay(ctx context.Context, day time.Time) ([]*feedback.Entry, error) {
	if day.IsZero() {
		return nil, &InvalidEntryDataError{Err: ErrInvalidParameters}
	}

	return s.entryRepo.FindByDay(ctx, day)
}

// GetEntriesByDateRange retrieves all feedback entries within a date range
func (s *feedbackService) GetEntriesByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*feedback.Entry, error) {
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
func (s *feedbackService) GetEntriesByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate time.Time) ([]*feedback.Entry, error) {
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

// CountByDay counts feedback entries for a specific day
func (s *feedbackService) CountByDay(ctx context.Context, day time.Time) (int, error) {
	if day.IsZero() {
		return 0, &InvalidEntryDataError{Err: ErrInvalidParameters}
	}

	return s.entryRepo.CountByDay(ctx, day)
}

// CountByStudent counts feedback entries for a specific student
func (s *feedbackService) CountByStudent(ctx context.Context, studentID int64) (int, error) {
	if studentID <= 0 {
		return 0, &InvalidEntryDataError{Err: ErrInvalidParameters}
	}

	return s.entryRepo.CountByStudentID(ctx, studentID)
}

// CountMensaFeedback counts feedback entries related to the cafeteria
func (s *feedbackService) CountMensaFeedback(ctx context.Context, isMensaFeedback bool) (int, error) {
	return s.entryRepo.CountMensaFeedback(ctx, isMensaFeedback)
}

// CreateEntries creates multiple feedback entries in a batch operation
func (s *feedbackService) CreateEntries(ctx context.Context, entries []*feedback.Entry) ([]error, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	var errors []error

	// Execute in transaction using txHandler
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(Service)

		// Process all entries and collect errors
		for _, entry := range entries {
			if err := txService.CreateEntry(ctx, entry); err != nil {
				errors = append(errors, err)
			}
		}

		// If any errors occurred, rollback the transaction
		if len(errors) > 0 {
			return &BatchOperationError{Errors: errors}
		}

		return nil
	})

	if err != nil {
		// Include the transaction error in the list of errors
		if len(errors) == 0 {
			errors = []error{err}
		} else {
			errors = append(errors, err)
		}
		return errors, &BatchOperationError{Errors: errors}
	}

	if len(errors) > 0 {
		return errors, &BatchOperationError{Errors: errors}
	}

	return nil, nil
}
