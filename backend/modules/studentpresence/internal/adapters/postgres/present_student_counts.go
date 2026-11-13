package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// NewPresentStudents reads on the caller's administrative transaction.
func NewPresentStudents(transaction func(context.Context) (any, bool), admin func(context.Context) bool) *presentStudents {
	return &presentStudents{transaction: transaction, admin: admin}
}

type presentStudents struct {
	transaction func(context.Context) (any, bool)
	admin       func(context.Context) bool
}

const presentStudentIDsSQL = `SELECT tenant_id, student_id FROM (
  SELECT tenant_id, student_id FROM active.attendance
  WHERE tenant_id = ANY(?0) AND date = ?1::date AND check_out_time IS NULL
  UNION
  SELECT v.tenant_id, v.student_id FROM active.visits AS v
  JOIN active.groups AS g ON g.id = v.active_group_id AND g.tenant_id = v.tenant_id
  WHERE v.tenant_id = ANY(?0) AND v.exit_time IS NULL AND g.end_time IS NULL
) AS present`

func (c presentStudents) PresentStudentIDsByTenant(ctx context.Context, schools []int64, at time.Time) (map[int64][]int64, error) {
	transaction, ok := c.transaction(ctx)
	if !ok {
		return nil, errors.New("student presence: transaction is required")
	}
	if !c.admin(ctx) {
		return nil, errors.New("student presence: present student counts need the administrative transaction")
	}
	var db bun.IDB
	switch tx := transaction.(type) {
	case bun.Tx:
		db = tx
	case *bun.Tx:
		if tx != nil {
			db = *tx
		}
	}
	if db == nil {
		return nil, fmt.Errorf("student presence postgres: unsupported transaction %T", transaction)
	}
	var rows []struct {
		TenantID  int64 `bun:"tenant_id"`
		StudentID int64 `bun:"student_id"`
	}
	if err := db.NewRaw(presentStudentIDsSQL, pgdialect.Array(schools), calendar.DateFromTime(at)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("student presence postgres: present students: %w", err)
	}
	present := make(map[int64][]int64, len(rows))
	for _, row := range rows {
		present[row.TenantID] = append(present[row.TenantID], row.StudentID)
	}
	return present, nil
}
