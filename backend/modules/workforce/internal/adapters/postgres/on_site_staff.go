package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// NewOnSiteStaff reads on the caller's administrative transaction.
func NewOnSiteStaff(transaction func(context.Context) (any, bool), admin func(context.Context) bool) *onSiteStaff {
	return &onSiteStaff{transaction: transaction, admin: admin}
}

type onSiteStaff struct {
	transaction func(context.Context) (any, bool)
	admin       func(context.Context) bool
}

const onSiteStaffIDsSQL = `SELECT DISTINCT tenant_id, staff_id FROM active.work_sessions
WHERE tenant_id = ANY(?) AND check_out_time IS NULL AND status = 'present'
  AND (date = ?::date OR check_in_time > ?)`

func (s onSiteStaff) OnSiteStaffIDsByTenant(ctx context.Context, schools []int64, at time.Time) (map[int64][]int64, error) {
	transaction, ok := s.transaction(ctx)
	if !ok {
		return nil, errors.New("workforce: transaction is required")
	}
	if !s.admin(ctx) {
		return nil, errors.New("workforce: on-site staff needs the administrative transaction")
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
		return nil, fmt.Errorf("workforce postgres: unsupported transaction %T", transaction)
	}
	var rows []struct {
		TenantID int64 `bun:"tenant_id"`
		StaffID  int64 `bun:"staff_id"`
	}
	query := db.NewRaw(onSiteStaffIDsSQL, pgdialect.Array(schools), calendar.DateFromTime(at), at.Add(-workforce.MaxOpenWorkSessionDuration))
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("workforce postgres: on-site staff: %w", err)
	}
	result := map[int64][]int64{}
	for _, row := range rows {
		result[row.TenantID] = append(result[row.TenantID], row.StaffID)
	}
	return result, nil
}
