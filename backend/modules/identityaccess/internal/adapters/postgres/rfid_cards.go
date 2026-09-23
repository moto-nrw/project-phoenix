package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (s *Store) LookupRFIDCard(ctx context.Context, tag string, tenantID int64) (domain.RFIDCard, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.RFIDCard{}, false, domain.OperationStats{}, err
	}
	var rows []struct {
		ID     string
		Active bool
	}
	started := time.Now()
	err = db.NewRaw(`SELECT id, active FROM users.rfid_cards WHERE id = ? AND tenant_id = ?`, tag, tenantID).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return domain.RFIDCard{}, false, stats, fmt.Errorf("identity access postgres: lookup RFID card: %w", err)
	}
	if len(rows) == 0 {
		return domain.RFIDCard{}, false, stats, nil
	}
	return domain.RFIDCard{ID: rows[0].ID, Active: rows[0].Active}, true, stats, nil
}

func (s *Store) RegisterRFIDCard(ctx context.Context, card domain.RFIDCard, tenantID int64) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewRaw(`INSERT INTO users.rfid_cards (id, active, tenant_id) VALUES (?, ?, ?)`, card.ID, card.Active, tenantID).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: register RFID card: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}
