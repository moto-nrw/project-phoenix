package compose

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/feedback"
	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type TransactionFunc func(context.Context, func(context.Context) error) error

func (fn TransactionFunc) Run(ctx context.Context, callback func(context.Context) error) error {
	return fn(ctx, callback)
}

type Observation = ports.Observation

type Dependencies struct {
	DB       *bun.DB
	Settings interface {
		FeedbackEnabled(context.Context) (bool, error)
		FeedbackRetentionDays(context.Context) (int, error)
	}
	Today   func() feedback.Date
	Observe func(Observation)
}

func New(dependencies Dependencies) (*feedback.Module, error) {
	if dependencies.DB == nil || dependencies.Settings == nil || dependencies.Today == nil || dependencies.Observe == nil {
		return nil, errors.New("feedback compose: all dependencies are required")
	}
	store := postgres.New(func(ctx context.Context) (bun.IDB, int64, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, 0, errors.New("feedback postgres: transaction is required")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, 0, fmt.Errorf("feedback postgres: unsupported transaction %T", transaction)
		}
		tenantID, err := tenant.TenantFromContext(ctx)
		if err != nil {
			return nil, 0, err
		}
		return tx, tenantID.Int64(), nil
	})
	today := func() domain.Date {
		date, err := domain.ParseDate(string(dependencies.Today()))
		if err != nil {
			panic(fmt.Sprintf("feedback compose: invalid current date: %v", err))
		}
		return date
	}
	observe := func(observation ports.Observation) {
		observation.Err = mapError(observation.Err, 0)
		dependencies.Observe(observation)
	}
	service := application.New(store, dependencies.Settings, TransactionFunc(tenant.NewTransactionRunner().RunInTx), observe, today)
	return feedback.NewModule(engine{service: service, observe: dependencies.Observe}), nil
}

// Settings are the feedback settings resolvers, bound at construction.
type Settings struct {
	enabled   func(context.Context) (bool, error)
	retention func(context.Context) (int, error)
}

// NewBoundSettings binds the resolvers; a missing one is a construction
// error.
func NewBoundSettings(enabled func(context.Context) (bool, error), retention func(context.Context) (int, error)) (*Settings, error) {
	if enabled == nil || retention == nil {
		return nil, errors.New("feedback settings: both resolvers are required")
	}
	return &Settings{enabled: enabled, retention: retention}, nil
}

func (s *Settings) FeedbackEnabled(ctx context.Context) (bool, error) { return s.enabled(ctx) }

func (s *Settings) FeedbackRetentionDays(ctx context.Context) (int, error) { return s.retention(ctx) }

type engine struct {
	service *application.Service
	observe ports.Observer
}

func (e engine) ObserveRejection(operation string, duration time.Duration, err error) {
	e.observe(ports.Observation{Operation: operation, Duration: duration, Err: err})
}

func (e engine) Available(ctx context.Context) (bool, error) { return e.service.Available(ctx) }

func (e engine) Submit(ctx context.Context, input feedback.CreateEntry) (feedback.Entry, error) {
	entry, err := e.service.Create(ctx, toDomainInput(input))
	if err != nil {
		return feedback.Entry{}, mapError(err, 0)
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
		return nil, mapError(err, 0)
	}
	return toPublicEntries(entries), nil
}

func (e engine) LookupEntry(ctx context.Context, id int64) (feedback.Entry, error) {
	entry, err := e.service.Get(ctx, id)
	if err != nil {
		return feedback.Entry{}, mapError(err, id)
	}
	return toPublicEntry(entry), nil
}

func (e engine) EraseEntry(ctx context.Context, id int64) error {
	return mapError(e.service.Delete(ctx, id), id)
}

func (e engine) FindEntries(ctx context.Context, filter feedback.Filter) ([]feedback.Entry, error) {
	entries, err := e.service.List(ctx, toDomainFilter(filter))
	if err != nil {
		return nil, mapError(err, 0)
	}
	return toPublicEntries(entries), nil
}

func (e engine) DeleteExpired(ctx context.Context) (int, error) {
	result, err := e.service.Cleanup(ctx)
	return result.Rows, mapError(err, 0)
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

func mapError(err error, id int64) error {
	if errors.Is(err, domain.ErrEntryNotFound) {
		return &feedback.EntryNotFoundError{EntryID: id}
	}
	if errors.Is(err, application.ErrInvalidSettings) {
		return &feedback.InvalidEntryDataError{Err: feedback.ErrInvalidParameters}
	}
	return err
}
