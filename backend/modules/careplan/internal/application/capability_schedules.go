package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

func (s *Service) FindArrivalSchedule(ctx context.Context, id int64) (careplan.ArrivalSchedule, error) {
	if id <= 0 {
		return careplan.ArrivalSchedule{}, invalid(careplan.ErrInvalidStudentSchedule, "arrival schedule ID is required")
	}

	started := time.Now()
	value, found, stats, err := scopedRecordFind(ctx, func(txCtx context.Context) (careplan.ArrivalSchedule, bool, domain.OperationStats, error) {
		return s.statusQueries.FindArrivalSchedule(txCtx, id)
	})
	if err == nil && !found {
		err = careplan.ErrStudentScheduleNotFound
	}

	s.observeRequest("find_arrival_schedule", started, stats, err)
	return value, err
}

func (s *Service) ListArrivalSchedules(ctx context.Context, filter careplan.StudentScheduleFilter) ([]careplan.ArrivalSchedule, error) {
	filter = normalizeStudentScheduleFilter(filter)
	started := time.Now()
	value, stats, err := scopedRecordRead(ctx, func(txCtx context.Context) ([]careplan.ArrivalSchedule, domain.OperationStats, error) {
		return s.statusQueries.ListArrivalSchedules(txCtx, filter)
	})

	s.observeRequest("list_arrival_schedules", started, stats, err)
	return value, err
}

func (s *Service) FindArrivalException(ctx context.Context, id int64, lock bool) (careplan.ArrivalException, error) {
	if id <= 0 {
		return careplan.ArrivalException{}, invalid(careplan.ErrInvalidStudentSchedule, "arrival exception ID is required")
	}

	started := time.Now()
	value, found, stats, err := scopedRecordFind(ctx, func(txCtx context.Context) (careplan.ArrivalException, bool, domain.OperationStats, error) {
		return s.statusQueries.FindArrivalException(txCtx, id, lock)
	})
	if err == nil && !found {
		err = careplan.ErrStudentScheduleNotFound
	}

	s.observeRequest("find_arrival_exception", started, stats, err)
	return value, err
}

func (s *Service) ListArrivalExceptions(ctx context.Context, filter careplan.StudentScheduleFilter) ([]careplan.ArrivalException, error) {
	filter = normalizeStudentScheduleFilter(filter)
	started := time.Now()
	value, stats, err := scopedRecordRead(ctx, func(txCtx context.Context) ([]careplan.ArrivalException, domain.OperationStats, error) {
		return s.statusQueries.ListArrivalExceptions(txCtx, filter)
	})

	s.observeRequest("list_arrival_exceptions", started, stats, err)
	return value, err
}

func (s *Service) FindArrivalNote(ctx context.Context, id int64) (careplan.ArrivalNote, error) {
	if id <= 0 {
		return careplan.ArrivalNote{}, invalid(careplan.ErrInvalidStudentSchedule, "arrival note ID is required")
	}

	started := time.Now()
	value, found, stats, err := scopedRecordFind(ctx, func(txCtx context.Context) (careplan.ArrivalNote, bool, domain.OperationStats, error) {
		return s.statusQueries.FindArrivalNote(txCtx, id)
	})
	if err == nil && !found {
		err = careplan.ErrStudentScheduleNotFound
	}

	s.observeRequest("find_arrival_note", started, stats, err)
	return value, err
}

func (s *Service) ListArrivalNotes(ctx context.Context, filter careplan.StudentScheduleFilter) ([]careplan.ArrivalNote, error) {
	filter = normalizeStudentScheduleFilter(filter)
	started := time.Now()
	value, stats, err := scopedRecordRead(ctx, func(txCtx context.Context) ([]careplan.ArrivalNote, domain.OperationStats, error) {
		return s.statusQueries.ListArrivalNotes(txCtx, filter)
	})

	s.observeRequest("list_arrival_notes", started, stats, err)
	return value, err
}

