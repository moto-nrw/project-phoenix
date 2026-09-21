package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/uptrace/bun"
)

// The statements below moved unchanged from the retained People Directory
// repository (#3221). The children's names and classes are no longer joined
// from users.students and users.persons: the caller passes the People
// Directory rows as a recordset with the same columns.

const tableExprWithdrawalCompletions = `users.care_withdrawal_completions AS "care_withdrawal_completion"`

const withdrawalStudentRecordset = `jsonb_to_recordset(?::jsonb) AS "student"(id bigint, first_name text, last_name text, school_class text)`

type withdrawalCompletionRow struct {
	bun.BaseModel           `bun:"table:care_withdrawal_completions,alias:care_withdrawal_completion"`
	ID                      int64           `bun:"id,pk,autoincrement"`
	CreatedAt               time.Time       `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt               time.Time       `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID                int64           `bun:"tenant_id,notnull"`
	StudentID               *int64          `bun:"student_id"`
	FirstBookinglessDay     calendarDate    `bun:"first_bookingless_day,type:date,notnull"`
	Trigger                 string          `bun:"trigger,notnull"`
	SourceAdjustmentID      *int64          `bun:"source_adjustment_id"`
	SourceRequestChildID    *int64          `bun:"source_request_child_id"`
	WithdrawalConfirmedBy   *int64          `bun:"withdrawal_confirmed_by"`
	WithdrawalConfirmedRole string          `bun:"withdrawal_confirmed_role,notnull"`
	WithdrawalConfirmedAt   time.Time       `bun:"withdrawal_confirmed_at,notnull"`
	SourceOfferings         json.RawMessage `bun:"source_offerings,type:jsonb,notnull"`
	State                   string          `bun:"state,notnull"`
	Outcome                 *string         `bun:"outcome"`
	ObsoleteReason          *string         `bun:"obsolete_reason"`
	ResolvedBy              *int64          `bun:"resolved_by"`
	ResolvedAt              *time.Time      `bun:"resolved_at"`

	FirstName   string `bun:"first_name,scanonly"`
	LastName    string `bun:"last_name,scanonly"`
	SchoolClass string `bun:"school_class,scanonly"`
}

func withdrawalCompletionToDomain(row withdrawalCompletionRow) domain.WithdrawalCompletion {
	return domain.WithdrawalCompletion{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StudentID: row.StudentID, FirstBookinglessDay: careplan.Date(row.FirstBookinglessDay), Trigger: row.Trigger,
		SourceAdjustmentID: row.SourceAdjustmentID, SourceRequestChildID: row.SourceRequestChildID,
		WithdrawalConfirmedBy: row.WithdrawalConfirmedBy, WithdrawalConfirmedRole: row.WithdrawalConfirmedRole,
		WithdrawalConfirmedAt: row.WithdrawalConfirmedAt, SourceOfferings: row.SourceOfferings,
		State: row.State, Outcome: row.Outcome, ObsoleteReason: row.ObsoleteReason,
		ResolvedBy: row.ResolvedBy, ResolvedAt: row.ResolvedAt,
		FirstName: row.FirstName, LastName: row.LastName, SchoolClass: row.SchoolClass,
	}
}

