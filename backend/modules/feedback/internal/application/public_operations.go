package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/feedback"
	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (s *Service) Submit(ctx context.Context, input feedback.CreateEntry) (feedback.Entry, error) {
	started := time.Now()
	normalized, err := validateEntry(input)
	if err != nil {
		s.ObserveRejection("submit", time.Since(started), err)
		return feedback.Entry{}, err
	}
	value := toDomainInput(normalized)
	var entry domain.Entry

	err = s.runWhenEnabled(ctx, "submit", func(txCtx context.Context) (domain.OperationStats, error) {
		var stats domain.OperationStats
		entry, stats, err = s.store.Create(txCtx, value)
		return stats, err
	})
	if err != nil {
		return feedback.Entry{}, err
	}
	return toPublicEntry(entry), nil
}

func (s *Service) SubmitBatch(ctx context.Context, inputs []feedback.CreateEntry) ([]feedback.Entry, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	normalized := make([]feedback.CreateEntry, 0, len(inputs))
	var failures []error
	for _, input := range inputs {
		entry, err := validateEntry(input)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		normalized = append(normalized, entry)
	}
	if len(failures) > 0 {
		err := &feedback.BatchOperationError{Errors: failures}
		s.ObserveRejection("submit_batch", 0, err)
		return nil, err
	}
	values := make([]domain.Entry, 0, len(normalized))
	for _, input := range normalized {
		values = append(values, toDomainInput(input))
	}
	var entries []domain.Entry

	err := s.runWhenEnabled(ctx, "submit_batch", func(txCtx context.Context) (stats domain.OperationStats, batchErr error) {
		entries = make([]domain.Entry, 0, len(values))
		for _, input := range values {
			entry, current, createErr := s.store.Create(txCtx, input)
			stats.Queries += current.Queries
			stats.Rows += current.Rows
			stats.StatementDuration += current.StatementDuration
			if createErr != nil {
				return stats, createErr
			}
			entries = append(entries, entry)
		}
		return stats, nil
	})
	if err != nil {
		return nil, err
	}
	return toPublicEntries(entries), nil
}

func (s *Service) LookupEntry(ctx context.Context, id int64) (feedback.Entry, error) {
	if id <= 0 {
		err := &feedback.InvalidEntryDataError{Err: feedback.ErrInvalidParameters}
		s.ObserveRejection("read", 0, err)
		return feedback.Entry{}, err
	}
	var entry domain.Entry

	var err error
	err = s.runWhenEnabled(ctx, "read", func(txCtx context.Context) (domain.OperationStats, error) {
		var stats domain.OperationStats
		entry, stats, err = s.store.Get(txCtx, id)
		return stats, err
	})
	if err != nil {
		return feedback.Entry{}, err
	}
	return toPublicEntry(entry), nil
}

func (s *Service) EraseEntry(ctx context.Context, id int64) error {
	if id <= 0 {
		err := &feedback.InvalidEntryDataError{Err: feedback.ErrInvalidParameters}
		s.ObserveRejection("delete", 0, err)
		return err
	}

	return s.run(ctx, "delete", func(txCtx context.Context) (domain.OperationStats, error) {
		return s.store.Delete(txCtx, id)
	})
}

func (s *Service) FindEntries(ctx context.Context, filter feedback.Filter) ([]feedback.Entry, error) {
	started := time.Now()
	normalized, err := validateFilter(filter)
	if err != nil {
		s.ObserveRejection("read_list", time.Since(started), err)
		return nil, err
	}
	value := toDomainFilter(normalized)
	var entries []domain.Entry

	err = s.runWhenEnabled(ctx, "read_list", func(txCtx context.Context) (domain.OperationStats, error) {
		var stats domain.OperationStats
		entries, stats, err = s.store.List(txCtx, value)
		return stats, err
	})
	if err != nil {
		return nil, err
	}
	return toPublicEntries(entries), nil
}

// CountForStudent supports deletion previews, including while feedback is off.
func (s *Service) CountForStudent(ctx context.Context, studentID int64) (int, error) {
	if studentID <= 0 {
		err := &feedback.InvalidEntryDataError{Err: feedback.ErrInvalidParameters}
		s.ObserveRejection("count_student", 0, err)
		return 0, err
	}
	var count int

	var err error
	err = s.run(ctx, "count_student", func(txCtx context.Context) (domain.OperationStats, error) {
		var stats domain.OperationStats
		count, stats, err = s.store.CountForStudent(txCtx, studentID)
		return stats, err
	})
	return count, err
}

