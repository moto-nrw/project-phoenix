package compose

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

type RequestStoreStats = domain.OperationStats

type RequestStore interface {
	FindExcusedAbsenceRequest(context.Context, int64, bool) (careplan.ExcusedAbsenceRequest, bool, RequestStoreStats, error)
	FindPendingExcusedAbsenceRequest(context.Context, int64) (careplan.ExcusedAbsenceRequest, RequestStoreStats, error)
	ListExcusedAbsenceRequests(context.Context, careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, RequestStoreStats, error)
	CreateExcusedAbsenceRequest(context.Context, careplan.ExcusedAbsenceRequest) (careplan.ExcusedAbsenceRequest, RequestStoreStats, error)
	LockExcusedAbsenceRequests(context.Context, int64) (RequestStoreStats, error)
	UpdatePendingExcusedAbsenceRequest(context.Context, int64, []careplan.Date, string, string) (RequestStoreStats, error)
	DecideExcusedAbsenceRequest(context.Context, careplan.ExcusedAbsenceDecision, bool) (RequestStoreStats, error)

	FindCareScheduleRequest(context.Context, int64, bool) (careplan.CareScheduleChangeRequest, bool, RequestStoreStats, error)
	FindPendingCareScheduleRequest(context.Context, int64) (careplan.CareScheduleChangeRequest, RequestStoreStats, error)
	ListCareScheduleRequests(context.Context, careplan.CareScheduleRequestFilter) ([]careplan.CareScheduleChangeRequest, RequestStoreStats, error)
	CreateCareScheduleRequest(context.Context, careplan.CareScheduleChangeRequest) (careplan.CareScheduleChangeRequest, RequestStoreStats, error)
	UpdatePendingCareScheduleRequest(context.Context, int64, json.RawMessage) (RequestStoreStats, error)
	DecideCareScheduleRequest(context.Context, careplan.CareScheduleRequestDecision, bool) (RequestStoreStats, error)
	UpdateCareScheduleRequestSnapshot(context.Context, int64, json.RawMessage) (RequestStoreStats, error)

	FindStudentDataRequest(context.Context, int64, bool) (careplan.StudentDataChangeRequest, bool, RequestStoreStats, error)
	FindPendingStudentDataRequest(context.Context, int64) (careplan.StudentDataChangeRequest, RequestStoreStats, error)
	ListStudentDataRequests(context.Context, careplan.StudentDataRequestFilter) ([]careplan.StudentDataChangeRequest, RequestStoreStats, error)
	HasPendingStudentDataRequest(context.Context, int64, string, string) (bool, RequestStoreStats, error)
	CreateStudentDataRequest(context.Context, careplan.StudentDataChangeRequest) (careplan.StudentDataChangeRequest, RequestStoreStats, error)
	UpdatePendingStudentDataRequest(context.Context, int64, json.RawMessage) (RequestStoreStats, error)
	DecideStudentDataRequest(context.Context, careplan.StudentDataRequestDecision, bool) (RequestStoreStats, error)
	CountOpenCareRequests(context.Context, []int64) (map[int64]int, RequestStoreStats, error)
	LockOpenCareRequests(context.Context, []int64) (RequestStoreStats, error)
	CloseOpenCareRequests(context.Context, []int64, string, *int64, time.Time) (int64, RequestStoreStats, error)
}

func (e engine) observeRequest(operation string, started time.Time, stats RequestStoreStats, err error) {
	e.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
}

func requestValue[T any](e engine, operation string, call func() (T, RequestStoreStats, error)) (result T, err error) {
	started := time.Now()
	result, stats, err := call()
	e.observeRequest(operation, started, stats, err)
	return result, err
}

func requestCommand(e engine, ctx context.Context, operation string, call func(context.Context) (RequestStoreStats, error)) (err error) {
	started := time.Now()
	var stats RequestStoreStats
	err = e.withinTenant(ctx, func(txCtx context.Context) error {
		var commandErr error
		stats, commandErr = call(txCtx)
		return commandErr
	})
	e.observeRequest(operation, started, stats, err)
	return err
}

func requestCommandValue[T any](e engine, ctx context.Context, operation string, call func(context.Context) (T, RequestStoreStats, error)) (result T, err error) {
	started := time.Now()
	var stats RequestStoreStats
	err = e.withinTenant(ctx, func(txCtx context.Context) error {
		result, stats, err = call(txCtx)
		return err
	})
	e.observeRequest(operation, started, stats, err)
	return result, err
}

