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

// NewPresentStudentCounts reads on the caller's administrative transaction.
func NewPresentStudentCounts(transaction func(context.Context) (any, bool), admin func(context.Context) bool) *presentStudentCounts {
	return &presentStudentCounts{transaction: transaction, admin: admin}
}

type presentStudentCounts struct {
	transaction func(context.Context) (any, bool)
	admin       func(context.Context) bool
}

const countPresentStudentsSQL = `SELECT tenant_id, COUNT(*)::int AS count FROM (
  SELECT tenant_id, student_id FROM active.attendance
  WHERE tenant_id = ANY(?0) AND date = ?1::date AND check_out_time IS NULL
  UNION
  SELECT v.tenant_id, v.student_id FROM active.visits AS v
  JOIN active.groups AS g ON g.id = v.active_group_id AND g.tenant_id = v.tenant_id
  WHERE v.tenant_id = ANY(?0) AND v.exit_time IS NULL AND g.end_time IS NULL
) AS present GROUP BY tenant_id`

func (c presentStudentCounts) CountPresentStudentsByTenant(ctx context.Context, schools []int64, at time.Time) (map[int64]int, error) {
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
		TenantID int64 `bun:"tenant_id"`
		Count    int   `bun:"count"`
	}
	if err := db.NewRaw(countPresentStudentsSQL, pgdialect.Array(schools), calendar.DateFromTime(at)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("student presence postgres: count present students: %w", err)
	}
	counts := make(map[int64]int, len(rows))
	for _, row := range rows {
		counts[row.TenantID] = row.Count
	}
	return counts, nil
}
