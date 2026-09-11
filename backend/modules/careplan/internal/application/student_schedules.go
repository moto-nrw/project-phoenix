package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

func notFound(found bool, err error) error {
	if err == nil && !found {
		return careplan.ErrStudentScheduleNotFound
	}
	return err
}

func (s *Service) FindArrivalSchedule(ctx context.Context, id int64) (result careplan.ArrivalSchedule, err error) {
	err = s.run("find_arrival_schedule", func(stats *domain.OperationStats) error {
		var found bool
		var q domain.OperationStats
		result, found, q, err = s.store.FindArrivalSchedule(ctx, id)
		stats.Add(q)
		return notFound(found, err)
	})
	return
}
func (s *Service) ListArrivalSchedules(ctx context.Context, f careplan.StudentScheduleFilter) (result []careplan.ArrivalSchedule, err error) {
	err = s.run("list_arrival_schedules", func(stats *domain.OperationStats) error {
		var q domain.OperationStats
		result, q, err = s.store.ListArrivalSchedules(ctx, f)
		stats.Add(q)
		return err
	})
	return
}
func (s *Service) CreateArrivalSchedule(ctx context.Context, v careplan.ArrivalSchedule) (result careplan.ArrivalSchedule, err error) {
	err = s.run("create_arrival_schedule", func(stats *domain.OperationStats) error {
		var q domain.OperationStats
		result, q, err = s.store.CreateArrivalSchedule(ctx, v)
		stats.Add(q)
		return err
	})
	return
}
func (s *Service) UpdateArrivalSchedule(ctx context.Context, v careplan.ArrivalSchedule) (err error) {
	return s.run("update_arrival_schedule", func(stats *domain.OperationStats) error {
		q, runErr := s.store.UpdateArrivalSchedule(ctx, v)
		stats.Add(q)
		return runErr
	})
}
func (s *Service) UpsertArrivalSchedule(ctx context.Context, v careplan.ArrivalSchedule) (result careplan.ArrivalSchedule, err error) {
	err = s.run("upsert_arrival_schedule", func(stats *domain.OperationStats) error {
		var q domain.OperationStats
		result, q, err = s.store.UpsertArrivalSchedule(ctx, v)
		stats.Add(q)
		return err
	})
	return
}
func (s *Service) DeleteArrivalSchedule(ctx context.Context, id int64) error {
	return s.run("delete_arrival_schedule", func(stats *domain.OperationStats) error {
		q, err := s.store.DeleteArrivalSchedule(ctx, id)
		stats.Add(q)
		return err
	})
}
func (s *Service) DeleteArrivalSchedulesByStudent(ctx context.Context, id int64) error {
	return s.run("delete_arrival_schedules_by_student", func(stats *domain.OperationStats) error {
		q, err := s.store.DeleteArrivalSchedulesByStudent(ctx, id)
		stats.Add(q)
		return err
	})
}

