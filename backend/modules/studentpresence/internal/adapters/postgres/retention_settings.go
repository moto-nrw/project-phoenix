package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Store) ListAcceptedRetentionSettings(ctx context.Context) ([]ports.StudentRetentionSetting, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	var rows []ports.StudentRetentionSetting
	started := time.Now()
	err = db.NewSelect().Table("users.privacy_consents").Column("student_id", "data_retention_days").
		Where("tenant_id = ?", tenantID).Where("accepted = TRUE").Distinct().Order("student_id", "data_retention_days").Scan(ctx, &rows)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("list accepted retention settings: %w", err)
	}
	return rows, stats, nil
}
