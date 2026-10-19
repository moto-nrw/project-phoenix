package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/feedback"
	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/ports"
)

func NewPublicModule(service *Service, observe ports.Observer) *feedback.Module {
	return feedback.NewModule(engine{service: service, observe: observe})
}

type engine struct {
	service *Service
	observe ports.Observer
}

func (e engine) ObserveRejection(operation string, duration time.Duration, err error) {
	e.observe(ports.Observation{Operation: operation, Duration: duration, Err: err})
}

func (e engine) Available(ctx context.Context) (bool, error) { return e.service.Available(ctx) }

func (e engine) Submit(ctx context.Context, input feedback.CreateEntry) (feedback.Entry, error) {
	entry, err := e.service.Create(ctx, toDomainInput(input))
	if err != nil {
		return feedback.Entry{}, MapPublicError(err, 0)
	}
	return toPublicEntry(entry), nil
}

func (e engine) SubmitBatch(ctx context.Context, inputs []feedback.CreateEntry) ([]feedback.Entry, error) {
	values := make([]domain.Entry, 0, len(inputs))
	for _, input := range inputs {
		values = append(values, toDomainInput(input))
	}
	entries, err := e.service.CreateBatch(ctx, values)
	if err != nil {
		return nil, MapPublicError(err, 0)
	}
	return toPublicEntries(entries), nil
}

func (e engine) LookupEntry(ctx context.Context, id int64) (feedback.Entry, error) {
	entry, err := e.service.Get(ctx, id)
	if err != nil {
		return feedback.Entry{}, MapPublicError(err, id)
	}
	return toPublicEntry(entry), nil
}

func (e engine) EraseEntry(ctx context.Context, id int64) error {
	return MapPublicError(e.service.Delete(ctx, id), id)
}

func (e engine) FindEntries(ctx context.Context, filter feedback.Filter) ([]feedback.Entry, error) {
	entries, err := e.service.List(ctx, toDomainFilter(filter))
	if err != nil {
		return nil, MapPublicError(err, 0)
	}
	return toPublicEntries(entries), nil
}

func (e engine) DeleteExpired(ctx context.Context) (int, error) {
	result, err := e.service.Cleanup(ctx)
	return result.Rows, MapPublicError(err, 0)
}

func (e engine) CountForStudent(ctx context.Context, studentID int64) (int, error) {
	return e.service.CountForStudent(ctx, studentID)
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

func MapPublicError(err error, id int64) error {
	if errors.Is(err, domain.ErrEntryNotFound) {
		return &feedback.EntryNotFoundError{EntryID: id}
	}
	if errors.Is(err, ErrInvalidSettings) {
		return &feedback.InvalidEntryDataError{Err: feedback.ErrInvalidParameters}
	}
	return err
}