func (s *Service) FindArrivalException(ctx context.Context, id int64, lock bool) (result careplan.ArrivalException, err error) {
	err = s.run("find_arrival_exception", func(stats *domain.OperationStats) error {
		var found bool
		var q domain.OperationStats
		result, found, q, err = s.store.FindArrivalException(ctx, id, lock)
		stats.Add(q)
		return notFound(found, err)
	})
	return
}
func (s *Service) ListArrivalExceptions(ctx context.Context, f careplan.StudentScheduleFilter) (result []careplan.ArrivalException, err error) {
	err = s.run("list_arrival_exceptions", func(stats *domain.OperationStats) error {
		var q domain.OperationStats
		result, q, err = s.store.ListArrivalExceptions(ctx, f)
		stats.Add(q)
		return err
	})
	return
}
func (s *Service) CreateArrivalException(ctx context.Context, v careplan.ArrivalException) (result careplan.ArrivalException, err error) {
	err = s.run("create_arrival_exception", func(stats *domain.OperationStats) error {
		var q domain.OperationStats
		result, q, err = s.store.CreateArrivalException(ctx, v)
		stats.Add(q)
		return err
	})
	return
}
func (s *Service) UpdateArrivalException(ctx context.Context, v careplan.ArrivalException) error {
	return s.run("update_arrival_exception", func(stats *domain.OperationStats) error {
		q, err := s.store.UpdateArrivalException(ctx, v)
		stats.Add(q)
		return err
	})
}
func (s *Service) DeleteArrivalException(ctx context.Context, id int64) error {
	return s.run("delete_arrival_exception", func(stats *domain.OperationStats) error {
		q, err := s.store.DeleteArrivalException(ctx, id)
		stats.Add(q)
		return err
	})
}
func (s *Service) DeleteArrivalExceptionsByStudent(ctx context.Context, id int64) error {
	return s.run("delete_arrival_exceptions_by_student", func(stats *domain.OperationStats) error {
		q, err := s.store.DeleteArrivalExceptionsByStudent(ctx, id)
		stats.Add(q)
		return err
	})
}
func (s *Service) DeleteArrivalExceptionsBefore(ctx context.Context, d careplan.Date) (rows int64, err error) {
	err = s.run("delete_arrival_exceptions_before", func(stats *domain.OperationStats) error {
		q, runErr := s.store.DeleteArrivalExceptionsBefore(ctx, d)
		stats.Add(q)
		rows = q.Rows
		return runErr
	})
	return
}

func (s *Service) FindArrivalNote(ctx context.Context, id int64) (result careplan.ArrivalNote, err error) {
	err = s.run("find_arrival_note", func(stats *domain.OperationStats) error {
		var found bool
		var q domain.OperationStats
		result, found, q, err = s.store.FindArrivalNote(ctx, id)
		stats.Add(q)
		return notFound(found, err)
	})
	return
}
func (s *Service) ListArrivalNotes(ctx context.Context, f careplan.StudentScheduleFilter) (result []careplan.ArrivalNote, err error) {
	err = s.run("list_arrival_notes", func(stats *domain.OperationStats) error {
		var q domain.OperationStats
		result, q, err = s.store.ListArrivalNotes(ctx, f)
		stats.Add(q)
		return err
	})
	return
}
func (s *Service) CreateArrivalNote(ctx context.Context, v careplan.ArrivalNote) (result careplan.ArrivalNote, err error) {
	err = s.run("create_arrival_note", func(stats *domain.OperationStats) error {
		var q domain.OperationStats
		result, q, err = s.store.CreateArrivalNote(ctx, v)
		stats.Add(q)
		return err
	})
	return
}
func (s *Service) UpdateArrivalNote(ctx context.Context, v careplan.ArrivalNote) error {
	return s.run("update_arrival_note", func(stats *domain.OperationStats) error {
		q, err := s.store.UpdateArrivalNote(ctx, v)
		stats.Add(q)
		return err
	})
}
func (s *Service) DeleteArrivalNote(ctx context.Context, id int64) error {
	return s.run("delete_arrival_note", func(stats *domain.OperationStats) error {
		q, err := s.store.DeleteArrivalNote(ctx, id)
		stats.Add(q)
		return err
	})
}
func (s *Service) DeleteArrivalNotesByStudent(ctx context.Context, id int64) error {
	return s.run("delete_arrival_notes_by_student", func(stats *domain.OperationStats) error {
		q, err := s.store.DeleteArrivalNotesByStudent(ctx, id)
		stats.Add(q)
		return err
	})
}
func (s *Service) DeleteArrivalNotesBefore(ctx context.Context, d careplan.Date) (rows int64, err error) {
	err = s.run("delete_arrival_notes_before", func(stats *domain.OperationStats) error {
		q, runErr := s.store.DeleteArrivalNotesBefore(ctx, d)
		stats.Add(q)
		rows = q.Rows
		return runErr
	})
	return
}

