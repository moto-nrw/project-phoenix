package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) FindStudentEnrollment(ctx context.Context, id int64) (result timetable.StudentEnrollment, err error) {
	if id <= 0 {
		return timetable.StudentEnrollment{}, s.reject("find_student_enrollment", timetable.ErrInvalidStudentEnrollmentQuery)
	}

	err = s.run(ctx, "find_student_enrollment", func(ctx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindStudentEnrollment(ctx, id)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return timetable.ErrStudentEnrollmentNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ListStudentEnrollments(ctx context.Context, filter timetable.StudentEnrollmentFilter) (result []timetable.StudentEnrollment, err error) {
	if hasInvalidID(filter.StudentIDs) || hasInvalidID(filter.ActivityGroupIDs) || filter.Limit < 0 || filter.Offset < 0 ||
		(filter.ActiveOn != nil && !timetable.ValidDate(*filter.ActiveOn)) {
		return nil, s.reject("list_student_enrollments", timetable.ErrInvalidStudentEnrollmentQuery)
	}

	err = s.run(ctx, "list_student_enrollments", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListStudentEnrollments(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) CreateStudentEnrollment(ctx context.Context, fields timetable.StudentEnrollmentInput) (result timetable.StudentEnrollment, err error) {
	if !validStudentEnrollment(fields) {
		return timetable.StudentEnrollment{}, s.reject("create_student_enrollment", timetable.ErrInvalidStudentEnrollment)
	}

	err = s.runWrite(ctx, "create_student_enrollment", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, createErr := s.store.CreateStudentEnrollment(txCtx, fields)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) UpdateStudentEnrollment(ctx context.Context, id int64, fields timetable.StudentEnrollmentInput) (result timetable.StudentEnrollment, err error) {
	if id <= 0 || !validStudentEnrollment(fields) {
		return timetable.StudentEnrollment{}, s.reject("update_student_enrollment", timetable.ErrInvalidStudentEnrollment)
	}

	err = s.runWrite(ctx, "update_student_enrollment", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, updateErr := s.store.UpdateStudentEnrollment(txCtx, id, fields)
		stats.Add(queryStats)
		if updateErr != nil {
			return updateErr
		}
		if !found {
			return timetable.ErrStudentEnrollmentNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) DeleteStudentEnrollment(ctx context.Context, id int64) error {
	if id <= 0 {
		return s.reject("delete_student_enrollment", timetable.ErrInvalidStudentEnrollment)
	}

	return s.runWrite(ctx, "delete_student_enrollment", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteStudentEnrollment(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

// BackfillStudentEnrollmentSource links the eligible enrollments to their
// request child through the provenance store.
func (s *Service) BackfillStudentEnrollmentSource(ctx context.Context, studentID, requestChildID int64, groupIDs []int64) (result int64, err error) {
	if studentID <= 0 || requestChildID <= 0 || hasInvalidID(groupIDs) {
		return 0, s.reject("backfill_student_enrollment_source", timetable.ErrInvalidStudentEnrollment)
	}
	err = s.runWrite(ctx, "backfill_student_enrollment_source", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, writeErr := s.provenance.BackfillStudentEnrollmentSource(txCtx, studentID, requestChildID, groupIDs)
		stats.Add(queryStats)
		result = rows
		return writeErr
	})
	return result, err
}

func (s *Service) DeleteStudentEnrollmentsBySource(ctx context.Context, studentID, requestChildID int64) (result int64, err error) {
	if studentID <= 0 || requestChildID <= 0 {
		return 0, s.reject("delete_student_enrollments_by_source", timetable.ErrInvalidStudentEnrollment)
	}

	err = s.runWrite(ctx, "delete_student_enrollments_by_source", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, writeErr := s.store.DeleteStudentEnrollmentsBySource(txCtx, studentID, requestChildID)
		stats.Add(queryStats)
		result = rows
		return writeErr
	})
	return result, err
}

func (s *Service) CapActiveStudentEnrollments(ctx context.Context, groupID int64, validUntil string) (result int64, err error) {
	if groupID <= 0 || !timetable.ValidDate(validUntil) {
		return 0, s.reject("cap_active_student_enrollments", timetable.ErrInvalidStudentEnrollment)
	}

	err = s.runWrite(ctx, "cap_active_student_enrollments", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, writeErr := s.store.CapActiveStudentEnrollments(txCtx, groupID, validUntil)
		stats.Add(queryStats)
		result = rows
		return writeErr
	})
	return result, err
}

func (s *Service) SetStudentEnrollmentValidUntil(ctx context.Context, id int64, validUntil string) error {
	if id <= 0 || !timetable.ValidDate(validUntil) {
		return s.reject("set_student_enrollment_valid_until", timetable.ErrInvalidStudentEnrollment)
	}

	return s.runWrite(ctx, "set_student_enrollment_valid_until", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := s.store.SetStudentEnrollmentValidUntil(txCtx, id, validUntil)
		stats.Add(queryStats)
		if err == nil && !found {
			return timetable.ErrStudentEnrollmentNotFound
		}
		return err
	})
}

// SetStudentEnrollmentsKeptWithoutBooking marks or unmarks hand rows of one
// student on the given weekdays (#5102); other weekdays keep their decision.
// Every id must name a hand row of that student: a booking-created row
// follows its booking and never carries the mark.
func (s *Service) SetStudentEnrollmentsKeptWithoutBooking(ctx context.Context, studentID int64, ids []int64, weekdays []int, through *string) error {
	if studentID <= 0 || len(weekdays) == 0 || (through != nil && !timetable.ValidDate(*through)) {
		return s.reject("set_student_enrollments_kept_without_booking", timetable.ErrInvalidStudentEnrollment)
	}
	return s.runWrite(ctx, "set_student_enrollments_kept_without_booking", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		return s.keepHandRows(txCtx, stats, studentID, ids, weekdays, through)
	})
}

// keepHandRows writes the merged decision onto each of the student's hand
// rows and refuses any other row.
func (s *Service) keepHandRows(ctx context.Context, stats *domain.OperationStats, studentID int64, ids []int64, weekdays []int, through *string) error {
	rows, queryStats, err := s.store.ListStudentEnrollments(ctx, timetable.StudentEnrollmentFilter{StudentIDs: []int64{studentID}})
	stats.Add(queryStats)
	if err != nil {
		return err
	}
	hand := make(map[int64]timetable.StudentEnrollment, len(rows))
	for _, row := range rows {
		if row.EnrollmentRequestChildID == nil {
			hand[row.ID] = row
		}
	}
	for _, id := range ids {
		row, found := hand[id]
		if !found {
			return timetable.ErrStudentEnrollmentNotFound
		}
		queryStats, err = s.store.SetStudentEnrollmentKeptWithoutBooking(ctx, id, mergeKeptWeekdays(row.KeptWithoutBooking, weekdays, through))
		stats.Add(queryStats)
		if err != nil {
			return err
		}
	}
	return nil
}

// mergeKeptWeekdays sets the weekdays to the later of their decision and
// through, or clears them when through is nil; other weekdays stay.
func mergeKeptWeekdays(current map[int]string, weekdays []int, through *string) map[int]string {
	result := make(map[int]string, len(current)+len(weekdays))
	for weekday, date := range current {
		result[weekday] = date
	}
	for _, weekday := range weekdays {
		switch {
		case through == nil:
			delete(result, weekday)
		case result[weekday] < *through:
			result[weekday] = *through
		}
	}
	return result
}

func (s *Service) CloseOpenStudentEnrollments(ctx context.Context, groupID int64, periodID *int64, validUntil string) error {
	if groupID <= 0 || (periodID != nil && *periodID <= 0) || !timetable.ValidDate(validUntil) {
		return s.reject("close_open_student_enrollments", timetable.ErrInvalidStudentEnrollment)
	}

	return s.runWrite(ctx, "close_open_student_enrollments", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.CloseOpenStudentEnrollments(txCtx, groupID, periodID, validUntil)
		stats.Add(queryStats)
		return err
	})
}