func (e engine) FindExcusedAbsenceRequest(ctx context.Context, id int64, lock bool) (careplan.ExcusedAbsenceRequest, error) {
	value, err := requestValue(e, "find_excused_absence_request", func() (careplan.ExcusedAbsenceRequest, RequestStoreStats, error) {
		value, found, stats, findErr := e.requests.FindExcusedAbsenceRequest(ctx, id, lock)
		if findErr == nil && !found {
			findErr = careplan.ErrExcusedRequestNotFound
		}
		return value, stats, findErr
	})
	return value, err
}
func (e engine) FindPendingExcusedAbsenceRequest(ctx context.Context, id int64) (careplan.ExcusedAbsenceRequest, error) {
	return requestValue(e, "find_pending_excused_absence_request", func() (careplan.ExcusedAbsenceRequest, RequestStoreStats, error) {
		return e.requests.FindPendingExcusedAbsenceRequest(ctx, id)
	})
}
func (e engine) ListExcusedAbsenceRequests(ctx context.Context, filter careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, error) {
	return requestValue(e, "list_excused_absence_requests", func() ([]careplan.ExcusedAbsenceRequest, RequestStoreStats, error) {
		return e.requests.ListExcusedAbsenceRequests(ctx, filter)
	})
}
func (e engine) CreateExcusedAbsenceRequest(ctx context.Context, value careplan.ExcusedAbsenceRequest) (careplan.ExcusedAbsenceRequest, error) {
	return requestCommandValue(e, ctx, "create_excused_absence_request", func(txCtx context.Context) (careplan.ExcusedAbsenceRequest, RequestStoreStats, error) {
		return e.requests.CreateExcusedAbsenceRequest(txCtx, value)
	})
}
func (e engine) LockExcusedAbsenceRequests(ctx context.Context, studentID int64) error {
	return requestCommand(e, ctx, "lock_excused_absence_requests", func(txCtx context.Context) (RequestStoreStats, error) {
		return e.requests.LockExcusedAbsenceRequests(txCtx, studentID)
	})
}
func (e engine) UpdatePendingExcusedAbsenceRequest(ctx context.Context, id int64, dates []careplan.Date, note, status string) error {
	return requestCommand(e, ctx, "update_pending_excused_absence_request", func(txCtx context.Context) (RequestStoreStats, error) {
		return e.requests.UpdatePendingExcusedAbsenceRequest(txCtx, id, dates, note, status)
	})
}
func (e engine) DecideExcusedAbsenceRequest(ctx context.Context, value careplan.ExcusedAbsenceDecision) error {
	return requestCommand(e, ctx, "decide_excused_absence_request", func(txCtx context.Context) (RequestStoreStats, error) {
		return e.requests.DecideExcusedAbsenceRequest(txCtx, value, false)
	})
}
func (e engine) RedecideExcusedAbsenceRequest(ctx context.Context, value careplan.ExcusedAbsenceDecision) error {
	return requestCommand(e, ctx, "redecide_excused_absence_request", func(txCtx context.Context) (RequestStoreStats, error) {
		return e.requests.DecideExcusedAbsenceRequest(txCtx, value, true)
	})
}

func (e engine) FindCareScheduleRequest(ctx context.Context, id int64, lock bool) (careplan.CareScheduleChangeRequest, error) {
	return requestValue(e, "find_care_schedule_request", func() (careplan.CareScheduleChangeRequest, RequestStoreStats, error) {
		value, found, stats, findErr := e.requests.FindCareScheduleRequest(ctx, id, lock)
		if findErr == nil && !found {
			findErr = careplan.ErrCareScheduleRequestNotFound
		}
		return value, stats, findErr
	})
}
func (e engine) FindPendingCareScheduleRequest(ctx context.Context, id int64) (careplan.CareScheduleChangeRequest, error) {
	return requestValue(e, "find_pending_care_schedule_request", func() (careplan.CareScheduleChangeRequest, RequestStoreStats, error) {
		return e.requests.FindPendingCareScheduleRequest(ctx, id)
	})
}
func (e engine) ListCareScheduleRequests(ctx context.Context, filter careplan.CareScheduleRequestFilter) ([]careplan.CareScheduleChangeRequest, error) {
	return requestValue(e, "list_care_schedule_requests", func() ([]careplan.CareScheduleChangeRequest, RequestStoreStats, error) {
		return e.requests.ListCareScheduleRequests(ctx, filter)
	})
}
func (e engine) CreateCareScheduleRequest(ctx context.Context, value careplan.CareScheduleChangeRequest) (careplan.CareScheduleChangeRequest, error) {
	return requestCommandValue(e, ctx, "create_care_schedule_request", func(txCtx context.Context) (careplan.CareScheduleChangeRequest, RequestStoreStats, error) {
		return e.requests.CreateCareScheduleRequest(txCtx, value)
	})
}
func (e engine) UpdatePendingCareScheduleRequest(ctx context.Context, id int64, payload json.RawMessage) error {
	return requestCommand(e, ctx, "update_pending_care_schedule_request", func(txCtx context.Context) (RequestStoreStats, error) {
		return e.requests.UpdatePendingCareScheduleRequest(txCtx, id, payload)
	})
}
func (e engine) DecideCareScheduleRequest(ctx context.Context, value careplan.CareScheduleRequestDecision) error {
	return requestCommand(e, ctx, "decide_care_schedule_request", func(txCtx context.Context) (RequestStoreStats, error) {
		return e.requests.DecideCareScheduleRequest(txCtx, value, false)
	})
}
func (e engine) RedecideCareScheduleRequest(ctx context.Context, value careplan.CareScheduleRequestDecision) error {
	return requestCommand(e, ctx, "redecide_care_schedule_request", func(txCtx context.Context) (RequestStoreStats, error) {
		return e.requests.DecideCareScheduleRequest(txCtx, value, true)
	})
}
func (e engine) UpdateCareScheduleRequestSnapshot(ctx context.Context, id int64, snapshot json.RawMessage) error {
	return requestCommand(e, ctx, "update_care_schedule_request_snapshot", func(txCtx context.Context) (RequestStoreStats, error) {
		return e.requests.UpdateCareScheduleRequestSnapshot(txCtx, id, snapshot)
	})
}

