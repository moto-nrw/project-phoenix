package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Store) ListOpenVisitStudentIDs(ctx context.Context, hostingTenantID int64) ([]int64, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	if hostingTenantID <= 0 || hostingTenantID != tenantID {
		return nil, ports.Stats{}, fmt.Errorf("cross-tenant visitors require the hosting tenant context")
	}
	var ids []int64
	started := time.Now()
	err = db.NewSelect().Table("active.visits").Column("student_id").Distinct().
		Where("tenant_id = ?", tenantID).Where("exit_time IS NULL").Order("student_id").Scan(ctx, &ids)
	stats := ports.Stats{Queries: 1, Rows: int64(len(ids)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("find open visit students: %w", err)
	}
	return ids, stats, nil
}
