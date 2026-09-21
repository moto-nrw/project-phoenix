package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

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

// Pickup exception writes validate here because guardian and approval writes
// reach the store through this capability without passing the effective-time core.
func (s *Service) CreatePickupException(ctx context.Context, v careplan.PickupException) (result careplan.PickupException, err error) {
	if err = v.Validate(); err != nil {
		return result, err
	}
	err = s.run("create_pickup_exception", func(stats *domain.OperationStats) error {
		var q domain.OperationStats
		result, q, err = s.store.CreatePickupException(ctx, v)
		stats.Add(q)
		return err
	})
	return
}
func (s *Service) UpdatePickupException(ctx context.Context, v careplan.PickupException) error {
	if err := v.Validate(); err != nil {
		return err
	}
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
func (s *Service) ReplaceWeekdayPickupNotes(ctx context.Context, studentID, createdBy int64, wanted map[int]string) error {
	return s.run("replace_weekday_pickup_notes", func(stats *domain.OperationStats) error {
		stored, err := s.lockWeekdayPickupNotes(ctx, studentID, stats)
		if err != nil {
			return err
		}
		if err := s.deleteRemovedWeekdayPickupNotes(ctx, stored, wanted, stats); err != nil {
			return err
		}
		return s.saveWeekdayPickupNotes(ctx, studentID, createdBy, stored, wanted, stats)
	})
}

func (s *Service) lockWeekdayPickupNotes(ctx context.Context, studentID int64, stats *domain.OperationStats) (map[int]careplan.PickupNote, error) {
	stored, q, err := s.store.ListPickupNotes(ctx, careplan.StudentScheduleFilter{
		StudentIDs: []int64{studentID}, LockForUpdate: true,
	})
	stats.Add(q)
	if err != nil {
		return nil, err
	}
	byWeekday := make(map[int]careplan.PickupNote, len(stored))
	for _, note := range stored {
		if note.Weekday != 0 {
			byWeekday[note.Weekday] = note
		}
	}
	return byWeekday, nil
}

func (s *Service) deleteRemovedWeekdayPickupNotes(ctx context.Context, stored map[int]careplan.PickupNote, wanted map[int]string, stats *domain.OperationStats) error {
	for weekday, note := range stored {
		if _, keep := wanted[weekday]; !keep {
			q, err := s.store.DeletePickupNote(ctx, note.ID)
			stats.Add(q)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) saveWeekdayPickupNotes(ctx context.Context, studentID, createdBy int64, stored map[int]careplan.PickupNote, wanted map[int]string, stats *domain.OperationStats) error {
	for weekday := 1; weekday <= 5; weekday++ {
		content, wantedForWeekday := wanted[weekday]
		if !wantedForWeekday {
			continue
		}
		storedNote, exists := stored[weekday]
		if !exists {
			_, q, err := s.store.CreatePickupNote(ctx, careplan.PickupNote{
				StudentID: studentID, Weekday: weekday, Content: content, CreatedBy: createdBy,
			})
			stats.Add(q)
			if err != nil {
				return err
			}
			continue
		}
		if storedNote.Content == content {
			continue
		}
		storedNote.Content = content
		q, err := s.store.UpdatePickupNote(ctx, storedNote)
		stats.Add(q)
		if err != nil {
			return err
		}
	}
	return nil
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
