package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
)

type scheduledInstanceRow struct {
	StudentRowID       int64      `bun:"is_id"`
	StudentTenantID    int64      `bun:"is_tenant_id"`
	InstanceID         int64      `bun:"is_instance_id"`
	StudentID          int64      `bun:"is_student_id"`
	StudentRoomID      *int64     `bun:"is_room_id"`
	AttendanceStatus   string     `bun:"is_status"`
	Substatus          *string    `bun:"is_substatus"`
	Note               *string    `bun:"is_note"`
	CheckedInAt        *time.Time `bun:"is_checked_in_at"`
	CheckedOutAt       *time.Time `bun:"is_checked_out_at"`
	IsUnplanned        bool       `bun:"is_unplanned"`
	NotScheduled       bool       `bun:"is_not_scheduled"`
	ManualStatusAt     *time.Time `bun:"is_manual_status_at"`
	StudentStatusDay   *int64     `bun:"is_student_status_day_id"`
	StudentCreatedAt   time.Time  `bun:"is_created_at"`
	StudentUpdatedAt   time.Time  `bun:"is_updated_at"`
	ActivityInstanceID int64      `bun:"ai_id"`
	ActivityTenantID   int64      `bun:"ai_tenant_id"`
	Date               string     `bun:"ai_date"`
	ActivityGroupID    *int64     `bun:"ai_activity_group_id"`
	CalendarPeriodID   *int64     `bun:"ai_calendar_period_id"`
	Title              string     `bun:"ai_title"`
	Description        *string    `bun:"ai_description"`
	StartTime          string     `bun:"ai_start_time"`
	EndTime            string     `bun:"ai_end_time"`
	ActivityRoomID     int64      `bun:"ai_room_id"`
	ActivityStatus     string     `bun:"ai_status"`
	ActiveGroupID      *int64     `bun:"ai_active_group_id"`
	ActivityUnplanned  bool       `bun:"ai_is_spontaneous"`
	ActivityNotes      *string    `bun:"ai_notes"`
	CreatedBy          *int64     `bun:"ai_created_by"`
	StartedBy          *int64     `bun:"ai_started_by"`
	StartedAt          *time.Time `bun:"ai_started_at"`
	CompletedAt        *time.Time `bun:"ai_completed_at"`
	ActivityCreatedAt  time.Time  `bun:"ai_created_at"`
	ActivityUpdatedAt  time.Time  `bun:"ai_updated_at"`
}