func (s *Service) FindPickupSchedule(ctx context.Context, id int64) (careplan.PickupSchedule, error) {
	if id <= 0 {
		return careplan.PickupSchedule{}, invalid(careplan.ErrInvalidStudentSchedule, "pickup schedule ID is required")
	}

	started := time.Now()
	value, found, stats, err := scopedRecordFind(ctx, func(txCtx context.Context) (careplan.PickupSchedule, bool, domain.OperationStats, error) {
		return s.statusQueries.FindPickupSchedule(txCtx, id)
	})
	if err == nil && !found {
		err = careplan.ErrStudentScheduleNotFound
	}

	s.observeRequest("find_pickup_schedule", started, stats, err)
	return value, err
}

func (s *Service) ListPickupSchedules(ctx context.Context, filter careplan.StudentScheduleFilter) ([]careplan.PickupSchedule, error) {
	filter = normalizeStudentScheduleFilter(filter)
	started := time.Now()
	value, stats, err := scopedRecordRead(ctx, func(txCtx context.Context) ([]careplan.PickupSchedule, domain.OperationStats, error) {
		return s.statusQueries.ListPickupSchedules(txCtx, filter)
	})

	s.observeRequest("list_pickup_schedules", started, stats, err)
	return value, err
}

func (s *Service) FindPickupException(ctx context.Context, id int64, lock bool) (careplan.PickupException, error) {
	if id <= 0 {
		return careplan.PickupException{}, invalid(careplan.ErrInvalidStudentSchedule, "pickup exception ID is required")
	}

	started := time.Now()
	value, found, stats, err := scopedRecordFind(ctx, func(txCtx context.Context) (careplan.PickupException, bool, domain.OperationStats, error) {
		return s.statusQueries.FindPickupException(txCtx, id, lock)
	})
	if err == nil && !found {
		err = careplan.ErrStudentScheduleNotFound
	}

	s.observeRequest("find_pickup_exception", started, stats, err)
	return value, err
}

func (s *Service) ListPickupExceptions(ctx context.Context, filter careplan.StudentScheduleFilter) ([]careplan.PickupException, error) {
	filter = normalizeStudentScheduleFilter(filter)
	started := time.Now()
	value, stats, err := scopedRecordRead(ctx, func(txCtx context.Context) ([]careplan.PickupException, domain.OperationStats, error) {
		return s.statusQueries.ListPickupExceptions(txCtx, filter)
	})

	s.observeRequest("list_pickup_exceptions", started, stats, err)
	return value, err
}

func (s *Service) FindPickupNote(ctx context.Context, id int64) (careplan.PickupNote, error) {
	if id <= 0 {
		return careplan.PickupNote{}, invalid(careplan.ErrInvalidStudentSchedule, "pickup note ID is required")
	}

	started := time.Now()
	value, found, stats, err := scopedRecordFind(ctx, func(txCtx context.Context) (careplan.PickupNote, bool, domain.OperationStats, error) {
		return s.statusQueries.FindPickupNote(txCtx, id)
	})
	if err == nil && !found {
		err = careplan.ErrStudentScheduleNotFound
	}

	s.observeRequest("find_pickup_note", started, stats, err)
	return value, err
}

func (s *Service) ListPickupNotes(ctx context.Context, filter careplan.StudentScheduleFilter) ([]careplan.PickupNote, error) {
	filter = normalizeStudentScheduleFilter(filter)
	started := time.Now()
	value, stats, err := scopedRecordRead(ctx, func(txCtx context.Context) ([]careplan.PickupNote, domain.OperationStats, error) {
		return s.statusQueries.ListPickupNotes(txCtx, filter)
	})

	s.observeRequest("list_pickup_notes", started, stats, err)
	return value, err
}

