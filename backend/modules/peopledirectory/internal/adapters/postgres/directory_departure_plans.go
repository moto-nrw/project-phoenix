package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/uptrace/bun"
)

func (s *studentDirectoryReads) ListDepartureModes(ctx context.Context, ids []int64) (map[int64]map[string][]string, directoryStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, directoryStats{}, err
	}
	if scope.TenantID <= 0 {
		return nil, directoryStats{}, errors.New("people directory: departure modes require a tenant")
	}
	var rows []struct {
		ID                    int64                        `bun:"id"`
		AllowedDepartureModes domain.AllowedDepartureModes `bun:"allowed_departure_modes"`
		DepartureDays         domain.DepartureDays         `bun:"departure_days"`
		BusDays               domain.BusDays               `bun:"bus_days"`
		PickupDays            domain.PickupDays            `bun:"pickup_days"`
	}
	stats := directoryStats{Queries: 1}
	started := time.Now()
	err = s.studentRows(db, scope).
		ColumnExpr(`"student".id, "student".allowed_departure_modes, "student".departure_days, "student".bus_days, "student".pickup_days`).
		Where(`"student".tenant_id = ?`, scope.TenantID).
		Where(`"student".id IN (?)`, bun.List(ids)).
		Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("list student departure modes: %w", err)
	}
	stats.Rows = int64(len(rows))
	result := make(map[int64]map[string][]string, len(rows))
	for _, row := range rows {
		// Resolve through the owner's read precedence, as every other plan
		// read does: a row whose mode set is empty still answers from its
		// legacy columns (#6990).
		plan := domain.DeparturePlan{
			AllowedDepartureModes: row.AllowedDepartureModes, DepartureDays: row.DepartureDays,
			BusDays: row.BusDays, PickupDays: row.PickupDays,
		}.Effective()
		modes := make(map[string][]string, len(plan.AllowedDepartureModes))
		for day, values := range plan.AllowedDepartureModes {
			for _, mode := range values {
				modes[day] = append(modes[day], string(mode))
			}
		}
		result[row.ID] = modes
	}
	return result, stats, nil
}
