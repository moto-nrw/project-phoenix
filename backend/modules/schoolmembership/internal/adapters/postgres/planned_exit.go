package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/adapters/postgres/calendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
	"github.com/uptrace/bun"
)

// plannedExitDetails is the planned exit half of a live membership (#4809)
// beside its day, which staffRow names itself. The columns live on
// users.staff_school_memberships, every membership read scans them, and only
// the commands below write them; the retirement clears them.
type plannedExitDetails struct {
	PlannedExitByAccountID  *int64     `bun:"planned_exit_by_account_id,scanonly"`
	PlannedExitByStaffID    *int64     `bun:"planned_exit_by_staff_id,scanonly"`
	PlannedExitByName       *string    `bun:"planned_exit_by_name,scanonly"`
	PlannedExitAt           *time.Time `bun:"planned_exit_at,scanonly"`
	ExitWeekNoticeSentAt    *time.Time `bun:"exit_week_notice_sent_at,scanonly"`
	ExitDayNoticeSentAt     *time.Time `bun:"exit_day_notice_sent_at,scanonly"`
	ExitBlockedNoticeSentAt *time.Time `bun:"exit_blocked_notice_sent_at,scanonly"`
	ExitAccessLockedAt      *time.Time `bun:"exit_access_locked_at,scanonly"`
	ExitNextAttemptAt       *time.Time `bun:"exit_next_attempt_at,scanonly"`
}

// plannedExitProgressColumns are the scheduler's progress columns; a moved
// day clears them.
var plannedExitProgressColumns = []string{
	"exit_week_notice_sent_at", "exit_day_notice_sent_at", "exit_blocked_notice_sent_at",
	"exit_access_locked_at", "exit_next_attempt_at",
}

// SavePlannedExit writes the planned exit of the live member the caller
// locked.
func (s *Store) SavePlannedExit(ctx context.Context, plan schoolmembership.PlanExit, keepProgress bool) (schoolmembership.Staff, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return schoolmembership.Staff{}, domain.OperationStats{}, err
	}
	day, err := calendar.ParseDate(plan.LastWorkingDay)
	if err != nil {
		return schoolmembership.Staff{}, domain.OperationStats{}, fmt.Errorf("school membership postgres: parse last working day: %w", err)
	}
	row := staffRow{}
	query := db.NewUpdate().Model(&row).ModelTableExpr(`users.staff_school_memberships AS "staff"`).
		Set(`planned_exit_day = ?`, day).
		Set(`planned_exit_by_account_id = ?`, plan.PlannedByAccountID).
		Set(`planned_exit_by_staff_id = ?`, plan.PlannedByStaffID).
		Set(`planned_exit_by_name = ?`, plan.PlannedByName).
		Set(`planned_exit_at = NOW()`)
	if !keepProgress {
		query = setNull(query, plannedExitProgressColumns)
	}
	query = withTenant(query.Where(`"staff".id = ?`, plan.StaffID).Where(`"staff".deleted_at IS NULL`), "staff", scope)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return schoolmembership.Staff{}, stats, schoolmembership.ErrStaffNotFound
	}
	if err != nil {
		return schoolmembership.Staff{}, stats, fmt.Errorf("school membership postgres: save planned exit: %w", err)
	}
	stats.Rows = 1
	return staffToPublic(row), stats, nil
}

// ClearPlannedExit removes the planned exit of the live member the caller
// locked.
func (s *Store) ClearPlannedExit(ctx context.Context, staffID int64) (domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(clearPlannedExit(db.NewUpdate().Model((*staffRow)(nil)).
		ModelTableExpr(`users.staff_school_memberships AS "staff"`)).
		Where(`"staff".id = ?`, staffID).
		Where(`"staff".deleted_at IS NULL`), "staff", scope)
	return execOne(ctx, query, "clear planned exit", schoolmembership.ErrStaffNotFound)
}

// clearPlannedExit sets every planned exit column back to NULL; the
// retirement uses it as well, so no planned exit outlives the membership.
func clearPlannedExit(query *bun.UpdateQuery) *bun.UpdateQuery {
	return setNull(query, append([]string{
		"planned_exit_day", "planned_exit_by_account_id", "planned_exit_by_staff_id", "planned_exit_by_name", "planned_exit_at",
	}, plannedExitProgressColumns...))
}

func setNull(query *bun.UpdateQuery, columns []string) *bun.UpdateQuery {
	for _, column := range columns {
		query = query.Set(`? = NULL`, bun.Ident(column))
	}
	return query
}

// RecordPlannedExitProgress writes the scheduler's progress on the live
// member the caller locked.
func (s *Store) RecordPlannedExitProgress(ctx context.Context, progress schoolmembership.PlannedExitProgress) (domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := db.NewUpdate().Model((*staffRow)(nil)).
		ModelTableExpr(`users.staff_school_memberships AS "staff"`).
		Set(`exit_next_attempt_at = ?`, progress.NextAttemptAt).
		Where(`"staff".id = ?`, progress.StaffID).
		Where(`"staff".planned_exit_day IS NOT NULL`).
		Where(`"staff".deleted_at IS NULL`)
	for column, value := range map[string]*time.Time{
		"exit_week_notice_sent_at":    progress.WeekNoticeSentAt,
		"exit_day_notice_sent_at":     progress.DayNoticeSentAt,
		"exit_blocked_notice_sent_at": progress.BlockedNoticeSentAt,
		"exit_access_locked_at":       progress.AccessLockedAt,
	} {
		if value != nil {
			query = query.Set(`? = ?`, bun.Ident(column), *value)
		}
	}
	return execOne(ctx, withTenant(query, "staff", scope), "record planned exit progress", schoolmembership.ErrPlannedExitNotFound)
}

// plannedExitOf maps the planned exit columns of a membership row; nil
// without a planned exit.
func plannedExitOf(row staffRow) *schoolmembership.PlannedExit {
	if row.PlannedExitDay == nil {
		return nil
	}
	return &schoolmembership.PlannedExit{
		StaffID: row.ID, TenantID: row.TenantID, LastWorkingDay: row.PlannedExitDay.String(),
		PlannedByAccountID: deref(row.PlannedExitByAccountID), PlannedByStaffID: deref(row.PlannedExitByStaffID),
		PlannedByName: deref(row.PlannedExitByName), PlannedAt: deref(row.PlannedExitAt),
		WeekNoticeSentAt: row.ExitWeekNoticeSentAt, DayNoticeSentAt: row.ExitDayNoticeSentAt,
		BlockedNoticeSentAt: row.ExitBlockedNoticeSentAt, AccessLockedAt: row.ExitAccessLockedAt,
		NextAttemptAt: row.ExitNextAttemptAt,
	}
}

func deref[T any](value *T) T {
	var zero T
	if value == nil {
		return zero
	}
	return *value
}
