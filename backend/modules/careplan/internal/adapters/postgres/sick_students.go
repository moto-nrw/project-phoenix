package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// NewSickStudents reads on the caller's administrative transaction.
func NewSickStudents(transaction func(context.Context) (any, bool), admin func(context.Context) bool) *sickStudents {
	return &sickStudents{transaction: transaction, admin: admin}
}

type sickStudents struct {
	transaction func(context.Context) (any, bool)
	admin       func(context.Context) bool
}

const sickDayStudentIDsSQL = `SELECT DISTINCT tenant_id, student_id AS id FROM active.student_status_days
WHERE tenant_id = ANY(?) AND date = ?::date AND status = 'sick' AND cleared_at IS NULL`

const sickFlagMembershipIDsSQL = `SELECT tenant_id, membership_id AS id FROM users.student_care_profiles
WHERE tenant_id = ANY(?) AND sick`

type sickStudentRow struct {
	TenantID int64 `bun:"tenant_id"`
	ID       int64 `bun:"id"`
}

func (s sickStudents) database(ctx context.Context) (bun.IDB, error) {
	transaction, ok := s.transaction(ctx)
	if !ok {
		return nil, errors.New("care plan: transaction is required")
	}
	if !s.admin(ctx) {
		return nil, errors.New("care plan: sick students need the administrative transaction")
	}
	switch tx := transaction.(type) {
	case bun.Tx:
		return tx, nil
	case *bun.Tx:
		if tx != nil {
			return *tx, nil
		}
	}
	return nil, fmt.Errorf("care plan postgres: unsupported transaction %T", transaction)
}

func idsByTenant(rows []sickStudentRow) map[int64][]int64 {
	result := map[int64][]int64{}
	for _, row := range rows {
		result[row.TenantID] = append(result[row.TenantID], row.ID)
	}
	return result
}

func (s sickStudents) SickDayStudentIDsByTenant(ctx context.Context, schools []int64, at time.Time) (map[int64][]int64, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []sickStudentRow
	if err := db.NewRaw(sickDayStudentIDsSQL, pgdialect.Array(schools), calendar.DateFromTime(at)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care plan postgres: sick days: %w", err)
	}
	return idsByTenant(rows), nil
}

func (s sickStudents) SickFlagMembershipIDsByTenant(ctx context.Context, schools []int64) (map[int64][]int64, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []sickStudentRow
	if err := db.NewRaw(sickFlagMembershipIDsSQL, pgdialect.Array(schools)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care plan postgres: sick flags: %w", err)
	}
	return idsByTenant(rows), nil
}
