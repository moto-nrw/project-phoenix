package postgres

import (
	"context"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

func (s *Store) ApplyStatusDay(ctx context.Context, studentID int64, date string, statusDayID int64, substatus string, updatedAt time.Time) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := db.NewUpdate().TableExpr(`schedule.instance_students AS "attendance"`).
		Set(`status = 'absent'`).Set(`substatus = ?`, substatus).Set(`student_status_day_id = ?`, statusDayID).
		Set(`updated_at = ?`, updatedAt).Where(`"attendance".tenant_id = ?`, tenantID).
		Where(`"attendance".student_id = ?`, studentID).Where(`NOT "attendance".not_scheduled`).
		Where(`("attendance".status = 'expected' OR "attendance".student_status_day_id IS NOT NULL)`).
		Where(`EXISTS (SELECT 1 FROM schedule.activity_instances AS "instance" WHERE "instance".id = "attendance".instance_id AND "instance".tenant_id = "attendance".tenant_id AND "instance".date = ?::date AND "instance".status <> 'cancelled')`, date)
	return runInstanceStudentUpdate(ctx, query, "apply student status day to slots")
}

func (s *Store) ReleaseStatusDay(ctx context.Context, statusDayID, studentID int64, replacement *domain.StudentStatusDay, updatedAt time.Time) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	replacementID, replacementStatus := statusDayReplacement(replacement)
	query := db.NewUpdate().TableExpr(`schedule.instance_students AS "attendance"`).
		Set(`status = CASE WHEN ?::bigint IS NOT NULL THEN 'absent' WHEN EXISTS (SELECT 1 FROM schedule.activity_instances AS "instance" WHERE "instance".id = "attendance".instance_id AND "instance".tenant_id = "attendance".tenant_id AND "instance".status = 'completed') THEN 'absent' ELSE 'expected' END`, replacementID).
		Set(`substatus = ?`, statusDaySubstatus(replacementStatus)).Set(`student_status_day_id = ?`, replacementID).
		Set(`updated_at = ?`, updatedAt).Where(`"attendance".tenant_id = ?`, tenantID).
		Where(`"attendance".student_status_day_id = ?`, statusDayID).Where(`"attendance".student_id = ?`, studentID)
	return runInstanceStudentUpdate(ctx, query, "release student status day from slots")
}