func (s *Service) DeleteExpired(ctx context.Context) (int, error) {
	var rows int
	err := s.run(ctx, "retention_cleanup", func(txCtx context.Context) (domain.OperationStats, error) {
		retentionDays, err := s.settings.FeedbackRetentionDays(txCtx)
		if err != nil {
			return domain.OperationStats{}, fmt.Errorf("feedback: resolve retention: %w", err)
		}
		if retentionDays <= 0 {
			return domain.OperationStats{}, &feedback.InvalidEntryDataError{Err: feedback.ErrInvalidParameters}
		}
		stats, err := s.store.DeleteOlderThan(txCtx, s.today().AddDays(-retentionDays))
		rows = int(stats.Rows)
		return stats, err
	})
	return rows, err
}
func (s *Service) ObserveRejection(operation string, duration time.Duration, err error) {
	s.observe(ports.Observation{Operation: operation, Duration: duration, Err: err})
}
func validateEntry(input feedback.CreateEntry) (feedback.CreateEntry, error) {
	input.Value = strings.TrimSpace(input.Value)
	switch input.Value {
	case feedback.ValuePositive, feedback.ValueNeutral, feedback.ValueNegative:
	default:
		return feedback.CreateEntry{}, &feedback.InvalidEntryDataError{Err: errors.New("value must be 'positive', 'neutral', or 'negative'")}
	}
	if input.StudentID <= 0 {
		return feedback.CreateEntry{}, &feedback.InvalidEntryDataError{Err: errors.New("student ID is required")}
	}
	date, err := feedback.ParseDate(string(input.Day))
	if err != nil {
		return feedback.CreateEntry{}, &feedback.InvalidEntryDataError{Err: errors.New("day is required")}
	}
	if _, err := time.Parse("15:04:05", input.Time); err != nil {
		return feedback.CreateEntry{}, &feedback.InvalidEntryDataError{Err: errors.New("time is required")}
	}
	input.Day = date
	return input, nil
}

func validateFilter(filter feedback.Filter) (feedback.Filter, error) {
	if filter.StudentID != nil && *filter.StudentID <= 0 {
		return feedback.Filter{}, &feedback.InvalidEntryDataError{Err: feedback.ErrInvalidParameters}
	}
	for _, value := range []*feedback.Date{filter.Day, filter.DayFrom, filter.DayTo} {
		if value == nil {
			continue
		}
		parsed, err := feedback.ParseDate(string(*value))
		if err != nil {
			return feedback.Filter{}, &feedback.InvalidEntryDataError{Err: feedback.ErrInvalidParameters}
		}
		*value = parsed
	}
	if filter.DayFrom != nil && filter.DayTo != nil {
		start, _ := calendar.ParseDate(string(*filter.DayFrom))
		end, _ := calendar.ParseDate(string(*filter.DayTo))
		if start.After(end) {
			return feedback.Filter{}, &feedback.InvalidDateRangeError{StartDate: *filter.DayFrom, EndDate: *filter.DayTo}
		}
	}
	return filter, nil
}

func toDomainInput(input feedback.CreateEntry) domain.Entry {
	day, _ := domain.ParseDate(string(input.Day))
	parsedTime, _ := time.Parse("15:04:05", input.Time)
	return domain.Entry{
		Value: input.Value, Day: day, Time: parsedTime,
		StudentID: input.StudentID, IsMensaFeedback: input.IsMensaFeedback,
	}
}

func toDomainFilter(filter feedback.Filter) domain.Filter {
	return domain.Filter{
		StudentID: filter.StudentID, Day: parseOptionalDate(filter.Day), IsMensaFeedback: filter.IsMensaFeedback,
		DayFrom: parseOptionalDate(filter.DayFrom), DayTo: parseOptionalDate(filter.DayTo), ValueLike: filter.ValueLike,
	}
}

func parseOptionalDate(value *feedback.Date) *domain.Date {
	if value == nil {
		return nil
	}
	parsed, _ := domain.ParseDate(string(*value))
	return &parsed
}

func toPublicEntries(entries []domain.Entry) []feedback.Entry {
	result := make([]feedback.Entry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, toPublicEntry(entry))
	}
	return result
}

func toPublicEntry(entry domain.Entry) feedback.Entry {
	return feedback.Entry{
		ID: entry.ID, Value: entry.Value, Day: feedback.Date(entry.Day.String()), Time: entry.Time.Format("15:04:05"),
		StudentID: entry.StudentID, IsMensaFeedback: entry.IsMensaFeedback, CreatedAt: entry.CreatedAt, UpdatedAt: entry.UpdatedAt,
	}
}
