package application

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

type RequestStore interface {
	FindExcusedAbsenceRequest(context.Context, int64, bool) (careplan.ExcusedAbsenceRequest, bool, domain.OperationStats, error)
	FindPendingExcusedAbsenceRequest(context.Context, int64) (careplan.ExcusedAbsenceRequest, domain.OperationStats, error)
	ListExcusedAbsenceRequests(context.Context, careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, domain.OperationStats, error)
	CreateExcusedAbsenceRequest(context.Context, careplan.ExcusedAbsenceRequest) (careplan.ExcusedAbsenceRequest, domain.OperationStats, error)
	LockExcusedAbsenceRequests(context.Context, int64) (domain.OperationStats, error)
	UpdatePendingExcusedAbsenceRequest(context.Context, int64, []careplan.Date, string, string) (domain.OperationStats, error)
	DecideExcusedAbsenceRequest(context.Context, careplan.ExcusedAbsenceDecision, bool) (domain.OperationStats, error)

	FindCareScheduleRequest(context.Context, int64, bool) (careplan.CareScheduleChangeRequest, bool, domain.OperationStats, error)
	FindPendingCareScheduleRequest(context.Context, int64) (careplan.CareScheduleChangeRequest, domain.OperationStats, error)
	ListCareScheduleRequests(context.Context, careplan.CareScheduleRequestFilter) ([]careplan.CareScheduleChangeRequest, domain.OperationStats, error)
	CreateCareScheduleRequest(context.Context, careplan.CareScheduleChangeRequest) (careplan.CareScheduleChangeRequest, domain.OperationStats, error)
	UpdatePendingCareScheduleRequest(context.Context, int64, json.RawMessage) (domain.OperationStats, error)
	DecideCareScheduleRequest(context.Context, careplan.CareScheduleRequestDecision, bool) (domain.OperationStats, error)
	UpdateCareScheduleRequestSnapshot(context.Context, int64, json.RawMessage) (domain.OperationStats, error)

	FindStudentDataRequest(context.Context, int64, bool) (careplan.StudentDataChangeRequest, bool, domain.OperationStats, error)
	FindPendingStudentDataRequest(context.Context, int64) (careplan.StudentDataChangeRequest, domain.OperationStats, error)
	ListStudentDataRequests(context.Context, careplan.StudentDataRequestFilter) ([]careplan.StudentDataChangeRequest, domain.OperationStats, error)
	HasPendingStudentDataRequest(context.Context, int64, string, string) (bool, domain.OperationStats, error)
	CreateStudentDataRequest(context.Context, careplan.StudentDataChangeRequest) (careplan.StudentDataChangeRequest, domain.OperationStats, error)
	UpdatePendingStudentDataRequest(context.Context, int64, json.RawMessage) (domain.OperationStats, error)
	DecideStudentDataRequest(context.Context, careplan.StudentDataRequestDecision, bool) (domain.OperationStats, error)
	CountOpenCareRequests(context.Context, []int64) (map[int64]int, domain.OperationStats, error)
	LockOpenCareRequests(context.Context, []int64) (domain.OperationStats, error)
	CloseOpenCareRequests(context.Context, []int64, string, *int64, time.Time) (int64, domain.OperationStats, error)
}

func (s *Service) observeRequest(operation string, started time.Time, stats domain.OperationStats, err error) {
	s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
}

func requestValue[T any](ctx context.Context, e interface {
	observeRequest(string, time.Time, domain.OperationStats, error)
}, operation string, call func(ctx context.Context) (T, domain.OperationStats, error)) (result T, err error) {
	started := time.Now()
	var stats domain.OperationStats
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		result, stats, err = call(txCtx)
		return err
	})
	e.observeRequest(operation, started, stats, err)
	return result, err
}

func requestCommand(e *Service, ctx context.Context, operation string, call func(context.Context) (domain.OperationStats, error)) (err error) {
	started := time.Now()
	var stats domain.OperationStats
	err = e.withinTenant(ctx, func(txCtx context.Context) error {
		var commandErr error
		stats, commandErr = call(txCtx)
		return commandErr
	})
	e.observeRequest(operation, started, stats, err)
	return err
}

func requestCommandValue[T any](e *Service, ctx context.Context, operation string, call func(context.Context) (T, domain.OperationStats, error)) (result T, err error) {
	started := time.Now()
	var stats domain.OperationStats
	err = e.withinTenant(ctx, func(txCtx context.Context) error {
		result, stats, err = call(txCtx)
		return err
	})
	e.observeRequest(operation, started, stats, err)
	return result, err
}