func (s *Service) CountStudentScheduleRows(ctx context.Context, studentID int64) (int, error) {
	if studentID <= 0 {
		return 0, invalid(careplan.ErrInvalidStudentSchedule, "student ID is required")
	}

	started := time.Now()
	value, stats, err := scopedRecordRead(ctx, func(txCtx context.Context) (int, domain.OperationStats, error) {
		return s.statusQueries.CountStudentScheduleRows(txCtx, studentID)
	})

	s.observeRequest("count_student_schedule_rows", started, stats, err)
	return value, err
}

func (s *Service) EndStudentSchedulesForCareExit(ctx context.Context, studentIDs []int64, validUntil careplan.Date) (rows int64, err error) {
	studentIDs = uniquePositive(studentIDs)
	if len(studentIDs) == 0 {
		return 0, nil
	}
	if validUntil.IsZero() {
		return 0, invalid(careplan.ErrInvalidStudentSchedule, "care-exit date is required")
	}

	err = s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "end_student_schedules_for_care_exit", func(ctx context.Context, stats *domain.OperationStats) error {
			q, runErr := s.store.EndStudentSchedulesForCareExit(ctx, studentIDs, validUntil)
			stats.Add(q)
			rows = q.Rows
			return runErr
		})
	})
	return rows, err
}

func (s *Service) RestoreStudentSchedulesForCareExit(ctx context.Context, studentIDs []int64) (rows int64, err error) {
	studentIDs = uniquePositive(studentIDs)
	if len(studentIDs) == 0 {
		return 0, nil
	}

	err = s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "restore_student_schedules_for_care_exit", func(ctx context.Context, stats *domain.OperationStats) error {
			q, runErr := s.store.RestoreStudentSchedulesForCareExit(ctx, studentIDs)
			stats.Add(q)
			rows = q.Rows
			return runErr
		})
	})
	return rows, err
}

func (s *Service) CreateArrivalSchedule(ctx context.Context, v careplan.ArrivalSchedule) (result careplan.ArrivalSchedule, err error) {
	if !validateArrivalSchedule(v) {
		return careplan.ArrivalSchedule{}, scheduleInvalid()
	}

	err = s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "create_arrival_schedule", func(ctx context.Context, stats *domain.OperationStats) error {
			var q domain.OperationStats
			result, q, err = s.store.CreateArrivalSchedule(ctx, v)
			stats.Add(q)
			return err
		})
	})
	return result, err
}

func (s *Service) UpdateArrivalSchedule(ctx context.Context, v careplan.ArrivalSchedule) error {
	if v.ID <= 0 || !validateArrivalSchedule(v) {
		return scheduleInvalid()
	}

	return s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "update_arrival_schedule", func(ctx context.Context, stats *domain.OperationStats) error {
			q, runErr := s.store.UpdateArrivalSchedule(ctx, v)
			stats.Add(q)
			return runErr
		})
	})
}

func (s *Service) UpsertArrivalSchedule(ctx context.Context, v careplan.ArrivalSchedule) (result careplan.ArrivalSchedule, err error) {
	if !validateArrivalSchedule(v) {
		return careplan.ArrivalSchedule{}, scheduleInvalid()
	}

	err = s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "upsert_arrival_schedule", func(ctx context.Context, stats *domain.OperationStats) error {
			var q domain.OperationStats
			result, q, err = s.store.UpsertArrivalSchedule(ctx, v)
			stats.Add(q)
			return err
		})
	})
	return result, err
}

func (s *Service) DeleteArrivalSchedule(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}

	return s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "delete_arrival_schedule", func(ctx context.Context, stats *domain.OperationStats) error {
			q, err := s.store.DeleteArrivalSchedule(ctx, id)
			stats.Add(q)
			return err
		})
	})
}

func (s *Service) DeleteArrivalSchedulesByStudent(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}

	return s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "delete_arrival_schedules_by_student", func(ctx context.Context, stats *domain.OperationStats) error {
			q, err := s.store.DeleteArrivalSchedulesByStudent(ctx, id)
			stats.Add(q)
			return err
		})
	})
}