func (s *Service) FindPickupSchedule(ctx context.Context, id int64) (result careplan.PickupSchedule, err error) {
	err = s.run("find_pickup_schedule", func(stats *domain.OperationStats) error {
		var found bool
		var q domain.OperationStats
		result, found, q, err = s.store.FindPickupSchedule(ctx, id)
		stats.Add(q)
		return notFound(found, err)
	})
	return
}
func (s *Service) ListPickupSchedules(ctx context.Context, f careplan.StudentScheduleFilter) (result []careplan.PickupSchedule, err error) {
	err = s.run("list_pickup_schedules", func(stats *domain.OperationStats) error {
		var q domain.OperationStats
		result, q, err = s.store.ListPickupSchedules(ctx, f)
		stats.Add(q)
		return err
	})
	return
}
func (s *Service) CreatePickupSchedule(ctx context.Context, v careplan.PickupSchedule) (result careplan.PickupSchedule, err error) {
	err = s.run("create_pickup_schedule", func(stats *domain.OperationStats) error {
		var q domain.OperationStats
		result, q, err = s.store.CreatePickupSchedule(ctx, v)
		stats.Add(q)
		return err
	})
	return
}
func (s *Service) UpdatePickupSchedule(ctx context.Context, v careplan.PickupSchedule) error {
	return s.run("update_pickup_schedule", func(stats *domain.OperationStats) error {
		q, err := s.store.UpdatePickupSchedule(ctx, v)
		stats.Add(q)
		return err
	})
}
func (s *Service) UpsertPickupSchedule(ctx context.Context, v careplan.PickupSchedule) (result careplan.PickupSchedule, err error) {
	err = s.run("upsert_pickup_schedule", func(stats *domain.OperationStats) error {
		var q domain.OperationStats
		result, q, err = s.store.UpsertPickupSchedule(ctx, v)
		stats.Add(q)
		return err
	})
	return
}
func (s *Service) DeletePickupSchedule(ctx context.Context, id int64) error {
	return s.run("delete_pickup_schedule", func(stats *domain.OperationStats) error {
		q, err := s.store.DeletePickupSchedule(ctx, id)
		stats.Add(q)
		return err
	})
}
func (s *Service) DeletePickupSchedulesByStudent(ctx context.Context, id int64) error {
	return s.run("delete_pickup_schedules_by_student", func(stats *domain.OperationStats) error {
		q, err := s.store.DeletePickupSchedulesByStudent(ctx, id)
		stats.Add(q)
		return err
	})
}

func (s *Service) FindPickupException(ctx context.Context, id int64, lock bool) (result careplan.PickupException, err error) {
	err = s.run("find_pickup_exception", func(stats *domain.OperationStats) error {
		var found bool
		var q domain.OperationStats
		result, found, q, err = s.store.FindPickupException(ctx, id, lock)
		stats.Add(q)
		return notFound(found, err)
	})
	return
}
func (s *Service) ListPickupExceptions(ctx context.Context, f careplan.StudentScheduleFilter) (result []careplan.PickupException, err error) {
	err = s.run("list_pickup_exceptions", func(stats *domain.OperationStats) error {
		var q domain.OperationStats
		result, q, err = s.store.ListPickupExceptions(ctx, f)
		stats.Add(q)
		return err
	})
	return
}
func (s *Service) CreatePickupException(ctx context.Context, v careplan.PickupException) (result careplan.PickupException, err error) {
	err = s.run("create_pickup_exception", func(stats *domain.OperationStats) error {
		var q domain.OperationStats
		result, q, err = s.store.CreatePickupException(ctx, v)
		stats.Add(q)
		return err
	})
	return
}
func (s *Service) UpdatePickupException(ctx context.Context, v careplan.PickupException) error {
	return s.run("update_pickup_exception", func(stats *domain.OperationStats) error {
		q, err := s.store.UpdatePickupException(ctx, v)
		stats.Add(q)
		return err
	})
}
func (s *Service) DeletePickupException(ctx context.Context, id int64) error {
	return s.run("delete_pickup_exception", func(stats *domain.OperationStats) error {
		q, err := s.store.DeletePickupException(ctx, id)
		stats.Add(q)
		return err
	})
}
func (s *Service) DeletePickupExceptionsByStudent(ctx context.Context, id int64) error {
	return s.run("delete_pickup_exceptions_by_student", func(stats *domain.OperationStats) error {
		q, err := s.store.DeletePickupExceptionsByStudent(ctx, id)
		stats.Add(q)
		return err
	})
}
func (s *Service) DeletePickupExceptionsBefore(ctx context.Context, d careplan.Date) (rows int64, err error) {
	err = s.run("delete_pickup_exceptions_before", func(stats *domain.OperationStats) error {
		q, runErr := s.store.DeletePickupExceptionsBefore(ctx, d)
		stats.Add(q)
		rows = q.Rows
		return runErr
	})
	return
}

