package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/uptrace/bun"
)

type careExitRemovalRow struct {
	bun.BaseModel            `bun:"table:student_care_exit_removals,alias:care_exit_removal"`
	ID                       int64         `bun:"id,pk,autoincrement"`
	TenantID                 int64         `bun:"tenant_id,notnull"`
	StudentID                int64         `bun:"student_id,notnull"`
	Kind                     string        `bun:"kind,notnull"`
	InstanceID               *int64        `bun:"instance_id"`
	RoomID                   *int64        `bun:"room_id"`
	Status                   *string       `bun:"status"`
	Substatus                *string       `bun:"substatus"`
	Note                     *string       `bun:"note"`
	IsUnplanned              *bool         `bun:"is_unplanned"`
	NotScheduled             *bool         `bun:"not_scheduled"`
	ManualStatusAt           *time.Time    `bun:"manual_status_at"`
	StudentStatusDayID       *int64        `bun:"student_status_day_id"`
	PickupExceptionID        *int64        `bun:"pickup_exception_id"`
	EnrollmentID             *int64        `bun:"enrollment_id"`
	WasDeleted               bool          `bun:"was_deleted,notnull"`
	PreviousValidUntil       *calendarDate `bun:"previous_valid_until,type:date"`
	ActivityGroupID          *int64        `bun:"activity_group_id"`
	ValidFrom                *calendarDate `bun:"valid_from,type:date"`
	CalendarPeriodID         *int64        `bun:"calendar_period_id"`
	EnrollmentRequestChildID *int64        `bun:"enrollment_request_child_id"`
	SelectedWeekdays         []int         `bun:"selected_weekdays,type:jsonb"`
	AttendanceStatus         *string       `bun:"attendance_status"`
	Weekday                  *int          `bun:"weekday"`
	CreatedAt                time.Time     `bun:"created_at,nullzero,notnull,default:current_timestamp"`
}

type careExitSourceRemovalRow struct {
	bun.BaseModel `bun:"table:student_care_exit_source_removals,alias:care_exit_source_removal"`
	ID            int64           `bun:"id,pk,autoincrement"`
	TenantID      int64           `bun:"tenant_id,notnull"`
	StudentID     int64           `bun:"student_id,notnull"`
	Kind          string          `bun:"kind,notnull"`
	SourceRowID   int64           `bun:"source_row_id,notnull"`
	WasDeleted    bool            `bun:"was_deleted,notnull"`
	Snapshot      json.RawMessage `bun:"snapshot,type:jsonb,notnull"`
	CreatedAt     time.Time       `bun:"created_at,nullzero,notnull,default:current_timestamp"`
}

