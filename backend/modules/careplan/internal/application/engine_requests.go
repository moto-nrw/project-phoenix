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

func (e Engine) observeRequest(operation string, started time.Time, stats domain.OperationStats, err error) {
	e.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
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

func requestCommand(e Engine, ctx context.Context, operation string, call func(context.Context) (domain.OperationStats, error)) (err error) {
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

func requestCommandValue[T any](e Engine, ctx context.Context, operation string, call func(context.Context) (T, domain.OperationStats, error)) (result T, err error) {
	started := time.Now()
	var stats domain.OperationStats
	err = e.withinTenant(ctx, func(txCtx context.Context) error {
		result, stats, err = call(txCtx)
		return err
	})
	e.observeRequest(operation, started, stats, err)
	return result, err
}

func (e Engine) FindExcusedAbsenceRequest(ctx context.Context, id int64, lock bool) (careplan.ExcusedAbsenceRequest, error) {
	value, err := requestValue(ctx, e, "find_excused_absence_request", func(ctx context.Context) (careplan.ExcusedAbsenceRequest, domain.OperationStats, error) {
		value, found, stats, findErr := e.requests.FindExcusedAbsenceRequest(ctx, id, lock)
		if findErr == nil && !found {
			findErr = careplan.ErrExcusedRequestNotFound
		}
		return value, stats, findErr
	})
	return value, err
}
func (e Engine) FindPendingExcusedAbsenceRequest(ctx context.Context, id int64) (careplan.ExcusedAbsenceRequest, error) {
	return requestValue(ctx, e, "find_pending_excused_absence_request", func(ctx context.Context) (careplan.ExcusedAbsenceRequest, domain.OperationStats, error) {
		return e.requests.FindPendingExcusedAbsenceRequest(ctx, id)
	})
}
func (e Engine) ListExcusedAbsenceRequests(ctx context.Context, filter careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, error) {
	return requestValue(ctx, e, "list_excused_absence_requests", func(ctx context.Context) ([]careplan.ExcusedAbsenceRequest, domain.OperationStats, error) {
		return e.requests.ListExcusedAbsenceRequests(ctx, filter)
	})
}
func (e Engine) CreateExcusedAbsenceRequest(ctx context.Context, value careplan.ExcusedAbsenceRequest) (careplan.ExcusedAbsenceRequest, error) {
	return requestCommandValue(e, ctx, "create_excused_absence_request", func(txCtx context.Context) (careplan.ExcusedAbsenceRequest, domain.OperationStats, error) {
		return e.requests.CreateExcusedAbsenceRequest(txCtx, value)
	})
}
func (e Engine) LockExcusedAbsenceRequests(ctx context.Context, studentID int64) error {
	return requestCommand(e, ctx, "lock_excused_absence_requests", func(txCtx context.Context) (domain.OperationStats, error) {
		return e.requests.LockExcusedAbsenceRequests(txCtx, studentID)
	})
}
func (e Engine) UpdatePendingExcusedAbsenceRequest(ctx context.Context, id int64, dates []careplan.Date, note, status string) error {
	return requestCommand(e, ctx, "update_pending_excused_absence_request", func(txCtx context.Context) (domain.OperationStats, error) {
		return e.requests.UpdatePendingExcusedAbsenceRequest(txCtx, id, dates, note, status)
	})
}
func (e Engine) DecideExcusedAbsenceRequest(ctx context.Context, value careplan.ExcusedAbsenceDecision) error {
	return requestCommand(e, ctx, "decide_excused_absence_request", func(txCtx context.Context) (domain.OperationStats, error) {
		return e.requests.DecideExcusedAbsenceRequest(txCtx, value, false)
	})
}
func (e Engine) RedecideExcusedAbsenceRequest(ctx context.Context, value careplan.ExcusedAbsenceDecision) error {
	return requestCommand(e, ctx, "redecide_excused_absence_request", func(txCtx context.Context) (domain.OperationStats, error) {
		return e.requests.DecideExcusedAbsenceRequest(txCtx, value, true)
	})
}

func (e Engine) FindCareScheduleRequest(ctx context.Context, id int64, lock bool) (careplan.CareScheduleChangeRequest, error) {
	return requestValue(ctx, e, "find_care_schedule_request", func(ctx context.Context) (careplan.CareScheduleChangeRequest, domain.OperationStats, error) {
		value, found, stats, findErr := e.requests.FindCareScheduleRequest(ctx, id, lock)
		if findErr == nil && !found {
			findErr = careplan.ErrCareScheduleRequestNotFound
		}
		return value, stats, findErr
	})
}
func (e Engine) FindPendingCareScheduleRequest(ctx context.Context, id int64) (careplan.CareScheduleChangeRequest, error) {
	return requestValue(ctx, e, "find_pending_care_schedule_request", func(ctx context.Context) (careplan.CareScheduleChangeRequest, domain.OperationStats, error) {
		return e.requests.FindPendingCareScheduleRequest(ctx, id)
	})
}
func (e Engine) ListCareScheduleRequests(ctx context.Context, filter careplan.CareScheduleRequestFilter) ([]careplan.CareScheduleChangeRequest, error) {
	return requestValue(ctx, e, "list_care_schedule_requests", func(ctx context.Context) ([]careplan.CareScheduleChangeRequest, domain.OperationStats, error) {
		return e.requests.ListCareScheduleRequests(ctx, filter)
	})
}
func (e Engine) CreateCareScheduleRequest(ctx context.Context, value careplan.CareScheduleChangeRequest) (careplan.CareScheduleChangeRequest, error) {
	return requestCommandValue(e, ctx, "create_care_schedule_request", func(txCtx context.Context) (careplan.CareScheduleChangeRequest, domain.OperationStats, error) {
		return e.requests.CreateCareScheduleRequest(txCtx, value)
	})
}
func (e Engine) UpdatePendingCareScheduleRequest(ctx context.Context, id int64, payload json.RawMessage) error {
	return requestCommand(e, ctx, "update_pending_care_schedule_request", func(txCtx context.Context) (domain.OperationStats, error) {
		return e.requests.UpdatePendingCareScheduleRequest(txCtx, id, payload)
	})
}
func (e Engine) DecideCareScheduleRequest(ctx context.Context, value careplan.CareScheduleRequestDecision) error {
	return requestCommand(e, ctx, "decide_care_schedule_request", func(txCtx context.Context) (domain.OperationStats, error) {
		return e.requests.DecideCareScheduleRequest(txCtx, value, false)
	})
}
func (e Engine) RedecideCareScheduleRequest(ctx context.Context, value careplan.CareScheduleRequestDecision) error {
	return requestCommand(e, ctx, "redecide_care_schedule_request", func(txCtx context.Context) (domain.OperationStats, error) {
		return e.requests.DecideCareScheduleRequest(txCtx, value, true)
	})
}
func (e Engine) UpdateCareScheduleRequestSnapshot(ctx context.Context, id int64, snapshot json.RawMessage) error {
	return requestCommand(e, ctx, "update_care_schedule_request_snapshot", func(txCtx context.Context) (domain.OperationStats, error) {
		return e.requests.UpdateCareScheduleRequestSnapshot(txCtx, id, snapshot)
	})
}

func (e Engine) FindStudentDataRequest(ctx context.Context, id int64, lock bool) (careplan.StudentDataChangeRequest, error) {
	return requestValue(ctx, e, "find_student_data_request", func(ctx context.Context) (careplan.StudentDataChangeRequest, domain.OperationStats, error) {
		value, found, stats, findErr := e.requests.FindStudentDataRequest(ctx, id, lock)
		if findErr == nil && !found {
			findErr = careplan.ErrStudentDataRequestNotFound
		}
		return value, stats, findErr
	})
}
func (e Engine) FindPendingStudentDataRequest(ctx context.Context, id int64) (careplan.StudentDataChangeRequest, error) {
	return requestValue(ctx, e, "find_pending_student_data_request", func(ctx context.Context) (careplan.StudentDataChangeRequest, domain.OperationStats, error) {
		return e.requests.FindPendingStudentDataRequest(ctx, id)
	})
}
func (e Engine) ListStudentDataRequests(ctx context.Context, filter careplan.StudentDataRequestFilter) ([]careplan.StudentDataChangeRequest, error) {
	return requestValue(ctx, e, "list_student_data_requests", func(ctx context.Context) ([]careplan.StudentDataChangeRequest, domain.OperationStats, error) {
		return e.requests.ListStudentDataRequests(ctx, filter)
	})
}
func (e Engine) HasPendingStudentDataRequest(ctx context.Context, studentID int64, target, field string) (bool, error) {
	return requestValue(ctx, e, "has_pending_student_data_request", func(ctx context.Context) (bool, domain.OperationStats, error) {
		return e.requests.HasPendingStudentDataRequest(ctx, studentID, target, field)
	})
}
func (e Engine) CreateStudentDataRequest(ctx context.Context, value careplan.StudentDataChangeRequest) (careplan.StudentDataChangeRequest, error) {
	return requestCommandValue(e, ctx, "create_student_data_request", func(txCtx context.Context) (careplan.StudentDataChangeRequest, domain.OperationStats, error) {
		return e.requests.CreateStudentDataRequest(txCtx, value)
	})
}
func (e Engine) UpdatePendingStudentDataRequest(ctx context.Context, id int64, value json.RawMessage) error {
	return requestCommand(e, ctx, "update_pending_student_data_request", func(txCtx context.Context) (domain.OperationStats, error) {
		return e.requests.UpdatePendingStudentDataRequest(txCtx, id, value)
	})
}
func (e Engine) DecideStudentDataRequest(ctx context.Context, value careplan.StudentDataRequestDecision) error {
	return requestCommand(e, ctx, "decide_student_data_request", func(txCtx context.Context) (domain.OperationStats, error) {
		return e.requests.DecideStudentDataRequest(txCtx, value, false)
	})
}
func (e Engine) RedecideStudentDataRequest(ctx context.Context, value careplan.StudentDataRequestDecision) error {
	return requestCommand(e, ctx, "redecide_student_data_request", func(txCtx context.Context) (domain.OperationStats, error) {
		return e.requests.DecideStudentDataRequest(txCtx, value, true)
	})
}

func (e Engine) CountOpenCareRequests(ctx context.Context, studentIDs []int64) (map[int64]int, error) {
	return requestValue(ctx, e, "count_open_care_requests", func(ctx context.Context) (map[int64]int, domain.OperationStats, error) {
		return e.requests.CountOpenCareRequests(ctx, studentIDs)
	})
}

func (e Engine) LockOpenCareRequests(ctx context.Context, studentIDs []int64) error {
	return requestCommand(e, ctx, "lock_open_care_requests", func(txCtx context.Context) (domain.OperationStats, error) {
		return e.requests.LockOpenCareRequests(txCtx, studentIDs)
	})
}

func (e Engine) CloseOpenCareRequests(ctx context.Context, studentIDs []int64, reason string, reviewedBy *int64, at time.Time) (int64, error) {
	return requestCommandValue(e, ctx, "close_open_care_requests", func(txCtx context.Context) (int64, domain.OperationStats, error) {
		return e.requests.CloseOpenCareRequests(txCtx, studentIDs, reason, reviewedBy, at)
	})
}