func (s *Service) FindPickupNote(ctx context.Context, id int64) (result careplan.PickupNote, err error) {
	err = s.run("find_pickup_note", func(stats *domain.OperationStats) error {
		var found bool
		var q domain.OperationStats
		result, found, q, err = s.store.FindPickupNote(ctx, id)
		stats.Add(q)
		return notFound(found, err)
	})
	return
}
func (s *Service) ListPickupNotes(ctx context.Context, f careplan.StudentScheduleFilter) (result []careplan.PickupNote, err error) {
	err = s.run("list_pickup_notes", func(stats *domain.OperationStats) error {
		var q domain.OperationStats
		result, q, err = s.store.ListPickupNotes(ctx, f)
		stats.Add(q)
		return err
	})
	return
}
func (s *Service) CreatePickupNote(ctx context.Context, v careplan.PickupNote) (result careplan.PickupNote, err error) {
	err = s.run("create_pickup_note", func(stats *domain.OperationStats) error {
		var q domain.OperationStats
		result, q, err = s.store.CreatePickupNote(ctx, v)
		stats.Add(q)
		return err
	})
	return
}
func (s *Service) UpdatePickupNote(ctx context.Context, v careplan.PickupNote) error {
	return s.run("update_pickup_note", func(stats *domain.OperationStats) error {
		q, err := s.store.UpdatePickupNote(ctx, v)
		stats.Add(q)
		return err
	})
}
func (s *Service) DeletePickupNote(ctx context.Context, id int64) error {
	return s.run("delete_pickup_note", func(stats *domain.OperationStats) error {
		q, err := s.store.DeletePickupNote(ctx, id)
		stats.Add(q)
		return err
	})
}
func (s *Service) DeletePickupNotesByStudent(ctx context.Context, id int64) error {
	return s.run("delete_pickup_notes_by_student", func(stats *domain.OperationStats) error {
		q, err := s.store.DeletePickupNotesByStudent(ctx, id)
		stats.Add(q)
		return err
	})
}
func (s *Service) DeletePickupNotesBefore(ctx context.Context, d careplan.Date) (rows int64, err error) {
	err = s.run("delete_pickup_notes_before", func(stats *domain.OperationStats) error {
		q, runErr := s.store.DeletePickupNotesBefore(ctx, d)
		stats.Add(q)
		rows = q.Rows
		return runErr
	})
	return
}

func (s *Service) EndStudentSchedulesForCareExit(ctx context.Context, studentIDs []int64, validUntil careplan.Date) (rows int64, err error) {
	err = s.run("end_student_schedules_for_care_exit", func(stats *domain.OperationStats) error {
		q, runErr := s.store.EndStudentSchedulesForCareExit(ctx, studentIDs, validUntil)
		stats.Add(q)
		rows = q.Rows
		return runErr
	})
	return
}

func (s *Service) RestoreStudentSchedulesForCareExit(ctx context.Context, studentIDs []int64) (rows int64, err error) {
	err = s.run("restore_student_schedules_for_care_exit", func(stats *domain.OperationStats) error {
		q, runErr := s.store.RestoreStudentSchedulesForCareExit(ctx, studentIDs)
		stats.Add(q)
		rows = q.Rows
		return runErr
	})
	return
}

func (s *Service) CountStudentScheduleRows(ctx context.Context, studentID int64) (result int, err error) {
	err = s.run("count_student_schedule_rows", func(stats *domain.OperationStats) error {
		var q domain.OperationStats
		result, q, err = s.store.CountStudentScheduleRows(ctx, studentID)
		stats.Add(q)
		return err
	})
	return
}