func (s *Service) CreateArrivalException(ctx context.Context, v careplan.ArrivalException) (result careplan.ArrivalException, err error) {
	if !validateArrivalException(v) {
		return careplan.ArrivalException{}, scheduleInvalid()
	}

	err = s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "create_arrival_exception", func(ctx context.Context, stats *domain.OperationStats) error {
			var q domain.OperationStats
			result, q, err = s.store.CreateArrivalException(ctx, v)
			stats.Add(q)
			return err
		})
	})
	return result, err
}

func (s *Service) UpdateArrivalException(ctx context.Context, v careplan.ArrivalException) error {
	if v.ID <= 0 || !validateArrivalException(v) {
		return scheduleInvalid()
	}

	return s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "update_arrival_exception", func(ctx context.Context, stats *domain.OperationStats) error {
			q, err := s.store.UpdateArrivalException(ctx, v)
			stats.Add(q)
			return err
		})
	})
}

func (s *Service) DeleteArrivalException(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}

	return s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "delete_arrival_exception", func(ctx context.Context, stats *domain.OperationStats) error {
			q, err := s.store.DeleteArrivalException(ctx, id)
			stats.Add(q)
			return err
		})
	})
}

func (s *Service) DeleteArrivalExceptionsByStudent(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}

	return s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "delete_arrival_exceptions_by_student", func(ctx context.Context, stats *domain.OperationStats) error {
			q, err := s.store.DeleteArrivalExceptionsByStudent(ctx, id)
			stats.Add(q)
			return err
		})
	})
}

func (s *Service) DeleteArrivalExceptionsBefore(ctx context.Context, d careplan.Date) (rows int64, err error) {
	if d.IsZero() {
		return 0, scheduleInvalid()
	}

	err = s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "delete_arrival_exceptions_before", func(ctx context.Context, stats *domain.OperationStats) error {
			q, runErr := s.store.DeleteArrivalExceptionsBefore(ctx, d)
			stats.Add(q)
			rows = q.Rows
			return runErr
		})
	})
	return rows, err
}

func (s *Service) CreateArrivalNote(ctx context.Context, v careplan.ArrivalNote) (result careplan.ArrivalNote, err error) {
	if !validateNote(v.StudentID, v.NoteDate, v.Content, v.CreatedBy) {
		return careplan.ArrivalNote{}, scheduleInvalid()
	}

	err = s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "create_arrival_note", func(ctx context.Context, stats *domain.OperationStats) error {
			var q domain.OperationStats
			result, q, err = s.store.CreateArrivalNote(ctx, v)
			stats.Add(q)
			return err
		})
	})
	return result, err
}

func (s *Service) UpdateArrivalNote(ctx context.Context, v careplan.ArrivalNote) error {
	if v.ID <= 0 || !validateNote(v.StudentID, v.NoteDate, v.Content, v.CreatedBy) {
		return scheduleInvalid()
	}

	return s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "update_arrival_note", func(ctx context.Context, stats *domain.OperationStats) error {
			q, err := s.store.UpdateArrivalNote(ctx, v)
			stats.Add(q)
			return err
		})
	})
}

func (s *Service) DeleteArrivalNote(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}

	return s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "delete_arrival_note", func(ctx context.Context, stats *domain.OperationStats) error {
			q, err := s.store.DeleteArrivalNote(ctx, id)
			stats.Add(q)
			return err
		})
	})
}

func (s *Service) DeleteArrivalNotesByStudent(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}

	return s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "delete_arrival_notes_by_student", func(ctx context.Context, stats *domain.OperationStats) error {
			q, err := s.store.DeleteArrivalNotesByStudent(ctx, id)
			stats.Add(q)
			return err
		})
	})
}

