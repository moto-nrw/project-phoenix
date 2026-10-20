package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	careplanPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
)

// Runtime is what the service needs from the tenant runtime and the
// database: commands join the caller's tenant transaction or open one, and
// the student and exception-day locks serialize the schedule writes. The
// composition binds it.
type Runtime interface {
	DayLocks
	WithinTenant(ctx context.Context, command func(context.Context) error) error
	LockStudent(ctx context.Context, studentID int64) error
}

// DayLocks are the student and exception-day locks the capability serves as
// they are.
type DayLocks interface {
	LockStudentAndExceptionDay(ctx context.Context, studentID int64, date string) error
	LockExceptionDay(ctx context.Context, studentID int64, date string) error
}

// Dependencies are the stores and seams the service serves the capability
// over.
type Dependencies struct {
	OfferingBookings *careplan.OfferingBookings
	Store            ExceptionStore
	Requests         RequestStore
	StatusDays       StatusDayStore
	Withdrawals      ports.WithdrawalList
	Observe          func(ports.Observation)
	People           careplanPorts.StudentNameFinder
	Runtime          Runtime
}

// Service serves the Care Plan capability (#4453): it validates the public
// input, runs each command in the caller's tenant transaction, observes the
// operation and maps the store errors onto the contract.
type Service struct {
	*careplan.OfferingBookings
	DayLocks
	store         ports.Store
	statusQueries StatusQueries
	observe       ports.Observer
	requests      RequestStore
	statusDays    StatusDayStore
	withdrawals   ports.WithdrawalList
	people        careplanPorts.StudentNameFinder
	runtime       Runtime
}

// NewService serves the complete Care Plan capability over dependencies.
func NewService(dependencies Dependencies) *Service {
	service := NewExceptionQueryService(dependencies.Store, dependencies.Observe)
	service.OfferingBookings, service.DayLocks = dependencies.OfferingBookings, dependencies.Runtime
	service.requests, service.statusDays, service.withdrawals = dependencies.Requests, dependencies.StatusDays, dependencies.Withdrawals
	service.people, service.runtime = dependencies.People, dependencies.Runtime
	return service
}

// New serves the catalog and offering change reads over store.
func New(store ports.Store, observe ports.Observer) *Service {
	if store == nil || observe == nil {
		panic("care plan application: store and observer are required")
	}
	return &Service{store: store, observe: observe}
}

// NewExceptionQueryService serves the schedule, exception, note and status
// day reads over store.
func NewExceptionQueryService(store ExceptionStore, observe func(ports.Observation)) *Service {
	service := New(store, observe)
	service.statusQueries = store
	return service
}

func (s *Service) withinTenant(ctx context.Context, command func(context.Context) error) error {
	return s.runtime.WithinTenant(ctx, command)
}

func (s *Service) run(ctx context.Context, operation string, command func(context.Context, *domain.OperationStats) error) (err error) {
	started := time.Now()
	stats := domain.OperationStats{}
	defer func() {
		s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	}()
	return tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error { return command(txCtx, &stats) })
}