// The preserveConfirmed* SET fragments keep a school-confirmed withdrawal's
// columns when a booking expiry conflicts with it: the stored value wins when
// the existing row was confirmed directly by the school and the incoming row
// comes from an expired booking; otherwise the incoming (EXCLUDED) value wins.
// Each fragment binds the two trigger values through
// preserveConfirmedWithdrawalArgs, in that order.
const (
	preserveConfirmedTrigger = `trigger = CASE
		WHEN "care_withdrawal_completion".trigger = ? AND EXCLUDED.trigger = ?
		THEN "care_withdrawal_completion".trigger ELSE EXCLUDED.trigger END`
	preserveConfirmedSourceAdjustmentID = `source_adjustment_id = CASE
		WHEN "care_withdrawal_completion".trigger = ? AND EXCLUDED.trigger = ?
		THEN "care_withdrawal_completion".source_adjustment_id ELSE EXCLUDED.source_adjustment_id END`
	preserveConfirmedSourceRequestChildID = `source_request_child_id = CASE
		WHEN "care_withdrawal_completion".trigger = ? AND EXCLUDED.trigger = ?
		THEN "care_withdrawal_completion".source_request_child_id ELSE EXCLUDED.source_request_child_id END`
	preserveConfirmedWithdrawalConfirmedBy = `withdrawal_confirmed_by = CASE
		WHEN "care_withdrawal_completion".trigger = ? AND EXCLUDED.trigger = ?
		THEN "care_withdrawal_completion".withdrawal_confirmed_by ELSE EXCLUDED.withdrawal_confirmed_by END`
	preserveConfirmedWithdrawalConfirmedRole = `withdrawal_confirmed_role = CASE
		WHEN "care_withdrawal_completion".trigger = ? AND EXCLUDED.trigger = ?
		THEN "care_withdrawal_completion".withdrawal_confirmed_role ELSE EXCLUDED.withdrawal_confirmed_role END`
	preserveConfirmedWithdrawalConfirmedAt = `withdrawal_confirmed_at = CASE
		WHEN "care_withdrawal_completion".trigger = ? AND EXCLUDED.trigger = ?
		THEN "care_withdrawal_completion".withdrawal_confirmed_at ELSE EXCLUDED.withdrawal_confirmed_at END`
	preserveConfirmedSourceOfferings = `source_offerings = CASE
		WHEN "care_withdrawal_completion".trigger = ? AND EXCLUDED.trigger = ?
		THEN "care_withdrawal_completion".source_offerings ELSE EXCLUDED.source_offerings END`
)

var preserveConfirmedWithdrawalArgs = []any{
	careplan.WithdrawalTriggerDirectSchool,
	careplan.WithdrawalTriggerBookingExpired,
}

// UpsertPendingWithdrawal implements the domain invariant of one pending task
// per child through the partial unique index.
func (s *Store) UpsertPendingWithdrawal(ctx context.Context, value domain.WithdrawalCompletion) (domain.WithdrawalCompletion, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.WithdrawalCompletion{}, domain.OperationStats{}, err
	}
	row := withdrawalCompletionRow{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		StudentID: value.StudentID, FirstBookinglessDay: calendarDate(value.FirstBookinglessDay), Trigger: value.Trigger,
		SourceAdjustmentID: value.SourceAdjustmentID, SourceRequestChildID: value.SourceRequestChildID,
		WithdrawalConfirmedBy: value.WithdrawalConfirmedBy, WithdrawalConfirmedRole: value.WithdrawalConfirmedRole,
		WithdrawalConfirmedAt: value.WithdrawalConfirmedAt, SourceOfferings: value.SourceOfferings,
		State: careplan.WithdrawalStatePending, Outcome: value.Outcome, ObsoleteReason: value.ObsoleteReason,
		ResolvedBy: value.ResolvedBy, ResolvedAt: value.ResolvedAt,
	}
	if row.TenantID == 0 {
		row.TenantID = tenantID
	}
	if len(row.SourceOfferings) == 0 {
		row.SourceOfferings = json.RawMessage(`[]`)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().
		Model(&row).
		ModelTableExpr(tableExprWithdrawalCompletions).
		On("CONFLICT (tenant_id, student_id) WHERE state = 'pending' AND student_id IS NOT NULL DO UPDATE").
		Set("first_bookingless_day = EXCLUDED.first_bookingless_day").
		Set(preserveConfirmedTrigger, preserveConfirmedWithdrawalArgs...).
		Set(preserveConfirmedSourceAdjustmentID, preserveConfirmedWithdrawalArgs...).
		Set(preserveConfirmedSourceRequestChildID, preserveConfirmedWithdrawalArgs...).
		Set(preserveConfirmedWithdrawalConfirmedBy, preserveConfirmedWithdrawalArgs...).
		Set(preserveConfirmedWithdrawalConfirmedRole, preserveConfirmedWithdrawalArgs...).
		Set(preserveConfirmedWithdrawalConfirmedAt, preserveConfirmedWithdrawalArgs...).
		Set(preserveConfirmedSourceOfferings, preserveConfirmedWithdrawalArgs...).
		Set("updated_at = NOW()").
		Returning("id, created_at, updated_at").
		Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		if isUniqueViolation(err) {
			stats.Conflicts = 1
		}
		return domain.WithdrawalCompletion{}, stats, fmt.Errorf("care plan postgres: upsert pending care withdrawal completion: %w", err)
	}
	stats.Rows = 1
	return withdrawalCompletionToDomain(row), stats, nil
}