func (s *Store) ListCareExitRemovals(ctx context.Context, studentIDs []int64) ([]domain.CareExitRemoval, domain.OperationStats, error) {
	if len(studentIDs) == 0 {
		return []domain.CareExitRemoval{}, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []careExitRemovalRow{}
	query := withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(`users.student_care_exit_removals AS "care_exit_removal"`).
		Where(`"care_exit_removal".student_id IN (?)`, bun.List(studentIDs)), "care_exit_removal", tenantID)
	stats, err := scanAll(ctx, query, "list care exit removals")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.CareExitRemoval, 0, len(rows))
	for _, row := range rows {
		result = append(result, careExitRemovalToDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func (s *Store) ListCareExitSourceRemovals(ctx context.Context, studentIDs []int64) ([]domain.CareExitSourceRemoval, domain.OperationStats, error) {
	if len(studentIDs) == 0 {
		return []domain.CareExitSourceRemoval{}, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []careExitSourceRemovalRow{}
	query := withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(`users.student_care_exit_source_removals AS "care_exit_source_removal"`).
		Where(`"care_exit_source_removal".student_id IN (?)`, bun.List(studentIDs)), "care_exit_source_removal", tenantID)
	stats, err := scanAll(ctx, query, "list care exit source removals")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.CareExitSourceRemoval, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.CareExitSourceRemoval{
			ID: row.ID, TenantID: row.TenantID, StudentID: row.StudentID,
			Kind: row.Kind, SourceRowID: row.SourceRowID, WasDeleted: row.WasDeleted,
			Snapshot: row.Snapshot, CreatedAt: row.CreatedAt,
		})
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func (s *Store) RecordCareExitRemovals(ctx context.Context, values []domain.CareExitRemoval) (domain.OperationStats, error) {
	if len(values) == 0 {
		return domain.OperationStats{}, nil
	}
	db, tenantID, err := s.databaseForWrite(ctx, "record care exit removals")
	if err != nil {
		return domain.OperationStats{}, err
	}
	rows := make([]careExitRemovalRow, 0, len(values))
	for _, value := range values {
		row := careExitRemovalFromDomain(value)
		row.ID, row.TenantID, row.CreatedAt = 0, tenantID, time.Time{}
		rows = append(rows, row)
	}
	attempted := int64(len(rows))
	stats, err := execAny(ctx, db.NewInsert().Model(&rows).
		ModelTableExpr(`users.student_care_exit_removals AS "care_exit_removal"`).
		On("CONFLICT DO NOTHING"), "record care exit removals")
	if err == nil {
		stats.Conflicts = attempted - stats.Rows
	}
	return stats, err
}

func (s *Store) RecordCareExitSourceRemovals(ctx context.Context, values []domain.CareExitSourceRemoval) (domain.OperationStats, error) {
	if len(values) == 0 {
		return domain.OperationStats{}, nil
	}
	db, tenantID, err := s.databaseForWrite(ctx, "record care exit source removals")
	if err != nil {
		return domain.OperationStats{}, err
	}
	rows := make([]careExitSourceRemovalRow, 0, len(values))
	for _, value := range values {
		rows = append(rows, careExitSourceRemovalRow{
			TenantID: tenantID, StudentID: value.StudentID, Kind: value.Kind,
			SourceRowID: value.SourceRowID, WasDeleted: value.WasDeleted, Snapshot: value.Snapshot,
		})
	}
	attempted := int64(len(rows))
	stats, err := execAny(ctx, db.NewInsert().Model(&rows).
		ModelTableExpr(`users.student_care_exit_source_removals AS "care_exit_source_removal"`).
		On("CONFLICT DO NOTHING"), "record care exit source removals")
	if err == nil {
		stats.Conflicts = attempted - stats.Rows
	}
	return stats, err
}

func (s *Store) DiscardCareExitRemovals(ctx context.Context, studentIDs []int64) (domain.OperationStats, error) {
	if len(studentIDs) == 0 {
		return domain.OperationStats{}, nil
	}
	db, tenantID, err := s.databaseForWrite(ctx, "discard care exit removals")
	if err != nil {
		return domain.OperationStats{}, err
	}
	stats, err := execAny(ctx, db.NewDelete().Model((*careExitRemovalRow)(nil)).
		ModelTableExpr(`users.student_care_exit_removals AS "care_exit_removal"`).
		Where(`"care_exit_removal".tenant_id = ?`, tenantID).
		Where(`"care_exit_removal".student_id IN (?)`, bun.List(studentIDs)), "discard care exit removals")
	if err != nil {
		return stats, err
	}
	other, err := execAny(ctx, db.NewDelete().Model((*careExitSourceRemovalRow)(nil)).
		ModelTableExpr(`users.student_care_exit_source_removals AS "care_exit_source_removal"`).
		Where(`"care_exit_source_removal".tenant_id = ?`, tenantID).
		Where(`"care_exit_source_removal".student_id IN (?)`, bun.List(studentIDs)), "discard care exit source removals")
	stats.Add(other)
	return stats, err
}

func careExitRemovalFromDomain(value domain.CareExitRemoval) careExitRemovalRow {
	row := careExitRemovalRow{
		ID: value.ID, TenantID: value.TenantID, StudentID: value.StudentID, Kind: value.Kind,
		InstanceID: value.InstanceID, RoomID: value.RoomID, Status: value.Status, Substatus: value.Substatus,
		Note: value.Note, IsUnplanned: value.IsUnplanned, NotScheduled: value.NotScheduled,
		ManualStatusAt: value.ManualStatusAt, StudentStatusDayID: value.StudentStatusDayID,
		PickupExceptionID: value.PickupExceptionID, EnrollmentID: value.EnrollmentID,
		WasDeleted: value.WasDeleted, ActivityGroupID: value.ActivityGroupID,
		CalendarPeriodID: value.CalendarPeriodID, EnrollmentRequestChildID: value.EnrollmentRequestChildID,
		SelectedWeekdays: value.SelectedWeekdays, AttendanceStatus: value.AttendanceStatus,
		Weekday: value.Weekday, CreatedAt: value.CreatedAt,
	}
	if value.PreviousValidUntil != nil {
		date := calendarDate(value.PreviousValidUntil.String())
		row.PreviousValidUntil = &date
	}
	if value.ValidFrom != nil {
		date := calendarDate(value.ValidFrom.String())
		row.ValidFrom = &date
	}
	return row
}

func careExitRemovalToDomain(row careExitRemovalRow) domain.CareExitRemoval {
	value := domain.CareExitRemoval{
		ID: row.ID, TenantID: row.TenantID, StudentID: row.StudentID, Kind: row.Kind,
		InstanceID: row.InstanceID, RoomID: row.RoomID, Status: row.Status, Substatus: row.Substatus,
		Note: row.Note, IsUnplanned: row.IsUnplanned, NotScheduled: row.NotScheduled,
		ManualStatusAt: row.ManualStatusAt, StudentStatusDayID: row.StudentStatusDayID,
		PickupExceptionID: row.PickupExceptionID, EnrollmentID: row.EnrollmentID,
		WasDeleted: row.WasDeleted, ActivityGroupID: row.ActivityGroupID,
		CalendarPeriodID: row.CalendarPeriodID, EnrollmentRequestChildID: row.EnrollmentRequestChildID,
		SelectedWeekdays: row.SelectedWeekdays, AttendanceStatus: row.AttendanceStatus,
		Weekday: row.Weekday, CreatedAt: row.CreatedAt,
	}
	if row.PreviousValidUntil != nil {
		date := careplan.Date(*row.PreviousValidUntil)
		value.PreviousValidUntil = &date
	}
	if row.ValidFrom != nil {
		date := careplan.Date(*row.ValidFrom)
		value.ValidFrom = &date
	}
	return value
}
