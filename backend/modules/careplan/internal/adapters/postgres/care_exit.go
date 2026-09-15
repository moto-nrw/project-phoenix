package postgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/uptrace/bun"
)

type careExitRow struct {
	bun.BaseModel          `bun:"table:student_care_exits,alias:care_exit"`
	ID                     int64         `bun:"id,pk,autoincrement"`
	TenantID               int64         `bun:"tenant_id,notnull"`
	CreatedAt              time.Time     `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt              time.Time     `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID              int64         `bun:"student_id,notnull"`
	PreviousEnrolledUntil  *calendarDate `bun:"previous_enrolled_until,type:date"`
	Reason                 string        `bun:"reason,notnull"`
	ReasonNote             *string       `bun:"reason_note"`
	RecordedBy             *int64        `bun:"recorded_by"`
	WithdrawalCompletionID *int64        `bun:"withdrawal_completion_id"`
}

func (s *Store) FindCareExits(ctx context.Context, studentIDs []int64) (map[int64]domain.CareExit, domain.OperationStats, error) {
	result := make(map[int64]domain.CareExit, len(studentIDs))
	if len(studentIDs) == 0 {
		return result, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []careExitRow{}
	query := withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(`users.student_care_exits AS "care_exit"`).
		Where(`"care_exit".student_id IN (?)`, bun.List(studentIDs)), "care_exit", tenantID)
	stats, err := scanAll(ctx, query, "find care exits by student ids")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	for _, row := range rows {
		result[row.StudentID] = careExitToDomain(row)
	}
	return result, stats, nil
}

func (s *Store) UpsertCareExit(ctx context.Context, value domain.CareExit) (domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "upsert care exit")
	if err != nil {
		return domain.OperationStats{}, err
	}
	row := careExitFromDomain(value)
	row.TenantID = tenantID
	return execAny(ctx, db.NewInsert().Model(&row).
		ModelTableExpr(`users.student_care_exits AS "care_exit"`).
		On("CONFLICT (tenant_id, student_id) DO UPDATE").
		Set("reason = EXCLUDED.reason").
		Set("reason_note = EXCLUDED.reason_note").
		Set("recorded_by = EXCLUDED.recorded_by").
		Set("withdrawal_completion_id = EXCLUDED.withdrawal_completion_id").
		Set("updated_at = NOW()"), "upsert care exit")
}

func (s *Store) DeleteCareExits(ctx context.Context, studentIDs []int64) (domain.OperationStats, error) {
	if len(studentIDs) == 0 {
		return domain.OperationStats{}, nil
	}
	db, tenantID, err := s.databaseForWrite(ctx, "delete care exits")
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := db.NewDelete().Model((*careExitRow)(nil)).
		ModelTableExpr(`users.student_care_exits AS "care_exit"`).
		Where(`"care_exit".student_id IN (?)`, bun.List(studentIDs)).
		Where(`"care_exit".tenant_id = ?`, tenantID)
	return execAny(ctx, query, "delete care exits")
}

func careExitFromDomain(value domain.CareExit) careExitRow {
	var previous *calendarDate
	if value.PreviousEnrolledUntil != nil {
		converted := calendarDate(value.PreviousEnrolledUntil.String())
		previous = &converted
	}
	return careExitRow{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		StudentID: value.StudentID, PreviousEnrolledUntil: previous, Reason: value.Reason,
		ReasonNote: value.ReasonNote, RecordedBy: value.RecordedBy, WithdrawalCompletionID: value.WithdrawalCompletionID,
	}
}

func careExitToDomain(row careExitRow) domain.CareExit {
	var previous *careplan.Date
	if row.PreviousEnrolledUntil != nil {
		converted := careplan.Date(*row.PreviousEnrolledUntil)
		previous = &converted
	}
	return domain.CareExit{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StudentID: row.StudentID, PreviousEnrolledUntil: previous, Reason: row.Reason,
		ReasonNote: row.ReasonNote, RecordedBy: row.RecordedBy, WithdrawalCompletionID: row.WithdrawalCompletionID,
	}
}
