package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	enrollmentprovenance "github.com/moto-nrw/project-phoenix/modules/timetableenrollmentprovenance"
	"github.com/uptrace/bun"
)

type studentEnrollmentRow struct {
	bun.BaseModel            `bun:"table:student_enrollments,alias:student_enrollment"`
	ID                       int64     `bun:"id,pk,autoincrement"`
	TenantID                 int64     `bun:"tenant_id,notnull"`
	CreatedAt                time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt                time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID                int64     `bun:"student_id,notnull"`
	ActivityGroupID          int64     `bun:"activity_group_id,notnull"`
	ValidFrom                string    `bun:"valid_from,notnull"`
	ValidUntil               *string   `bun:"valid_until"`
	CalendarPeriodID         *int64    `bun:"calendar_period_id"`
	EnrollmentRequestChildID *int64    `bun:"enrollment_request_child_id"`
	SelectedWeekdays         []int     `bun:"selected_weekdays,type:jsonb,nullzero"`
	AttendanceStatus         *string   `bun:"attendance_status"`
	Weekday                  *int      `bun:"weekday"`
}

func (s *Store) FindStudentEnrollment(ctx context.Context, id int64) (domain.StudentEnrollment, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StudentEnrollment{}, false, domain.OperationStats{}, err
	}
	row := studentEnrollmentRow{}
	found, stats, err := scanOne(ctx, studentEnrollmentSelect(db, &row, tenantID).Where(`"student_enrollment".id = ?`, id), "find student enrollment")
	return studentEnrollmentToDomain(row), found, stats, err
}

func (s *Store) ListStudentEnrollments(ctx context.Context, filter domain.StudentEnrollmentFilter) ([]domain.StudentEnrollment, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []studentEnrollmentRow{}
	query := filterStudentEnrollments(studentEnrollmentSelect(db, &rows, tenantID), filter)
	stats, err := scanAll(ctx, query, "list student enrollments")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.StudentEnrollment, 0, len(rows))
	for _, row := range rows {
		result = append(result, studentEnrollmentToDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func filterStudentEnrollments(query *bun.SelectQuery, filter domain.StudentEnrollmentFilter) *bun.SelectQuery {
	if len(filter.StudentIDs) > 0 {
		query = query.Where(`"student_enrollment".student_id IN (?)`, bun.List(filter.StudentIDs))
	}
	if len(filter.ActivityGroupIDs) > 0 {
		query = query.Where(`"student_enrollment".activity_group_id IN (?)`, bun.List(filter.ActivityGroupIDs))
	}
	if filter.ActiveOn != nil {
		query = activeStudentEnrollmentQuery(query, *filter.ActiveOn)
	}
	if filter.OrderByGroupName {
		query = query.Join(`LEFT JOIN activities.groups AS "activity_group" ON "activity_group".tenant_id = "student_enrollment".tenant_id AND "activity_group".id = "student_enrollment".activity_group_id`).OrderExpr(`"student_enrollment".student_id ASC, "activity_group".name ASC`)
	} else if filter.OrderByValidFrom {
		query = query.OrderExpr(`"student_enrollment".valid_from DESC`)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit).Offset(filter.Offset)
	}
	return query
}

func activeStudentEnrollmentQuery(query *bun.SelectQuery, onDate string) *bun.SelectQuery {
	return query.Where(`"student_enrollment".valid_from <= ?`, onDate).
		Where(`("student_enrollment".valid_until IS NULL OR "student_enrollment".valid_until > ?)`, onDate).
		Where(`("student_enrollment".weekday IS NULL OR "student_enrollment".weekday = EXTRACT(ISODOW FROM CAST(? AS DATE))::INT)`, onDate)
}

func (s *Store) CreateStudentEnrollment(ctx context.Context, fields domain.StudentEnrollmentFields) (domain.StudentEnrollment, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StudentEnrollment{}, domain.OperationStats{}, err
	}
	row := studentEnrollmentRow{TenantID: tenantID}
	applyStudentEnrollmentFields(&row, fields)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`activities.student_enrollments`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.StudentEnrollment{}, stats, classifyWriteError("create student enrollment", err, &stats)
	}
	stats.Rows = 1
	return studentEnrollmentToDomain(row), stats, nil
}

func (s *Store) UpdateStudentEnrollment(ctx context.Context, id int64, fields domain.StudentEnrollmentFields) (domain.StudentEnrollment, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StudentEnrollment{}, false, domain.OperationStats{}, err
	}
	row := studentEnrollmentRow{ID: id, TenantID: tenantID}
	applyStudentEnrollmentFields(&row, fields)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewUpdate().Model(&row).ModelTableExpr(`activities.student_enrollments`).
		Column("student_id", "activity_group_id", "valid_from", "valid_until", "calendar_period_id", "enrollment_request_child_id", "selected_weekdays", "attendance_status", "weekday").
		Where("id = ?", id).Where("tenant_id = ?", tenantID).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.StudentEnrollment{}, false, stats, nil
	}
	if err != nil {
		return domain.StudentEnrollment{}, false, stats, classifyWriteError("update student enrollment", err, &stats)
	}
	stats.Rows = 1
	return studentEnrollmentToDomain(row), true, stats, nil
}

