package application

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

func (s *Service) FindExcusedAbsenceRequest(ctx context.Context, id int64, lock bool) (careplan.ExcusedAbsenceRequest, error) {
	value, err := requestValue(ctx, s, "find_excused_absence_request", func(ctx context.Context) (careplan.ExcusedAbsenceRequest, domain.OperationStats, error) {
		value, found, stats, findErr := s.requests.FindExcusedAbsenceRequest(ctx, id, lock)
		if findErr == nil && !found {
			findErr = careplan.ErrExcusedRequestNotFound
		}
		return value, stats, findErr
	})
	return value, err
}

func (s *Service) FindPendingExcusedAbsenceRequest(ctx context.Context, id int64) (careplan.ExcusedAbsenceRequest, error) {
	return requestValue(ctx, s, "find_pending_excused_absence_request", func(ctx context.Context) (careplan.ExcusedAbsenceRequest, domain.OperationStats, error) {
		return s.requests.FindPendingExcusedAbsenceRequest(ctx, id)
	})
}

func (s *Service) ListExcusedAbsenceRequests(ctx context.Context, filter careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, error) {
	filter.IDs = uniquePositive(filter.IDs)
	filter.Statuses = uniqueStrings(filter.Statuses)

	return requestValue(ctx, s, "list_excused_absence_requests", func(ctx context.Context) ([]careplan.ExcusedAbsenceRequest, domain.OperationStats, error) {
		return s.requests.ListExcusedAbsenceRequests(ctx, filter)
	})
}

func (s *Service) CreateExcusedAbsenceRequest(ctx context.Context, value careplan.ExcusedAbsenceRequest) (careplan.ExcusedAbsenceRequest, error) {
	return requestCommandValue(s, ctx, "create_excused_absence_request", func(txCtx context.Context) (careplan.ExcusedAbsenceRequest, domain.OperationStats, error) {
		return s.requests.CreateExcusedAbsenceRequest(txCtx, value)
	})
}

func (s *Service) LockExcusedAbsenceRequests(ctx context.Context, studentID int64) error {
	return requestCommand(s, ctx, "lock_excused_absence_requests", func(txCtx context.Context) (domain.OperationStats, error) {
		return s.requests.LockExcusedAbsenceRequests(txCtx, studentID)
	})
}

func (s *Service) UpdatePendingExcusedAbsenceRequest(ctx context.Context, id int64, dates []careplan.Date, note, status string) error {
	return requestCommand(s, ctx, "update_pending_excused_absence_request", func(txCtx context.Context) (domain.OperationStats, error) {
		return s.requests.UpdatePendingExcusedAbsenceRequest(txCtx, id, dates, note, status)
	})
}

func (s *Service) DecideExcusedAbsenceRequest(ctx context.Context, value careplan.ExcusedAbsenceDecision) error {
	return requestCommand(s, ctx, "decide_excused_absence_request", func(txCtx context.Context) (domain.OperationStats, error) {
		return s.requests.DecideExcusedAbsenceRequest(txCtx, value, false)
	})
}

func (s *Service) RedecideExcusedAbsenceRequest(ctx context.Context, value careplan.ExcusedAbsenceDecision) error {
	return requestCommand(s, ctx, "redecide_excused_absence_request", func(txCtx context.Context) (domain.OperationStats, error) {
		return s.requests.DecideExcusedAbsenceRequest(txCtx, value, true)
	})
}

func (s *Service) FindCareScheduleRequest(ctx context.Context, id int64, lock bool) (careplan.CareScheduleChangeRequest, error) {
	return requestValue(ctx, s, "find_care_schedule_request", func(ctx context.Context) (careplan.CareScheduleChangeRequest, domain.OperationStats, error) {
		value, found, stats, findErr := s.requests.FindCareScheduleRequest(ctx, id, lock)
		if findErr == nil && !found {
			findErr = careplan.ErrCareScheduleRequestNotFound
		}
		return value, stats, findErr
	})
}

func (s *Service) FindPendingCareScheduleRequest(ctx context.Context, id int64) (careplan.CareScheduleChangeRequest, error) {
	return requestValue(ctx, s, "find_pending_care_schedule_request", func(ctx context.Context) (careplan.CareScheduleChangeRequest, domain.OperationStats, error) {
		return s.requests.FindPendingCareScheduleRequest(ctx, id)
	})
}

func (s *Service) ListCareScheduleRequests(ctx context.Context, filter careplan.CareScheduleRequestFilter) ([]careplan.CareScheduleChangeRequest, error) {
	filter.RequestKinds = uniqueStrings(filter.RequestKinds)
	filter.Statuses = uniqueStrings(filter.Statuses)

	return requestValue(ctx, s, "list_care_schedule_requests", func(ctx context.Context) ([]careplan.CareScheduleChangeRequest, domain.OperationStats, error) {
		return s.requests.ListCareScheduleRequests(ctx, filter)
	})
}

func (s *Service) CreateCareScheduleRequest(ctx context.Context, value careplan.CareScheduleChangeRequest) (careplan.CareScheduleChangeRequest, error) {
	return requestCommandValue(s, ctx, "create_care_schedule_request", func(txCtx context.Context) (careplan.CareScheduleChangeRequest, domain.OperationStats, error) {
		return s.requests.CreateCareScheduleRequest(txCtx, value)
	})
}

func (s *Service) UpdatePendingCareScheduleRequest(ctx context.Context, id int64, payload json.RawMessage) error {
	return requestCommand(s, ctx, "update_pending_care_schedule_request", func(txCtx context.Context) (domain.OperationStats, error) {
		return s.requests.UpdatePendingCareScheduleRequest(txCtx, id, payload)
	})
}