func (s *Service) DeleteArrivalNotesBefore(ctx context.Context, d careplan.Date) (rows int64, err error) {
	if d.IsZero() {
		return 0, scheduleInvalid()
	}

	err = s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "delete_arrival_notes_before", func(ctx context.Context, stats *domain.OperationStats) error {
			q, runErr := s.store.DeleteArrivalNotesBefore(ctx, d)
			stats.Add(q)
			rows = q.Rows
			return runErr
		})
	})
	return rows, err
}

func (s *Service) CreatePickupSchedule(ctx context.Context, v careplan.PickupSchedule) (result careplan.PickupSchedule, err error) {
	if !validatePickupSchedule(v) {
		return careplan.PickupSchedule{}, scheduleInvalid()
	}

	err = s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "create_pickup_schedule", func(ctx context.Context, stats *domain.OperationStats) error {
			var q domain.OperationStats
			result, q, err = s.store.CreatePickupSchedule(ctx, v)
			stats.Add(q)
			return err
		})
	})
	return result, err
}

func (s *Service) UpdatePickupSchedule(ctx context.Context, v careplan.PickupSchedule) error {
	if v.ID <= 0 || !validatePickupSchedule(v) {
		return scheduleInvalid()
	}

	return s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "update_pickup_schedule", func(ctx context.Context, stats *domain.OperationStats) error {
			q, err := s.store.UpdatePickupSchedule(ctx, v)
			stats.Add(q)
			return err
		})
	})
}

func (s *Service) UpsertPickupSchedule(ctx context.Context, v careplan.PickupSchedule) (result careplan.PickupSchedule, err error) {
	if !validatePickupSchedule(v) {
		return careplan.PickupSchedule{}, scheduleInvalid()
	}

	err = s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "upsert_pickup_schedule", func(ctx context.Context, stats *domain.OperationStats) error {
			var q domain.OperationStats
			result, q, err = s.store.UpsertPickupSchedule(ctx, v)
			stats.Add(q)
			return err
		})
	})
	return result, err
}

func (s *Service) DeletePickupSchedule(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}

	return s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "delete_pickup_schedule", func(ctx context.Context, stats *domain.OperationStats) error {
			q, err := s.store.DeletePickupSchedule(ctx, id)
			stats.Add(q)
			return err
		})
	})
}

func (s *Service) DeletePickupSchedulesByStudent(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}

	return s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "delete_pickup_schedules_by_student", func(ctx context.Context, stats *domain.OperationStats) error {
			q, err := s.store.DeletePickupSchedulesByStudent(ctx, id)
			stats.Add(q)
			return err
		})
	})
}

func (s *Service) CreatePickupException(ctx context.Context, v careplan.PickupException) (result careplan.PickupException, err error) {
	if !validatePickupException(v) {
		return careplan.PickupException{}, scheduleInvalid()
	}

	err = s.withinTenant(ctx, func(tx context.Context) error {
		if err = v.Validate(); err != nil {
			return err
		}
		return s.run(tx, "create_pickup_exception", func(ctx context.Context, stats *domain.OperationStats) error {
			var q domain.OperationStats
			result, q, err = s.store.CreatePickupException(ctx, v)
			stats.Add(q)
			return err
		})
	})
	return result, err
}

func (s *Service) UpdatePickupException(ctx context.Context, v careplan.PickupException) error {
	if v.ID <= 0 || !validatePickupException(v) {
		return scheduleInvalid()
	}

	return s.withinTenant(ctx, func(tx context.Context) error {
		if err := v.Validate(); err != nil {
			return err
		}
		return s.run(tx, "update_pickup_exception", func(ctx context.Context, stats *domain.OperationStats) error {
			q, err := s.store.UpdatePickupException(ctx, v)
			stats.Add(q)
			return err
		})
	})
}

func (s *Service) DeletePickupException(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}

	return s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "delete_pickup_exception", func(ctx context.Context, stats *domain.OperationStats) error {
			q, err := s.store.DeletePickupException(ctx, id)
			stats.Add(q)
			return err
		})
	})
}

