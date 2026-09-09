package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Store) LockStaffSupervision(ctx context.Context, staffID int64, date ports.Date) ([]int64, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	var ids []int64
	started := time.Now()
	err = db.NewSelect().TableExpr(`active.group_supervisors AS "group_supervisor"`).
		ColumnExpr(`"group_supervisor".id`).
		Where(`"group_supervisor".tenant_id = ?`, tenantID).
		Where(`"group_supervisor".staff_id = ?`, staffID).
		Where(`"group_supervisor".start_date <= ?`, date).
		Where(`"group_supervisor".end_date IS NULL OR "group_supervisor".end_date > ?`, date).
		OrderExpr(`"group_supervisor".id ASC`).For("UPDATE").Scan(ctx, &ids)
	stats := ports.Stats{Queries: 1, Rows: int64(len(ids)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("lock staff supervision: %w", err)
	}
	return ids, stats, nil
}