// FindWithdrawalCompletion reads one task of the tenant, optionally locking it.
func (s *Store) FindWithdrawalCompletion(ctx context.Context, id int64, lock bool) (domain.WithdrawalCompletion, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.WithdrawalCompletion{}, false, domain.OperationStats{}, err
	}
	row := new(withdrawalCompletionRow)
	query := withTenant(db.NewSelect().Model(row).
		ModelTableExpr(tableExprWithdrawalCompletions).
		Where(`"care_withdrawal_completion".id = ?`, id), "care_withdrawal_completion", tenantID)
	operation := "find care withdrawal completion"
	if lock {
		query = query.For("UPDATE")
		operation = "find care withdrawal completion for update"
	}
	found, stats, err := scanOne(ctx, query, operation)
	if err != nil || !found {
		return domain.WithdrawalCompletion{}, found, stats, err
	}
	return withdrawalCompletionToDomain(*row), true, stats, nil
}

// ListWithdrawalStudentIDs returns the distinct children named by the tasks in
// state; the list queries hydrate exactly these children.
func (s *Store) ListWithdrawalStudentIDs(ctx context.Context, state string, studentID int64) ([]int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	ids := make([]int64, 0)
	query := withTenant(db.NewSelect().
		TableExpr(tableExprWithdrawalCompletions).
		ColumnExpr(`DISTINCT "care_withdrawal_completion".student_id`).
		Where(`"care_withdrawal_completion".state = ?`, state).
		Where(`"care_withdrawal_completion".student_id IS NOT NULL`), "care_withdrawal_completion", tenantID)
	if studentID > 0 {
		query = query.Where(`"care_withdrawal_completion".student_id = ?`, studentID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.OrderExpr(`"care_withdrawal_completion".student_id ASC`).Scan(ctx, &ids)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("care plan postgres: list care withdrawal student ids: %w", err)
	}
	stats.Rows = int64(len(ids))
	return ids, stats, nil
}

// ListWithdrawals is the joined read model of the pending and the resolved
// queue. Pending tasks require a directory row with a person, as the former
// inner joins did; resolved tasks stay visible without one.
func (s *Store) ListWithdrawals(ctx context.Context, state string, filter domain.WithdrawalListFilter) ([]domain.WithdrawalCompletion, int, domain.OperationStats, error) {
	if state != careplan.WithdrawalStatePending && state != careplan.WithdrawalStateResolved {
		return nil, 0, domain.OperationStats{}, fmt.Errorf("care plan postgres: unsupported withdrawal list state %q", state)
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, 0, domain.OperationStats{}, err
	}
	students := filter.Students
	if students == nil {
		students = []careplan.WithdrawalStudent{}
	}
	recordset, err := json.Marshal(students)
	if err != nil {
		return nil, 0, domain.OperationStats{}, fmt.Errorf("care plan postgres: encode withdrawal students: %w", err)
	}
	pending := state == careplan.WithdrawalStatePending
	build := func() *bun.SelectQuery {
		query := db.NewSelect().TableExpr(tableExprWithdrawalCompletions)
		if pending {
			query = query.Join(`JOIN `+withdrawalStudentRecordset+` ON "student".id = "care_withdrawal_completion".student_id AND "student".first_name IS NOT NULL`, string(recordset))
		} else {
			query = query.Join(`LEFT JOIN `+withdrawalStudentRecordset+` ON "student".id = "care_withdrawal_completion".student_id`, string(recordset))
		}
		query = withTenant(query.Where(`"care_withdrawal_completion".state = ?`, state), "care_withdrawal_completion", tenantID)
		if search := filter.Search; search != "" {
			pattern := "%" + strings.ToLower(search) + "%"
			query = query.Where(`(LOWER("student".first_name) LIKE ? OR LOWER("student".last_name) LIKE ? OR LOWER("student".school_class) LIKE ?)`, pattern, pattern, pattern)
		}
		if filter.StudentID > 0 {
			query = query.Where(`"care_withdrawal_completion".student_id = ?`, filter.StudentID)
		}
		return query
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	total, err := build().Count(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, 0, stats, fmt.Errorf("care plan postgres: count %s care withdrawal completions: %w", state, err)
	}
	rows := make([]withdrawalCompletionRow, 0)
	query := build().ColumnExpr(`"care_withdrawal_completion".*`)
	if pending {
		query = query.
			ColumnExpr(`"student".first_name AS first_name`).
			ColumnExpr(`"student".last_name AS last_name`).
			ColumnExpr(`"student".school_class AS school_class`).
			OrderExpr(`"care_withdrawal_completion".first_bookingless_day ASC, "care_withdrawal_completion".id ASC`)
		if filter.PageSize > 0 {
			query = query.Limit(filter.PageSize)
			if filter.Page > 1 {
				query = query.Offset((filter.Page - 1) * filter.PageSize)
			}
		}
	} else {
		query = query.
			ColumnExpr(`COALESCE("student".first_name, '') AS first_name`).
			ColumnExpr(`COALESCE("student".last_name, '') AS last_name`).
			ColumnExpr(`COALESCE("student".school_class, '') AS school_class`).
			OrderExpr(`"care_withdrawal_completion".resolved_at DESC, "care_withdrawal_completion".id DESC`).
			Limit(filter.PageSize).
			Offset((filter.Page - 1) * filter.PageSize)
	}
	stats.Queries++
	started = time.Now()
	err = query.Scan(ctx, &rows)
	stats.StatementDuration += time.Since(started)
	if err != nil {
		return nil, 0, stats, fmt.Errorf("care plan postgres: list %s care withdrawal completions: %w", state, err)
	}
	stats.Rows = int64(len(rows))
	values := make([]domain.WithdrawalCompletion, 0, len(rows))
	for _, row := range rows {
		values = append(values, withdrawalCompletionToDomain(row))
	}
	return values, total, stats, nil
}

func (s *Store) ListPendingWithdrawalsByStudent(ctx context.Context, studentIDs []int64) (map[int64]domain.WithdrawalCompletion, domain.OperationStats, error) {
	result := make(map[int64]domain.WithdrawalCompletion)
	if len(studentIDs) == 0 {
		return result, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := make([]withdrawalCompletionRow, 0)
	query := withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(tableExprWithdrawalCompletions).
		Where(`"care_withdrawal_completion".state = ?`, careplan.WithdrawalStatePending).
		Where(`"care_withdrawal_completion".student_id IN (?)`, bun.List(studentIDs)), "care_withdrawal_completion", tenantID)
	stats, err := scanAll(ctx, query, "list pending care withdrawal completions by students")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	for _, row := range rows {
		if row.StudentID != nil {
			result[*row.StudentID] = withdrawalCompletionToDomain(row)
		}
	}
	return result, stats, nil
}

func (s *Store) ListPendingWithdrawalStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]bool, domain.OperationStats, error) {
	result := make(map[int64]bool)
	if len(studentIDs) == 0 {
		return result, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var ids []int64
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewSelect().
		Model((*withdrawalCompletionRow)(nil)).
		ModelTableExpr(tableExprWithdrawalCompletions).
		Column("student_id").
		Where(`"care_withdrawal_completion".tenant_id = ?`, tenantID).
		Where(`"care_withdrawal_completion".state = ?`, careplan.WithdrawalStatePending).
		Where(`"care_withdrawal_completion".student_id IN (?)`, bun.List(studentIDs)).
		Scan(ctx, &ids)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("care plan postgres: list pending care withdrawal student ids: %w", err)
	}
	stats.Rows = int64(len(ids))
	for _, id := range ids {
		result[id] = true
	}
	return result, stats, nil
}