func (e engine) FindStudentDataRequest(ctx context.Context, id int64, lock bool) (careplan.StudentDataChangeRequest, error) {
	return requestValue(e, "find_student_data_request", func() (careplan.StudentDataChangeRequest, RequestStoreStats, error) {
		value, found, stats, findErr := e.requests.FindStudentDataRequest(ctx, id, lock)
		if findErr == nil && !found {
			findErr = careplan.ErrStudentDataRequestNotFound
		}
		return value, stats, findErr
	})
}
func (e engine) FindPendingStudentDataRequest(ctx context.Context, id int64) (careplan.StudentDataChangeRequest, error) {
	return requestValue(e, "find_pending_student_data_request", func() (careplan.StudentDataChangeRequest, RequestStoreStats, error) {
		return e.requests.FindPendingStudentDataRequest(ctx, id)
	})
}
func (e engine) ListStudentDataRequests(ctx context.Context, filter careplan.StudentDataRequestFilter) ([]careplan.StudentDataChangeRequest, error) {
	return requestValue(e, "list_student_data_requests", func() ([]careplan.StudentDataChangeRequest, RequestStoreStats, error) {
		return e.requests.ListStudentDataRequests(ctx, filter)
	})
}
func (e engine) HasPendingStudentDataRequest(ctx context.Context, studentID int64, target, field string) (bool, error) {
	return requestValue(e, "has_pending_student_data_request", func() (bool, RequestStoreStats, error) {
		return e.requests.HasPendingStudentDataRequest(ctx, studentID, target, field)
	})
}
func (e engine) CreateStudentDataRequest(ctx context.Context, value careplan.StudentDataChangeRequest) (careplan.StudentDataChangeRequest, error) {
	return requestCommandValue(e, ctx, "create_student_data_request", func(txCtx context.Context) (careplan.StudentDataChangeRequest, RequestStoreStats, error) {
		return e.requests.CreateStudentDataRequest(txCtx, value)
	})
}
func (e engine) UpdatePendingStudentDataRequest(ctx context.Context, id int64, value json.RawMessage) error {
	return requestCommand(e, ctx, "update_pending_student_data_request", func(txCtx context.Context) (RequestStoreStats, error) {
		return e.requests.UpdatePendingStudentDataRequest(txCtx, id, value)
	})
}
func (e engine) DecideStudentDataRequest(ctx context.Context, value careplan.StudentDataRequestDecision) error {
	return requestCommand(e, ctx, "decide_student_data_request", func(txCtx context.Context) (RequestStoreStats, error) {
		return e.requests.DecideStudentDataRequest(txCtx, value, false)
	})
}
func (e engine) RedecideStudentDataRequest(ctx context.Context, value careplan.StudentDataRequestDecision) error {
	return requestCommand(e, ctx, "redecide_student_data_request", func(txCtx context.Context) (RequestStoreStats, error) {
		return e.requests.DecideStudentDataRequest(txCtx, value, true)
	})
}

func (e engine) CountOpenCareRequests(ctx context.Context, studentIDs []int64) (map[int64]int, error) {
	return requestValue(e, "count_open_care_requests", func() (map[int64]int, RequestStoreStats, error) {
		return e.requests.CountOpenCareRequests(ctx, studentIDs)
	})
}

func (e engine) LockOpenCareRequests(ctx context.Context, studentIDs []int64) error {
	return requestCommand(e, ctx, "lock_open_care_requests", func(txCtx context.Context) (RequestStoreStats, error) {
		return e.requests.LockOpenCareRequests(txCtx, studentIDs)
	})
}

func (e engine) CloseOpenCareRequests(ctx context.Context, studentIDs []int64, reason string, reviewedBy *int64, at time.Time) (int64, error) {
	return requestCommandValue(e, ctx, "close_open_care_requests", func(txCtx context.Context) (int64, RequestStoreStats, error) {
		return e.requests.CloseOpenCareRequests(txCtx, studentIDs, reason, reviewedBy, at)
	})
}