func (s *Service) DeletePickupExceptionsByStudent(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}

	return s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "delete_pickup_exceptions_by_student", func(ctx context.Context, stats *domain.OperationStats) error {
			q, err := s.store.DeletePickupExceptionsByStudent(ctx, id)
			stats.Add(q)
			return err
		})
	})
}

func (s *Service) DeletePickupExceptionsBefore(ctx context.Context, d careplan.Date) (rows int64, err error) {
	if d.IsZero() {
		return 0, scheduleInvalid()
	}

	err = s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "delete_pickup_exceptions_before", func(ctx context.Context, stats *domain.OperationStats) error {
			q, runErr := s.store.DeletePickupExceptionsBefore(ctx, d)
			stats.Add(q)
			rows = q.Rows
			return runErr
		})
	})
	return rows, err
}

func (s *Service) CreatePickupNote(ctx context.Context, v careplan.PickupNote) (result careplan.PickupNote, err error) {
	if !validatePickupNote(v) {
		return careplan.PickupNote{}, scheduleInvalid()
	}

	err = s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "create_pickup_note", func(ctx context.Context, stats *domain.OperationStats) error {
			var q domain.OperationStats
			result, q, err = s.store.CreatePickupNote(ctx, v)
			stats.Add(q)
			return err
		})
	})
	return result, err
}

func (s *Service) UpdatePickupNote(ctx context.Context, v careplan.PickupNote) error {
	if v.ID <= 0 || !validatePickupNote(v) {
		return scheduleInvalid()
	}

	return s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "update_pickup_note", func(ctx context.Context, stats *domain.OperationStats) error {
			q, err := s.store.UpdatePickupNote(ctx, v)
			stats.Add(q)
			return err
		})
	})
}

func (s *Service) DeletePickupNote(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}

	return s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "delete_pickup_note", func(ctx context.Context, stats *domain.OperationStats) error {
			q, err := s.store.DeletePickupNote(ctx, id)
			stats.Add(q)
			return err
		})
	})
}

func (s *Service) ReplaceWeekdayPickupNotes(ctx context.Context, studentID, createdBy int64, notes map[int]string) error {
	if studentID <= 0 || createdBy <= 0 || !validWeekdayPickupNotes(notes) {
		return scheduleInvalid()
	}

	return s.withinTenant(ctx, func(tx context.Context) error {
		// Existing note rows do not provide a lock on a child's first note.
		// Lock the stable owner first so concurrent replacements cannot both
		// attempt the same weekday insert.
		if err := s.runtime.LockStudent(tx, studentID); err != nil {
			return err
		}
		return s.run(tx, "replace_weekday_pickup_notes", func(ctx context.Context, stats *domain.OperationStats) error {
			stored, err := s.lockWeekdayPickupNotes(ctx, studentID, stats)
			if err != nil {
				return err
			}
			if err := s.deleteRemovedWeekdayPickupNotes(ctx, stored, notes, stats); err != nil {
				return err
			}
			return s.saveWeekdayPickupNotes(ctx, studentID, createdBy, stored, notes, stats)
		})
	})
}

func (s *Service) DeletePickupNotesByStudent(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}

	return s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "delete_pickup_notes_by_student", func(ctx context.Context, stats *domain.OperationStats) error {
			q, err := s.store.DeletePickupNotesByStudent(ctx, id)
			stats.Add(q)
			return err
		})
	})
}

func (s *Service) DeletePickupNotesBefore(ctx context.Context, d careplan.Date) (rows int64, err error) {
	if d.IsZero() {
		return 0, scheduleInvalid()
	}

	err = s.withinTenant(ctx, func(tx context.Context) error {
		return s.run(tx, "delete_pickup_notes_before", func(ctx context.Context, stats *domain.OperationStats) error {
			q, runErr := s.store.DeletePickupNotesBefore(ctx, d)
			stats.Add(q)
			rows = q.Rows
			return runErr
		})
	})
	return rows, err
}
