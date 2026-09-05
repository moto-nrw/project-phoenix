package application

import (
	"context"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/ports"
)

func (s *Service) FindInstanceStudent(ctx context.Context, id int64) (result domain.InstanceStudent, err error) {
	err = s.run("find_instance_student", func(stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindInstanceStudent(ctx, id)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrInstanceStudentNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) UpdateAttendanceFromCheckin(ctx context.Context, instanceID, studentID int64, checkedInAt time.Time) (updated bool, err error) {
	err = s.runWrite(ctx, "update_attendance_from_checkin", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, updateErr := s.store.UpdateAttendanceFromCheckin(txCtx, instanceID, studentID, checkedInAt)
		stats.Add(queryStats)
		updated = value
		return updateErr
	})
	return updated, err
}

func (s *Service) UpdateAttendanceFromCheckinBatch(ctx context.Context, keys []domain.InstanceStudentKey, checkedInAt time.Time) error {
	return s.runWrite(ctx, "update_attendance_from_checkin_batch", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.UpdateAttendanceFromCheckinBatch(txCtx, keys, checkedInAt)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) UpdateAttendanceCheckout(ctx context.Context, instanceID, studentID int64, checkedOutAt time.Time) error {
	return s.runWrite(ctx, "update_attendance_checkout", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.UpdateAttendanceCheckout(txCtx, instanceID, studentID, checkedOutAt)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) UpdateAttendanceCheckoutBatch(ctx context.Context, keys []domain.InstanceStudentKey, checkedOutAt time.Time) error {
	return s.runWrite(ctx, "update_attendance_checkout_batch", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.UpdateAttendanceCheckoutBatch(txCtx, keys, checkedOutAt)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) CreateUnplannedPresentIfAbsent(ctx context.Context, instanceID, studentID int64, checkedInAt time.Time) (result domain.InstanceStudent, err error) {
	err = s.runWrite(ctx, "create_unplanned_present_if_absent", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, createErr := s.store.CreateUnplannedPresentIfAbsent(txCtx, instanceID, studentID, checkedInAt)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) ReconcileAttendanceInterval(ctx context.Context, instanceID, studentID int64, previousCheckIn time.Time, previousCheckOut *time.Time, updatedCheckIn time.Time, updatedCheckOut *time.Time) (updated bool, err error) {
	err = s.runWrite(ctx, "reconcile_attendance_interval", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, updateErr := s.store.ReconcileAttendanceInterval(txCtx, instanceID, studentID, previousCheckIn, previousCheckOut, updatedCheckIn, updatedCheckOut)
		stats.Add(queryStats)
		updated = value
		return updateErr
	})
	return updated, err
}

func (s *Service) ListInstanceStudents(ctx context.Context, filter domain.InstanceStudentFilter) (result []domain.InstanceStudent, err error) {
	err = s.run("list_instance_students", func(stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListInstanceStudents(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) CountNonAbsentInstanceStudents(ctx context.Context, instanceIDs []int64) (result map[int64]int, err error) {
	err = s.run("count_non_absent_instance_students", func(stats *domain.OperationStats) error {
		values, queryStats, countErr := s.store.CountNonAbsentInstanceStudents(ctx, instanceIDs)
		stats.Add(queryStats)
		result = values
		return countErr
	})
	return result, err
}

func (s *Service) ListParallelStudentPresence(ctx context.Context, excludeInstanceID int64, date string, studentIDs []int64) (result []domain.ParallelPresence, err error) {
	err = s.run("list_parallel_student_presence", func(stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListParallelStudentPresence(ctx, excludeInstanceID, date, studentIDs)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) CreateInstanceStudent(ctx context.Context, fields domain.InstanceStudentFields) (result domain.InstanceStudent, err error) {
	err = s.runWrite(ctx, "create_instance_student", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, createErr := s.store.CreateInstanceStudent(txCtx, fields)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) UpdateInstanceStudent(ctx context.Context, id int64, fields domain.InstanceStudentFields) (result domain.InstanceStudent, err error) {
	err = s.runWrite(ctx, "update_instance_student", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, updateErr := s.store.UpdateInstanceStudent(txCtx, id, fields)
		stats.Add(queryStats)
		if updateErr != nil {
			return updateErr
		}
		if !found {
			return domain.ErrInstanceStudentNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) DeleteInstanceStudent(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_instance_student", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteInstanceStudent(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) DeleteInstanceStudentsByInstance(ctx context.Context, instanceID int64) error {
	return s.runWrite(ctx, "delete_instance_students_by_instance", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteInstanceStudentsByInstance(txCtx, instanceID)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) ListStudentInstanceRefsBefore(ctx context.Context, cutoff string) (result []domain.StudentInstanceRef, err error) {
	err = s.run("list_student_instance_refs_before", func(stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListStudentInstanceRefsBefore(ctx, cutoff)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) ListScheduledInstancesForStudent(ctx context.Context, studentID int64, from, to string) (result []domain.ScheduledInstanceRow, err error) {
	err = s.run("list_scheduled_instances_for_student", func(stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListScheduledInstancesForStudent(ctx, studentID, from, to)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) HasPlannedStudentSlots(ctx context.Context, from, to string) (result bool, err error) {
	err = s.run("has_planned_student_slots", func(stats *domain.OperationStats) error {
		value, queryStats, existsErr := s.store.HasPlannedStudentSlots(ctx, from, to)
		stats.Add(queryStats)
		result = value
		return existsErr
	})
	return result, err
}

func (s *Service) ListPlannedStudentIDs(ctx context.Context, studentIDs []int64, date string) (result []int64, err error) {
	err = s.run("list_planned_student_ids", func(stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListPlannedStudentIDs(ctx, studentIDs, date)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) ApplyStatusDay(ctx context.Context, studentID int64, date string, statusDayID int64, substatus string) (rows int, err error) {
	err = s.runWrite(ctx, "apply_status_day", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		carePlan, bindErr := s.carePlanDirectory()
		if bindErr != nil {
			return bindErr
		}
		if lockErr := s.locks.LockStudentAndExceptionDay(txCtx, studentID, date); lockErr != nil {
			return lockErr
		}
		incoming, findErr := carePlan.FindStudentStatusDay(txCtx, statusDayID, true)
		if findErr != nil {
			return findErr
		}
		latest, latestErr := latestActiveStatusDay(txCtx, carePlan, studentID, date)
		if latestErr != nil {
			return latestErr
		}
		if incoming == nil || latest == nil || incoming.StudentID != studentID || incoming.Date != date || latest.ID != statusDayID {
			return nil
		}
		updated, queryStats, updateErr := s.store.ApplyStatusDay(txCtx, studentID, date, statusDayID, substatus, time.Now().UTC())
		stats.Add(queryStats)
		rows = int(updated)
		return updateErr
	})
	return rows, err
}

func (s *Service) ReleaseStatusDay(ctx context.Context, statusDayID int64) (rows int, err error) {
	err = s.runWrite(ctx, "release_status_day", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		carePlan, bindErr := s.carePlanDirectory()
		if bindErr != nil {
			return bindErr
		}
		updated, releaseErr := s.releaseStatusDay(txCtx, carePlan, statusDayID, stats)
		rows = int(updated)
		return releaseErr
	})
	return rows, err
}

func (s *Service) releaseStatusDay(ctx context.Context, carePlan ports.CarePlanDirectory, statusDayID int64, stats *domain.OperationStats) (int64, error) {
	released, err := carePlan.FindStudentStatusDay(ctx, statusDayID, false)
	if err != nil || released == nil {
		return 0, err
	}
	if err = s.locks.LockStudentAndExceptionDay(ctx, released.StudentID, released.Date); err != nil {
		return 0, err
	}
	released, err = carePlan.FindStudentStatusDay(ctx, statusDayID, false)
	if err != nil || released == nil {
		return 0, err
	}
	replacement, err := latestActiveStatusDay(ctx, carePlan, released.StudentID, released.Date)
	if err != nil {
		return 0, err
	}
	rows, queryStats, err := s.store.ReleaseStatusDay(ctx, statusDayID, released.StudentID, replacement, time.Now().UTC())
	stats.Add(queryStats)
	if err != nil {
		return 0, err
	}
	return rows, s.replayPartialAbsences(ctx, carePlan, released.StudentID, released.Date, stats)
}

func (s *Service) replayPartialAbsences(ctx context.Context, carePlan ports.CarePlanDirectory, studentID int64, date string, stats *domain.OperationStats) error {
	exceptions, err := carePlan.ListPickupExceptions(ctx, domain.PickupExceptionFilter{StudentIDs: []int64{studentID}, Date: date})
	if err != nil {
		return err
	}
	for _, exception := range exceptions {
		queryRows, queryStats, applyErr := s.store.ApplyPartialAbsence(ctx, exception, false, time.Now().UTC())
		_ = queryRows
		stats.Add(queryStats)
		if applyErr != nil {
			return applyErr
		}
	}
	return nil
}

func (s *Service) ApplyActiveStatusDaysForInstance(ctx context.Context, instanceID int64, date string) (rows int, err error) {
	err = s.runWrite(ctx, "apply_active_status_days_for_instance", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		carePlan, bindErr := s.carePlanDirectory()
		if bindErr != nil {
			return bindErr
		}
		attendance, queryStats, listErr := s.store.ListInstanceStudents(txCtx, domain.InstanceStudentFilter{InstanceIDs: []int64{instanceID}})
		stats.Add(queryStats)
		if listErr != nil {
			return listErr
		}
		for _, studentID := range uniqueStudentIDs(attendance) {
			if lockErr := s.locks.LockStudentAndExceptionDay(txCtx, studentID, date); lockErr != nil {
				return lockErr
			}
		}
		statuses, listStatusErr := carePlan.ListStudentStatusDays(txCtx, domain.StudentStatusDayFilter{Date: date, ActiveOnly: true, LatestOnly: true})
		if listStatusErr != nil {
			return listStatusErr
		}
		updated, updateStats, updateErr := s.store.ApplyActiveStatusDaysForInstance(txCtx, instanceID, statuses, time.Now().UTC())
		stats.Add(updateStats)
		rows = int(updated)
		return updateErr
	})
	return rows, err
}

func uniqueStudentIDs(values []domain.InstanceStudent) []int64 {
	result := make([]int64, 0, len(values))
	for _, value := range values {
		result = append(result, value.StudentID)
	}
	slices.Sort(result)
	return slices.Compact(result)
}

func (s *Service) ApplyPartialAbsence(ctx context.Context, pickupExceptionID int64) (rows int, err error) {
	err = s.runWrite(ctx, "apply_partial_absence", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		carePlan, bindErr := s.carePlanDirectory()
		if bindErr != nil {
			return bindErr
		}
		exception, findErr := carePlan.FindPickupException(txCtx, pickupExceptionID)
		if findErr != nil || exception == nil || exception.ExcusedFrom == nil {
			return findErr
		}
		updated, queryStats, updateErr := s.store.ApplyPartialAbsence(txCtx, *exception, false, time.Now().UTC())
		stats.Add(queryStats)
		rows = int(updated)
		return updateErr
	})
	return rows, err
}

func (s *Service) ReleasePartialAbsence(ctx context.Context, pickupExceptionID int64) (rows int, err error) {
	err = s.runWrite(ctx, "release_partial_absence", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		carePlan, bindErr := s.carePlanDirectory()
		if bindErr != nil {
			return bindErr
		}
		exception, findErr := carePlan.FindPickupException(txCtx, pickupExceptionID)
		if findErr != nil || exception == nil {
			return findErr
		}
		if lockErr := s.locks.LockStudentAndExceptionDay(txCtx, exception.StudentID, exception.ExceptionDate); lockErr != nil {
			return lockErr
		}
		exception, findErr = carePlan.FindPickupException(txCtx, pickupExceptionID)
		if findErr != nil || exception == nil {
			return findErr
		}
		replacement, latestErr := latestActiveStatusDay(txCtx, carePlan, exception.StudentID, exception.ExceptionDate)
		if latestErr != nil {
			return latestErr
		}
		updated, queryStats, updateErr := s.store.ReleasePartialAbsence(txCtx, pickupExceptionID, exception.StudentID, replacement, time.Now().UTC())
		stats.Add(queryStats)
		rows = int(updated)
		return updateErr
	})
	return rows, err
}

func (s *Service) ApplyActivePartialAbsencesForInstance(ctx context.Context, instanceID int64, date string) (rows int, err error) {
	err = s.runWrite(ctx, "apply_active_partial_absences_for_instance", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		carePlan, bindErr := s.carePlanDirectory()
		if bindErr != nil {
			return bindErr
		}
		exceptions, listErr := carePlan.ListPickupExceptions(txCtx, domain.PickupExceptionFilter{Date: date})
		if listErr != nil {
			return listErr
		}
		for _, studentID := range uniqueExceptionStudentIDs(exceptions) {
			if lockErr := s.locks.LockExceptionDay(txCtx, studentID, date); lockErr != nil {
				return lockErr
			}
		}
		updated, queryStats, updateErr := s.store.ApplyActivePartialAbsencesForInstance(txCtx, instanceID, date, exceptions, time.Now().UTC())
		stats.Add(queryStats)
		rows = int(updated)
		return updateErr
	})
	return rows, err
}

func uniqueExceptionStudentIDs(values []domain.PickupException) []int64 {
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value.ExcusedFrom != nil {
			result = append(result, value.StudentID)
		}
	}
	slices.Sort(result)
	return slices.Compact(result)
}

func (s *Service) ListPartialAbsenceBlocks(ctx context.Context, studentID int64, date string, from time.Time) (result []domain.PartialAbsenceBlock, err error) {
	err = s.run("list_partial_absence_blocks", func(stats *domain.OperationStats) error {
		carePlan, bindErr := s.carePlanDirectory()
		if bindErr != nil {
			return bindErr
		}
		students, studentStats, listStudentsErr := s.students.ListEnrolledStudents(ctx)
		stats.Add(studentStats)
		if listStudentsErr != nil {
			return listStudentsErr
		}
		exceptions, listErr := carePlan.ListPickupExceptions(ctx, domain.PickupExceptionFilter{StudentIDs: []int64{studentID}, Date: date})
		if listErr != nil {
			return listErr
		}
		values, queryStats, queryErr := s.store.ListPartialAbsenceBlocks(ctx, studentID, date, from, enrolledStudent(students, studentID), autoExceptionIDs(exceptions))
		stats.Add(queryStats)
		result = values
		return queryErr
	})
	return result, err
}

func enrolledStudent(values []domain.TargetStudent, studentID int64) bool {
	for _, value := range values {
		if value.ID == studentID {
			return true
		}
	}
	return false
}

func autoExceptionIDs(values []domain.PickupException) []int64 {
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value.ExcusedAuto {
			result = append(result, value.ID)
		}
	}
	return result
}

func latestActiveStatusDay(ctx context.Context, carePlan ports.CarePlanDirectory, studentID int64, date string) (*domain.StudentStatusDay, error) {
	values, err := carePlan.ListStudentStatusDays(ctx, domain.StudentStatusDayFilter{
		StudentIDs: []int64{studentID}, Date: date, ActiveOnly: true, LatestOnly: true,
	})
	if err != nil || len(values) == 0 {
		return nil, err
	}
	return &values[0], nil
}

func (s *Service) UpdateAttendanceFields(ctx context.Context, id int64, patch domain.AttendanceFieldPatch) error {
	return s.runWrite(ctx, "update_attendance_fields", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.UpdateAttendanceFields(txCtx, id, patch)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) BulkUpdateStatus(ctx context.Context, instanceID int64, fromStatus, toStatus string, excludedStudentIDs []int64) (rows int, err error) {
	err = s.runWrite(ctx, "bulk_update_attendance_status", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		updated, queryStats, updateErr := s.store.BulkUpdateStatus(txCtx, instanceID, fromStatus, toStatus, excludedStudentIDs)
		stats.Add(queryStats)
		rows = int(updated)
		return updateErr
	})
	return rows, err
}

func (s *Service) MarkNotScheduled(ctx context.Context, refs []domain.StudentInstanceRef) error {
	return s.runWrite(ctx, "mark_attendance_not_scheduled", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.MarkNotScheduled(txCtx, refs)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) MarkExpectedAbsentByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, updatedAt time.Time, exclusions []domain.StudentInstanceRef) error {
	return s.runWrite(ctx, "mark_expected_absent_by_active_groups", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.MarkExpectedAbsentByActiveGroupIDs(txCtx, activeGroupIDs, updatedAt, exclusions)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) CloseOpenCheckoutsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, checkedOutAt time.Time) (rows int, err error) {
	err = s.runWrite(ctx, "close_open_checkouts_by_active_groups", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		updated, queryStats, updateErr := s.store.CloseOpenCheckoutsByActiveGroupIDs(txCtx, activeGroupIDs, checkedOutAt)
		stats.Add(queryStats)
		rows = int(updated)
		return updateErr
	})
	return rows, err
}