// ListPendingWithdrawalBoundaries returns the first bookingless day of each
// child's pending task. Booking-expiry tasks count only when requested.
func (s *Store) ListPendingWithdrawalBoundaries(ctx context.Context, studentIDs []int64, includeBookingExpired bool) (map[int64]domain.Date, domain.OperationStats, error) {
	result := make(map[int64]domain.Date, len(studentIDs))
	if len(studentIDs) == 0 {
		return result, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []struct {
		StudentID           int64        `bun:"student_id"`
		FirstBookinglessDay calendarDate `bun:"first_bookingless_day"`
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewSelect().
		TableExpr(tableExprWithdrawalCompletions).
		ColumnExpr(`"care_withdrawal_completion".student_id, "care_withdrawal_completion".first_bookingless_day`).
		Where(`"care_withdrawal_completion".tenant_id = ?`, tenantID).
		Where(`"care_withdrawal_completion".student_id IN (?)`, bun.List(studentIDs)).
		Where(`"care_withdrawal_completion".state = ?`, careplan.WithdrawalStatePending).
		Where(`("care_withdrawal_completion".trigger <> ? OR ?)`, careplan.WithdrawalTriggerBookingExpired, includeBookingExpired).
		Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("care plan postgres: list pending care withdrawal boundaries: %w", err)
	}
	stats.Rows = int64(len(rows))
	for _, row := range rows {
		result[row.StudentID] = careplan.Date(row.FirstBookinglessDay)
	}
	return result, stats, nil
}

// ResolveWithdrawal is a guarded pending-to-resolved transition; exactly one
// open event wins.
func (s *Store) ResolveWithdrawal(ctx context.Context, id, actorAccountID int64, at time.Time) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	stats, rows, err := execute(ctx, db.NewUpdate().
		Model((*withdrawalCompletionRow)(nil)).
		ModelTableExpr(tableExprWithdrawalCompletions).
		Set("state = ?", careplan.WithdrawalStateResolved).
		Set("outcome = ?", careplan.WithdrawalOutcomeCareEnded).
		Set("resolved_by = ?", actorAccountID).
		Set("resolved_at = ?", at).
		Set("updated_at = ?", at).
		Where(`"care_withdrawal_completion".id = ? AND "care_withdrawal_completion".state = ?`, id, careplan.WithdrawalStatePending).
		Where(`"care_withdrawal_completion".tenant_id = ?`, tenantID), "resolve care withdrawal completion")
	if err != nil {
		return false, stats, err
	}
	return rows == 1, stats, nil
}

