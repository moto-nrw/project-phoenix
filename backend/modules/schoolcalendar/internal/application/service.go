package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/ports"
)

// Service joins the caller's transaction or opens a school unit of work for
// record operations. Pure holiday/render operations need no database scope.
// It records one observation per call and preserves the domain errors.
type Service struct {
	store          ports.Store
	observe        ports.Observer
	administration Administration
	algorithms     CalendarAlgorithms
}

// New composes the service. The administrative write path needs a
// transaction runner and a clock; the gate, guard and federal-state ports
// are optional collaborators of the composition.
func New(store ports.Store, observe ports.Observer, administration Administration, algorithms CalendarAlgorithms) *Service {
	if algorithms == nil || store == nil || observe == nil || administration.Transaction == nil || administration.Today == nil {
		panic("school calendar application: all dependencies are required")
	}
	administration.Observe = observe
	return &Service{store: store, observe: observe, administration: administration, algorithms: algorithms}
}

// --- calendar periods ---

// --- closing days ---

// --- dateframes ---

// --- holidays and calendar documents ---

func (s *Service) ValidHolidayRegion(region string) bool {
	return s.algorithms.ValidHolidayRegion(region)
}

// --- plumbing ---

func (s *Service) run(ctx context.Context, operation string, fn func(context.Context, *domain.OperationStats) error) (err error) {
	return s.administration.Transaction(ctx, func(txCtx context.Context) error {
		return s.observeOperation(operation, func(_ context.Context, stats *domain.OperationStats) error { return fn(txCtx, stats) })
	})
}

func (s *Service) observeOperation(operation string, fn func(context.Context, *domain.OperationStats) error) (err error) {
	return observeCalendarOperation(s.observe, operation, fn)
}
func (s *Service) observeAdministration(operation string, fn func(context.Context, *domain.OperationStats) error) error {
	return observeCalendarOperation(s.administration.Observe, operation, fn)
}
func observeCalendarOperation(observe ports.Observer, operation string, fn func(context.Context, *domain.OperationStats) error) (err error) {
	started := time.Now()
	stats := domain.OperationStats{}
	defer func() {
		observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	}()
	err = fn(context.Background(), &stats)
	if err != nil {
		stats.Rows = 0
	}
	return err
}

func WithAdministration(records schoolcalendar.Days, administration Administration, observe ports.Observer) (*Service, error) {
	original, ok := records.(*Service)
	if !ok || original == nil {
		return nil, fmt.Errorf("school calendar compose: records must be composed by NewRecords")
	}
	administration.FederalState = original.administration.FederalState
	administration.Observe = observe
	return &Service{store: original.store, observe: original.observe, administration: administration, algorithms: original.algorithms}, nil
}

func BerlinToday() string { return time.Now().In(calendar.Berlin).Format(schoolcalendar.DateLayout) }