func (s *Store) DeleteStudentEnrollment(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execMeasuredWrite(ctx, db.NewDelete().Table("activities.student_enrollments").Where("tenant_id = ?", tenantID).Where("id = ?", id), "delete student enrollment")
}

func (s *Store) BackfillStudentEnrollmentSource(ctx context.Context, studentID, requestChildID int64, groupIDs []int64) (int64, domain.OperationStats, error) {
	if len(groupIDs) == 0 {
		return 0, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	ids, err := enrollmentprovenance.EligibleEnrollmentIDs(ctx, db, tenantID, studentID, requestChildID, groupIDs)
	stats.StatementDuration = time.Since(started)
	if err != nil || len(ids) == 0 {
		return 0, stats, err
	}
	writeStats, err := execMeasuredWrite(ctx, db.NewUpdate().Table("activities.student_enrollments").
		Set("enrollment_request_child_id = ?", requestChildID).Set("updated_at = NOW()").
		Where("tenant_id = ?", tenantID).Where("id IN (?)", bun.List(ids)), "backfill student enrollment source")
	stats.Add(writeStats)
	return writeStats.Rows, stats, err
}

func (s *Store) DeleteStudentEnrollmentsBySource(ctx context.Context, studentID, requestChildID int64) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats, err := execMeasuredWrite(ctx, db.NewDelete().Table("activities.student_enrollments").
		Where("tenant_id = ?", tenantID).Where("student_id = ?", studentID).
		Where("enrollment_request_child_id = ?", requestChildID), "delete student enrollments by source")
	return stats.Rows, stats, err
}

func (s *Store) SetStudentEnrollmentValidUntil(ctx context.Context, id int64, validUntil string) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	stats, err := execMeasuredWrite(ctx, db.NewUpdate().Table("activities.student_enrollments").Set("valid_until = ?", validUntil).
		Where("tenant_id = ?", tenantID).Where("id = ?", id), "set student enrollment valid_until")
	return stats.Rows == 1, stats, err
}

func (s *Store) CloseOpenStudentEnrollments(ctx context.Context, groupID int64, periodID *int64, validUntil string) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := db.NewUpdate().Table("activities.student_enrollments").Set("valid_until = ?", validUntil).
		Where("tenant_id = ?", tenantID).Where("activity_group_id = ?", groupID).Where("valid_until IS NULL")
	if periodID == nil {
		query = query.Where("calendar_period_id IS NULL")
	} else {
		query = query.Where("calendar_period_id = ?", *periodID)
	}
	return execMeasuredWrite(ctx, query, "close open student enrollments")
}

func (s *Store) CapActiveStudentEnrollments(ctx context.Context, groupID int64, validUntil string) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats, err := execMeasuredWrite(ctx, db.NewDelete().Table("activities.student_enrollments").
		Where("tenant_id = ?", tenantID).Where("activity_group_id = ?", groupID).Where("valid_from >= ?", validUntil).
		Where("valid_until IS NULL").Where("enrollment_request_child_id IS NULL").Where("COALESCE(jsonb_array_length(selected_weekdays), 0) = 0"), "delete future student enrollments")
	if err != nil {
		return stats.Rows, stats, err
	}
	updated, err := execMeasuredWrite(ctx, db.NewUpdate().Table("activities.student_enrollments").Set("valid_until = ?", validUntil).
		Where("tenant_id = ?", tenantID).Where("activity_group_id = ?", groupID).Where("valid_from < ?", validUntil).
		Where("valid_until IS NULL").Where("enrollment_request_child_id IS NULL").Where("COALESCE(jsonb_array_length(selected_weekdays), 0) = 0"), "cap active student enrollments")
	total := stats.Rows + updated.Rows
	stats.Add(updated)
	return total, stats, err
}

func execMeasuredWrite(ctx context.Context, query executableQuery, operation string) (domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, classifyWriteError(operation, err, &stats)
	}
	_, err = rowsAffected(result, operation, &stats)
	return stats, err
}

func studentEnrollmentSelect(db bun.IDB, model any, tenantID int64) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`activities.student_enrollments AS "student_enrollment"`).
		ColumnExpr(`"student_enrollment".*`).Where(`"student_enrollment".tenant_id = ?`, tenantID)
}

func applyStudentEnrollmentFields(row *studentEnrollmentRow, fields domain.StudentEnrollmentFields) {
	row.StudentID, row.ActivityGroupID = fields.StudentID, fields.ActivityGroupID
	row.ValidFrom, row.ValidUntil = fields.ValidFrom, fields.ValidUntil
	row.CalendarPeriodID, row.EnrollmentRequestChildID = fields.CalendarPeriodID, fields.EnrollmentRequestChildID
	row.SelectedWeekdays, row.AttendanceStatus, row.Weekday = fields.SelectedWeekdays, fields.AttendanceStatus, fields.Weekday
}

func studentEnrollmentToDomain(row studentEnrollmentRow) domain.StudentEnrollment {
	return domain.StudentEnrollment{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StudentID: row.StudentID, ActivityGroupID: row.ActivityGroupID, ValidFrom: row.ValidFrom, ValidUntil: row.ValidUntil,
		CalendarPeriodID: row.CalendarPeriodID, EnrollmentRequestChildID: row.EnrollmentRequestChildID,
		SelectedWeekdays: row.SelectedWeekdays, AttendanceStatus: row.AttendanceStatus, Weekday: row.Weekday}
}