func (s *Service) DecideCareScheduleRequest(ctx context.Context, value careplan.CareScheduleRequestDecision) error {
	return requestCommand(s, ctx, "decide_care_schedule_request", func(txCtx context.Context) (domain.OperationStats, error) {
		return s.requests.DecideCareScheduleRequest(txCtx, value, false)
	})
}

func (s *Service) RedecideCareScheduleRequest(ctx context.Context, value careplan.CareScheduleRequestDecision) error {
	return requestCommand(s, ctx, "redecide_care_schedule_request", func(txCtx context.Context) (domain.OperationStats, error) {
		return s.requests.DecideCareScheduleRequest(txCtx, value, true)
	})
}

func (s *Service) UpdateCareScheduleRequestSnapshot(ctx context.Context, id int64, snapshot json.RawMessage) error {
	return requestCommand(s, ctx, "update_care_schedule_request_snapshot", func(txCtx context.Context) (domain.OperationStats, error) {
		return s.requests.UpdateCareScheduleRequestSnapshot(txCtx, id, snapshot)
	})
}

func (s *Service) FindStudentDataRequest(ctx context.Context, id int64, lock bool) (careplan.StudentDataChangeRequest, error) {
	return requestValue(ctx, s, "find_student_data_request", func(ctx context.Context) (careplan.StudentDataChangeRequest, domain.OperationStats, error) {
		value, found, stats, findErr := s.requests.FindStudentDataRequest(ctx, id, lock)
		if findErr == nil && !found {
			findErr = careplan.ErrStudentDataRequestNotFound
		}
		return value, stats, findErr
	})
}

func (s *Service) FindPendingStudentDataRequest(ctx context.Context, id int64) (careplan.StudentDataChangeRequest, error) {
	return requestValue(ctx, s, "find_pending_student_data_request", func(ctx context.Context) (careplan.StudentDataChangeRequest, domain.OperationStats, error) {
		return s.requests.FindPendingStudentDataRequest(ctx, id)
	})
}

func (s *Service) ListStudentDataRequests(ctx context.Context, filter careplan.StudentDataRequestFilter) ([]careplan.StudentDataChangeRequest, error) {
	filter.Statuses = uniqueStrings(filter.Statuses)

	return requestValue(ctx, s, "list_student_data_requests", func(ctx context.Context) ([]careplan.StudentDataChangeRequest, domain.OperationStats, error) {
		return s.requests.ListStudentDataRequests(ctx, filter)
	})
}

func (s *Service) HasPendingStudentDataRequest(ctx context.Context, studentID int64, target, field string) (bool, error) {
	return requestValue(ctx, s, "has_pending_student_data_request", func(ctx context.Context) (bool, domain.OperationStats, error) {
		return s.requests.HasPendingStudentDataRequest(ctx, studentID, target, field)
	})
}

func (s *Service) CreateStudentDataRequest(ctx context.Context, value careplan.StudentDataChangeRequest) (careplan.StudentDataChangeRequest, error) {
	return requestCommandValue(s, ctx, "create_student_data_request", func(txCtx context.Context) (careplan.StudentDataChangeRequest, domain.OperationStats, error) {
		return s.requests.CreateStudentDataRequest(txCtx, value)
	})
}

func (s *Service) UpdatePendingStudentDataRequest(ctx context.Context, id int64, value json.RawMessage) error {
	return requestCommand(s, ctx, "update_pending_student_data_request", func(txCtx context.Context) (domain.OperationStats, error) {
		return s.requests.UpdatePendingStudentDataRequest(txCtx, id, value)
	})
}

func (s *Service) DecideStudentDataRequest(ctx context.Context, value careplan.StudentDataRequestDecision) error {
	return requestCommand(s, ctx, "decide_student_data_request", func(txCtx context.Context) (domain.OperationStats, error) {
		return s.requests.DecideStudentDataRequest(txCtx, value, false)
	})
}

func (s *Service) RedecideStudentDataRequest(ctx context.Context, value careplan.StudentDataRequestDecision) error {
	return requestCommand(s, ctx, "redecide_student_data_request", func(txCtx context.Context) (domain.OperationStats, error) {
		return s.requests.DecideStudentDataRequest(txCtx, value, true)
	})
}

func (s *Service) CountOpenCareRequests(ctx context.Context, studentIDs []int64) (map[int64]int, error) {
	ids := uniquePositive(studentIDs)
	if len(ids) == 0 {
		return map[int64]int{}, nil
	}

	return requestValue(ctx, s, "count_open_care_requests", func(ctx context.Context) (map[int64]int, domain.OperationStats, error) {
		return s.requests.CountOpenCareRequests(ctx, ids)
	})
}

func (s *Service) LockOpenCareRequests(ctx context.Context, studentIDs []int64) error {
	ids := uniquePositive(studentIDs)
	if len(ids) == 0 {
		return nil
	}

	return requestCommand(s, ctx, "lock_open_care_requests", func(txCtx context.Context) (domain.OperationStats, error) {
		return s.requests.LockOpenCareRequests(txCtx, ids)
	})
}

func (s *Service) CloseOpenCareRequests(ctx context.Context, studentIDs []int64, reason string, reviewedBy *int64, at time.Time) (int64, error) {
	ids := uniquePositive(studentIDs)
	if len(ids) == 0 {
		return 0, nil
	}

	return requestCommandValue(s, ctx, "close_open_care_requests", func(txCtx context.Context) (int64, domain.OperationStats, error) {
		return s.requests.CloseOpenCareRequests(txCtx, ids, reason, reviewedBy, at)
	})
}