// MarkWithdrawalObsoleteForRebooking atomically applies the no-gap predicate.
func (s *Store) MarkWithdrawalObsoleteForRebooking(ctx context.Context, studentID int64, careStartsOn domain.Date, at time.Time) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	stats, rows, err := execute(ctx, db.NewUpdate().
		Model((*withdrawalCompletionRow)(nil)).
		ModelTableExpr(tableExprWithdrawalCompletions).
		Set("state = ?", careplan.WithdrawalStateObsolete).
		Set("obsolete_reason = ?", careplan.WithdrawalObsoleteRebooked).
		Set("resolved_at = ?", at).
		Set("updated_at = ?", at).
		Where(`"care_withdrawal_completion".tenant_id = ?`, tenantID).
		Where(`"care_withdrawal_completion".student_id = ?`, studentID).
		Where(`"care_withdrawal_completion".state = ?`, careplan.WithdrawalStatePending).
		Where(`? <= "care_withdrawal_completion".first_bookingless_day`, calendarDate(careStartsOn)), "obsolete care withdrawal completion after rebooking")
	if err != nil {
		return false, stats, err
	}
	return rows == 1, stats, nil
}

// MarkPendingWithdrawalsObsoleteForWeeklyPlans closes booking-derived tasks
// when the school switches back to weekly-plan-driven care. Confirmed
// withdrawals stay.
func (s *Store) MarkPendingWithdrawalsObsoleteForWeeklyPlans(ctx context.Context, at time.Time) (int, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats, rows, err := execute(ctx, db.NewUpdate().
		Model((*withdrawalCompletionRow)(nil)).
		ModelTableExpr(tableExprWithdrawalCompletions).
		Set("state = ?", careplan.WithdrawalStateObsolete).
		Set("obsolete_reason = ?", careplan.WithdrawalObsoleteWeeklyPlans).
		Set("resolved_at = ?", at).
		Set("updated_at = ?", at).
		Where(`"care_withdrawal_completion".tenant_id = ?`, tenantID).
		Where(`"care_withdrawal_completion".state = ?`, careplan.WithdrawalStatePending).
		Where(`"care_withdrawal_completion".trigger = ?`, careplan.WithdrawalTriggerBookingExpired), "obsolete pending care withdrawal completions for weekly plans")
	if err != nil {
		return 0, stats, err
	}
	return int(rows), stats, nil
}

