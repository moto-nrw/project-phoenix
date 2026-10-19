package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type RequestQueueRecords interface {
	FindCareScheduleRequest(context.Context, int64, bool) (careplan.CareScheduleChangeRequest, bool, domain.OperationStats, error)
	ListStudentDataRequests(context.Context, careplan.StudentDataRequestFilter) ([]careplan.StudentDataChangeRequest, domain.OperationStats, error)
	ListCareScheduleRequests(context.Context, careplan.CareScheduleRequestFilter) ([]careplan.CareScheduleChangeRequest, domain.OperationStats, error)
}

func NewRequestQueueQueries(store RequestQueueRecords, observe func(ports.Observation)) *requestQueueQueries {
	return &requestQueueQueries{store: store, observe: observe}
}

type requestQueueQueries struct {
	store   RequestQueueRecords
	observe func(ports.Observation)
}

func (q requestQueueQueries) listCareScheduleRequests(ctx context.Context, filter careplan.CareScheduleRequestFilter) ([]careplan.CareScheduleChangeRequest, error) {
	started := time.Now()
	rows, stats, err := q.store.ListCareScheduleRequests(ctx, filter)
	q.observe(ports.Observation{Operation: "list_care_schedule_requests", Duration: time.Since(started), Stats: stats, Err: err})
	return rows, err
}

func (q requestQueueQueries) ListCareScheduleRequests(ctx context.Context, filter careplan.CareScheduleRequestFilter) (rows []careplan.CareScheduleChangeRequest, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		rows, err = q.listCareScheduleRequests(txCtx, filter)
		return err
	})
	return
}

func (q requestQueueQueries) listStudentDataRequests(ctx context.Context, filter careplan.StudentDataRequestFilter) ([]careplan.StudentDataChangeRequest, error) {
	started := time.Now()
	rows, stats, err := q.store.ListStudentDataRequests(ctx, filter)
	q.observe(ports.Observation{Operation: "list_student_data_requests", Duration: time.Since(started), Stats: stats, Err: err})
	return rows, err
}

func (q requestQueueQueries) ListStudentDataRequests(ctx context.Context, filter careplan.StudentDataRequestFilter) (rows []careplan.StudentDataChangeRequest, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		rows, err = q.listStudentDataRequests(txCtx, filter)
		return err
	})
	return
}

func (q requestQueueQueries) findCareScheduleRequest(ctx context.Context, id int64, lock bool) (careplan.CareScheduleChangeRequest, error) {
	started := time.Now()
	row, found, stats, err := q.store.FindCareScheduleRequest(ctx, id, lock)
	if err == nil && !found {
		err = careplan.ErrCareScheduleRequestNotFound
	}
	q.observe(ports.Observation{Operation: "find_care_schedule_request", Duration: time.Since(started), Stats: stats, Err: err})
	return row, err
}

func (q requestQueueQueries) FindCareScheduleRequest(ctx context.Context, id int64, lock bool) (row careplan.CareScheduleChangeRequest, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		row, err = q.findCareScheduleRequest(txCtx, id, lock)
		return err
	})
	return
}