func (s *Store) ApplyActiveStatusDaysForInstance(ctx context.Context, instanceID int64, statuses []domain.StudentStatusDay, updatedAt time.Time) (int64, domain.OperationStats, error) {
	if len(statuses) == 0 {
		return 0, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	studentIDs, statusDayIDs, substatuses := statusDayArrays(statuses)
	query := db.NewUpdate().TableExpr(`schedule.instance_students AS "attendance"`).Set(`status = 'absent'`).
		Set(`student_status_day_id = (?::bigint[])[array_position(?::bigint[], "attendance".student_id)]`, pgdialect.Array(statusDayIDs), pgdialect.Array(studentIDs)).
		Set(`substatus = (?::text[])[array_position(?::bigint[], "attendance".student_id)]`, pgdialect.Array(substatuses), pgdialect.Array(studentIDs)).
		Set(`updated_at = ?`, updatedAt).Where(`"attendance".tenant_id = ?`, tenantID).
		Where(`"attendance".instance_id = ?`, instanceID).Where(`"attendance".student_id IN (?)`, bun.List(studentIDs)).
		Where(`NOT "attendance".not_scheduled`).Where(`"attendance".status = 'expected'`)
	return runInstanceStudentUpdate(ctx, query, "apply active status days to instance")
}

func statusDayArrays(statuses []domain.StudentStatusDay) ([]int64, []int64, []string) {
	studentIDs, statusDayIDs, substatuses := make([]int64, 0, len(statuses)), make([]int64, 0, len(statuses)), make([]string, 0, len(statuses))
	for _, status := range statuses {
		studentIDs = append(studentIDs, status.StudentID)
		statusDayIDs = append(statusDayIDs, status.ID)
		substatus, _ := statusDaySubstatus(status.Status).(string)
		substatuses = append(substatuses, substatus)
	}
	return studentIDs, statusDayIDs, substatuses
}

func (s *Store) ApplyPartialAbsence(ctx context.Context, exception domain.PickupException, includeCompleted bool, updatedAt time.Time) (int64, domain.OperationStats, error) {
	if exception.ExcusedFrom == nil {
		return 0, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := db.NewUpdate().TableExpr(`schedule.instance_students AS "attendance"`).Set(`status = 'absent'`).
		Set(`substatus = 'excused'`).Set(`student_status_day_id = NULL`).Set(`pickup_exception_id = ?`, exception.ID).
		Set(`updated_at = ?`, updatedAt).Where(`"attendance".tenant_id = ?`, tenantID).
		Where(`"attendance".student_id = ?`, exception.StudentID).Where(`"attendance".manual_status_at IS NULL`).
		Where(`NOT "attendance".not_scheduled`).Where(`("attendance".status = 'expected' OR ("attendance".status = 'absent' AND "attendance".pickup_exception_id IS NULL AND "attendance".student_status_day_id IS NULL))`)
	if includeCompleted {
		query = query.Where(`EXISTS (SELECT 1 FROM schedule.activity_instances AS "instance" WHERE "instance".id = "attendance".instance_id AND "instance".tenant_id = "attendance".tenant_id AND "instance".date = ?::date AND "instance".start_time >= ?::time AND "instance".status <> 'cancelled')`, exception.ExceptionDate, wallClock(*exception.ExcusedFrom))
	} else {
		query = query.Where(`EXISTS (SELECT 1 FROM schedule.activity_instances AS "instance" WHERE "instance".id = "attendance".instance_id AND "instance".tenant_id = "attendance".tenant_id AND "instance".date = ?::date AND "instance".start_time >= ?::time AND "instance".status NOT IN ('cancelled', 'completed'))`, exception.ExceptionDate, wallClock(*exception.ExcusedFrom))
	}
	return runInstanceStudentUpdate(ctx, query, "apply partial absence to slots")
}

func (s *Store) ReleasePartialAbsence(ctx context.Context, pickupExceptionID, studentID int64, replacement *domain.StudentStatusDay, updatedAt time.Time) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	replacementID, replacementStatus := statusDayReplacement(replacement)
	query := db.NewUpdate().TableExpr(`schedule.instance_students AS "attendance"`).
		Set(`status = CASE WHEN ?::bigint IS NOT NULL THEN 'absent' ELSE 'expected' END`, replacementID).
		Set(`substatus = ?`, statusDaySubstatus(replacementStatus)).Set(`student_status_day_id = ?`, replacementID).
		Set(`pickup_exception_id = NULL`).Set(`updated_at = ?`, updatedAt).
		Where(`"attendance".tenant_id = ?`, tenantID).Where(`"attendance".pickup_exception_id = ?`, pickupExceptionID).
		Where(`"attendance".student_id = ?`, studentID).
		Where(`EXISTS (SELECT 1 FROM schedule.activity_instances AS "instance" WHERE "instance".id = "attendance".instance_id AND "instance".tenant_id = "attendance".tenant_id AND "instance".status <> 'completed')`)
	return runInstanceStudentUpdate(ctx, query, "release partial absence from slots")
}

func (s *Store) ApplyActivePartialAbsencesForInstance(ctx context.Context, instanceID int64, date string, exceptions []domain.PickupException, updatedAt time.Time) (int64, domain.OperationStats, error) {
	exceptions = timedPickupExceptions(exceptions)
	if len(exceptions) == 0 {
		return 0, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	studentIDs, exceptionIDs, cutoffs := partialAbsenceArrays(exceptions)
	query := db.NewUpdate().TableExpr(`schedule.instance_students AS "attendance"`).Set(`status = 'absent'`).
		Set(`substatus = 'excused'`).Set(`student_status_day_id = NULL`).
		Set(`pickup_exception_id = (?::bigint[])[array_position(?::bigint[], "attendance".student_id)]`, pgdialect.Array(exceptionIDs), pgdialect.Array(studentIDs)).
		Set(`updated_at = ?`, updatedAt).Where(`"attendance".tenant_id = ?`, tenantID).
		Where(`"attendance".instance_id = ?`, instanceID).Where(`"attendance".manual_status_at IS NULL`).
		Where(`NOT "attendance".not_scheduled`).
		Where(`("attendance".status = 'expected' OR ("attendance".status = 'absent' AND "attendance".pickup_exception_id IS NULL AND "attendance".student_status_day_id IS NULL))`).
		Where(`"attendance".student_id IN (?)`, bun.List(studentIDs)).
		Where(`EXISTS (SELECT 1 FROM schedule.activity_instances AS "instance" WHERE "instance".id = "attendance".instance_id AND "instance".tenant_id = "attendance".tenant_id AND "instance".date = ?::date AND "instance".start_time >= (?::time[])[array_position(?::bigint[], "attendance".student_id)] AND "instance".status <> 'cancelled')`, date, pgdialect.Array(cutoffs), pgdialect.Array(studentIDs))
	return runInstanceStudentUpdate(ctx, query, "apply active partial absences to instance")
}

func timedPickupExceptions(values []domain.PickupException) []domain.PickupException {
	result := make([]domain.PickupException, 0, len(values))
	for _, value := range values {
		if value.ExcusedFrom != nil {
			result = append(result, value)
		}
	}
	return result
}

func partialAbsenceArrays(values []domain.PickupException) ([]int64, []int64, []string) {
	studentIDs, exceptionIDs, cutoffs := make([]int64, 0, len(values)), make([]int64, 0, len(values)), make([]string, 0, len(values))
	for _, value := range values {
		studentIDs = append(studentIDs, value.StudentID)
		exceptionIDs = append(exceptionIDs, value.ID)
		cutoffs = append(cutoffs, wallClock(*value.ExcusedFrom))
	}
	return studentIDs, exceptionIDs, cutoffs
}

func (s *Store) ListPartialAbsenceBlocks(ctx context.Context, studentID int64, date string, from time.Time, enrolled bool, autoExceptionIDs []int64) ([]domain.PartialAbsenceBlock, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []domain.PartialAbsenceBlock{}
	stats, err := scanAllInto(ctx, partialAbsenceAttendanceQuery(db, tenantID, studentID, date, from, autoExceptionIDs), &rows, "list partial absence attendance blocks")
	if err != nil {
		return nil, stats, err
	}
	if enrolled {
		unmaterialized := []domain.PartialAbsenceBlock{}
		queryStats, queryErr := scanAllInto(ctx, partialAbsenceEnrollmentQuery(db, tenantID, studentID, date, from), &unmaterialized, "list partial absence enrollment blocks")
		stats.Add(queryStats)
		if queryErr != nil {
			return nil, stats, queryErr
		}
		rows = mergePartialAbsenceBlocks(rows, unmaterialized)
	}
	stats.Rows = int64(len(rows))
	return rows, stats, nil
}

func partialAbsenceAttendanceQuery(db bun.IDB, tenantID, studentID int64, date string, from time.Time, autoIDs []int64) *bun.SelectQuery {
	return db.NewSelect().TableExpr(`schedule.activity_instances AS "instance"`).
		ColumnExpr(`"instance".id, "instance".title, "instance".start_time, "instance".end_time`).
		Where(`"instance".tenant_id = ?`, tenantID).Where(`"instance".date = ?::date`, date).
		Where(`"instance".start_time >= ?::time`, wallClock(from)).Where(`"instance".status NOT IN ('cancelled', 'completed')`).
		Where(`EXISTS (
	SELECT 1 FROM schedule.instance_students AS "attendance"
	WHERE "attendance".tenant_id = "instance".tenant_id AND "attendance".instance_id = "instance".id
		AND "attendance".student_id = ? AND "attendance".manual_status_at IS NULL
		AND NOT "attendance".not_scheduled AND "attendance".status IN ('expected', 'absent')
		AND "attendance".student_status_day_id IS NULL
		AND ("attendance".pickup_exception_id IS NULL OR "attendance".pickup_exception_id = ANY(?::BIGINT[])))`, studentID, pgdialect.Array(autoIDs)).
		OrderExpr(`"instance".start_time ASC, "instance".id ASC`)
}

func partialAbsenceEnrollmentQuery(db bun.IDB, tenantID, studentID int64, date string, from time.Time) *bun.SelectQuery {
	return db.NewSelect().TableExpr(`schedule.activity_instances AS "instance"`).
		Distinct().ColumnExpr(`"instance".id, "instance".title, "instance".start_time, "instance".end_time`).
		Join(`LEFT JOIN schedule.instance_students AS "attendance" ON "attendance".tenant_id = "instance".tenant_id AND "attendance".instance_id = "instance".id AND "attendance".student_id = ?`, studentID).
		Join(`INNER JOIN activities.student_enrollments AS "enrollment" ON "enrollment".tenant_id = "instance".tenant_id AND "enrollment".student_id = ? AND "enrollment".activity_group_id = "instance".activity_group_id`, studentID).
		Where(`"instance".tenant_id = ?`, tenantID).Where(`"instance".date = ?::date`, date).
		Where(`"instance".start_time >= ?::time`, wallClock(from)).Where(`"instance".status NOT IN ('cancelled', 'completed')`).
		Where(`"attendance".id IS NULL`).Where(`"enrollment".valid_from <= "instance".date`).
		Where(`("enrollment".valid_until IS NULL OR "enrollment".valid_until > "instance".date)`).
		Where(`("enrollment".calendar_period_id IS NULL OR "enrollment".calendar_period_id = "instance".calendar_period_id)`).
		Where(`("enrollment".weekday IS NULL OR "enrollment".weekday = date_part('isodow', "instance".date))`).
		Where(`(COALESCE(jsonb_array_length("enrollment".selected_weekdays), 0) = 0 OR "enrollment".selected_weekdays @> to_jsonb(ARRAY[date_part('isodow', "instance".date)::integer]))`).
		OrderExpr(`"instance".start_time ASC, "instance".id ASC`)
}

func mergePartialAbsenceBlocks(first, second []domain.PartialAbsenceBlock) []domain.PartialAbsenceBlock {
	byID := make(map[int64]domain.PartialAbsenceBlock, len(first)+len(second))
	for _, value := range append(first, second...) {
		byID[value.ID] = value
	}
	result := make([]domain.PartialAbsenceBlock, 0, len(byID))
	for _, value := range byID {
		result = append(result, value)
	}
	slices.SortFunc(result, func(a, b domain.PartialAbsenceBlock) int {
		if order := a.StartTime.Compare(b.StartTime); order != 0 {
			return order
		}
		return int(a.ID - b.ID)
	})
	return result
}

func statusDayReplacement(value *domain.StudentStatusDay) (*int64, string) {
	if value == nil {
		return nil, ""
	}
	return &value.ID, value.Status
}

func statusDaySubstatus(status string) any {
	switch status {
	case "sick":
		return "sick"
	case "excused":
		return "excused"
	case "class_trip":
		return "field_trip"
	default:
		return nil
	}
}

func wallClock(value time.Time) string { return value.Format("15:04:05") }

func runInstanceStudentUpdate(ctx context.Context, query *bun.UpdateQuery, operation string) (int64, domain.OperationStats, error) {
	stats := domain.OperationStats{}
	err := execUpdate(ctx, query, operation, &stats)
	return stats.Rows, stats, err
}