// ReopenWithdrawalAfterCancelledExit copies one exact resolved event into a
// new event; the historical outcome is never mutated in place.
func (s *Store) ReopenWithdrawalAfterCancelledExit(ctx context.Context, completionID, studentID int64, at time.Time) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	stats, rows, err := execute(ctx, db.NewRaw(`
		INSERT INTO users.care_withdrawal_completions (
			tenant_id, student_id, first_bookingless_day, trigger,
			source_adjustment_id, source_request_child_id, withdrawal_confirmed_by,
			withdrawal_confirmed_role, withdrawal_confirmed_at, source_offerings, state,
			created_at, updated_at
		)
		SELECT tenant_id, student_id, first_bookingless_day, trigger,
		       source_adjustment_id, source_request_child_id, withdrawal_confirmed_by,
		       withdrawal_confirmed_role, withdrawal_confirmed_at, source_offerings, 'pending', ?, ?
		FROM users.care_withdrawal_completions
		WHERE tenant_id = ? AND id = ? AND student_id = ?
		  AND state = 'resolved' AND outcome = 'care_ended'
		ON CONFLICT (tenant_id, student_id)
			WHERE state = 'pending' AND student_id IS NOT NULL
		DO UPDATE SET
			first_bookingless_day = EXCLUDED.first_bookingless_day,
			source_adjustment_id = EXCLUDED.source_adjustment_id,
			source_request_child_id = EXCLUDED.source_request_child_id,
			withdrawal_confirmed_by = EXCLUDED.withdrawal_confirmed_by,
			withdrawal_confirmed_role = EXCLUDED.withdrawal_confirmed_role,
			withdrawal_confirmed_at = EXCLUDED.withdrawal_confirmed_at,
			source_offerings = EXCLUDED.source_offerings,
			updated_at = EXCLUDED.updated_at
	`, at, at, tenantID, completionID, studentID), "reopen care withdrawal after cancelled exit")
	if err != nil {
		return false, stats, err
	}
	return rows == 1, stats, nil
}
