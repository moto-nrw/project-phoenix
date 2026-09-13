package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (s *Store) FindRFIDCard(ctx context.Context, tag string, tenantID int64) (string, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return "", false, domain.OperationStats{}, err
	}
	var ids []string
	started := time.Now()
	err = db.NewRaw(`SELECT id FROM users.rfid_cards WHERE id = ? AND tenant_id = ?`, tag, tenantID).Scan(ctx, &ids)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return "", false, stats, fmt.Errorf("identity access postgres: find RFID card: %w", err)
	}
	if len(ids) == 0 {
		return "", false, stats, nil
	}
	return ids[0], true, stats, nil
}