func (s *Store) ListScheduledInstancesForStudent(ctx context.Context, studentID int64, from, to string) ([]domain.ScheduledInstanceRow, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []scheduledInstanceRow{}
	query := scheduledInstancesForStudentQuery(db, tenantID, studentID, from, to)
	stats, err := scanAllInto(ctx, query, &rows, "list scheduled instances for student")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.ScheduledInstanceRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, scheduledInstanceToDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func scheduledInstancesForStudentQuery(db bun.IDB, tenantID, studentID int64, from, to string) *bun.SelectQuery {
	return db.NewSelect().TableExpr(`schedule.instance_students AS "instance_student"`).
		ColumnExpr(scheduledInstanceStudentColumns).ColumnExpr(scheduledInstanceActivityColumns).
		Join(`INNER JOIN schedule.activity_instances AS "activity_instance" ON "activity_instance".id = "instance_student".instance_id AND "activity_instance".tenant_id = "instance_student".tenant_id`).
		Where(`"instance_student".tenant_id = ?`, tenantID).Where(`"instance_student".student_id = ?`, studentID).
		Where(`"activity_instance".date >= ?::date`, from).Where(`"activity_instance".date <= ?::date`, to).
		Where(`NOT ("instance_student".not_scheduled AND "instance_student".status = 'expected' AND "instance_student".manual_status_at IS NULL)`).
		OrderExpr(`"activity_instance".date ASC, "activity_instance".start_time ASC`)
}

const scheduledInstanceStudentColumns = `
	"instance_student".id AS is_id, "instance_student".tenant_id AS is_tenant_id,
	"instance_student".instance_id AS is_instance_id, "instance_student".student_id AS is_student_id,
	"instance_student".room_id AS is_room_id, "instance_student".status AS is_status,
	"instance_student".substatus AS is_substatus, "instance_student".note AS is_note,
	"instance_student".checked_in_at AS is_checked_in_at, "instance_student".checked_out_at AS is_checked_out_at,
	"instance_student".is_unplanned AS is_unplanned, "instance_student".not_scheduled AS is_not_scheduled,
	"instance_student".manual_status_at AS is_manual_status_at,
	"instance_student".student_status_day_id AS is_student_status_day_id,
	"instance_student".created_at AS is_created_at, "instance_student".updated_at AS is_updated_at`

const scheduledInstanceActivityColumns = `
	"activity_instance".id AS ai_id, "activity_instance".tenant_id AS ai_tenant_id,
	"activity_instance".date::text AS ai_date, "activity_instance".activity_group_id AS ai_activity_group_id,
	"activity_instance".calendar_period_id AS ai_calendar_period_id, "activity_instance".title AS ai_title,
	"activity_instance".description AS ai_description, "activity_instance".start_time::text AS ai_start_time,
	"activity_instance".end_time::text AS ai_end_time, "activity_instance".room_id AS ai_room_id,
	"activity_instance".status AS ai_status, "activity_instance".active_group_id AS ai_active_group_id,
	"activity_instance".is_spontaneous AS ai_is_spontaneous, "activity_instance".notes AS ai_notes,
	"activity_instance".created_by AS ai_created_by, "activity_instance".started_by AS ai_started_by,
	"activity_instance".started_at AS ai_started_at, "activity_instance".completed_at AS ai_completed_at,
	"activity_instance".created_at AS ai_created_at, "activity_instance".updated_at AS ai_updated_at`

func scheduledInstanceToDomain(row scheduledInstanceRow) domain.ScheduledInstanceRow {
	return domain.ScheduledInstanceRow{
		Instance: domain.ActivityInstance{ID: row.ActivityInstanceID, TenantID: row.ActivityTenantID,
			CreatedAt: row.ActivityCreatedAt, UpdatedAt: row.ActivityUpdatedAt, Date: row.Date,
			ActivityGroupID: row.ActivityGroupID, CalendarPeriodID: row.CalendarPeriodID, Title: row.Title,
			Description: row.Description, StartTime: row.StartTime, EndTime: row.EndTime, RoomID: row.ActivityRoomID,
			Status: row.ActivityStatus, ActiveGroupID: row.ActiveGroupID, IsSpontaneous: row.ActivityUnplanned,
			Notes: row.ActivityNotes, CreatedBy: row.CreatedBy, StartedBy: row.StartedBy,
			StartedAt: row.StartedAt, CompletedAt: row.CompletedAt},
		Attendance: domain.InstanceStudent{ID: row.StudentRowID, TenantID: row.StudentTenantID,
			CreatedAt: row.StudentCreatedAt, UpdatedAt: row.StudentUpdatedAt, InstanceID: row.InstanceID,
			StudentID: row.StudentID, RoomID: row.StudentRoomID, Status: row.AttendanceStatus,
			Substatus: row.Substatus, Note: row.Note, CheckedInAt: row.CheckedInAt, CheckedOutAt: row.CheckedOutAt,
			IsUnplanned: row.IsUnplanned, NotScheduled: row.NotScheduled, ManualStatusAt: row.ManualStatusAt,
			StudentStatusDayID: row.StudentStatusDay},
	}
}

func (s *Store) HasPlannedStudentSlots(ctx context.Context, from, to string) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	query := db.NewSelect().TableExpr(`schedule.instance_students AS "instance_student"`).
		Join(`INNER JOIN schedule.activity_instances AS "activity_instance" ON "activity_instance".id = "instance_student".instance_id AND "activity_instance".tenant_id = "instance_student".tenant_id`).
		Where(`"instance_student".tenant_id = ?`, tenantID).Where(`"instance_student".is_unplanned = FALSE`).
		Where(`"activity_instance".status <> 'cancelled'`).Where(`"activity_instance".date >= ?::date`, from).
		Where(`"activity_instance".date <= ?::date`, to)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	exists, err := query.Exists(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return false, stats, fmt.Errorf("timetable postgres: check planned student slots: %w", err)
	}
	return exists, stats, nil
}

func (s *Store) ListPlannedStudentIDs(ctx context.Context, studentIDs []int64, date string) ([]int64, domain.OperationStats, error) {
	if len(studentIDs) == 0 {
		return []int64{}, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	ids := []int64{}
	query := db.NewSelect().TableExpr(`schedule.instance_students AS "instance_student"`).
		ColumnExpr(`DISTINCT "instance_student".student_id`).
		Join(`INNER JOIN schedule.activity_instances AS "activity_instance" ON "activity_instance".id = "instance_student".instance_id AND "activity_instance".tenant_id = "instance_student".tenant_id`).
		Where(`"instance_student".tenant_id = ?`, tenantID).Where(`"instance_student".student_id IN (?)`, bun.List(studentIDs)).
		Where(`"activity_instance".date = ?::date`, date).Where(`"activity_instance".status <> 'cancelled'`).
		OrderExpr(`"instance_student".student_id ASC`)
	stats, err := scanAllInto(ctx, query, &ids, "list planned student ids")
	stats.Rows = int64(len(ids))
	return ids, stats, err
}
