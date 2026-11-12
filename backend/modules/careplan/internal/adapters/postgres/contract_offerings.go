package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// ContractOfferingStore reads the offerings for the Betreuungsvertrag
// (#4678) in the caller's tenant transaction.
type ContractOfferingStore struct{}

var _ careplan.ContractOfferings = ContractOfferingStore{}

// NewContractOfferingStore builds the store.
func NewContractOfferingStore() ContractOfferingStore { return ContractOfferingStore{} }

type contractOfferingRow struct {
	ID             int64    `bun:"id"`
	PhaseID        int64    `bun:"phase_id"`
	Name           string   `bun:"name"`
	PriceCents     *int     `bun:"price_cents"`
	IsActive       bool     `bun:"is_active"`
	PickupTimes    string   `bun:"pickup_times"`
	DaysOfWeekMode string   `bun:"days_of_week_mode"`
	AvailableDays  []string `bun:"available_days,type:jsonb"`
}

// ContractOfferings lists the offerings of the school.
func (ContractOfferingStore) ContractOfferings(ctx context.Context, ids []int64) ([]careplan.ContractOffering, error) {
	tx, tenantID, err := tenant.CurrentTenantTransaction[bun.Tx](ctx)
	if err != nil {
		return nil, fmt.Errorf("care plan postgres: contract offerings need the school transaction: %w", err)
	}
	if ids == nil {
		ids = []int64{}
	}
	var rows []contractOfferingRow
	if err := tx.NewRaw(`SELECT id, phase_id, name, price_cents, is_active, days_of_week_mode, available_days,
			COALESCE(pickup_times, '{}'::jsonb)::text AS pickup_times FROM enrollment.care_offerings
		WHERE tenant_id = ? AND counts_as_care AND (cardinality(?::bigint[]) = 0 OR id = ANY(?::bigint[]))
		ORDER BY sort_order, name, id`, tenantID, pgdialect.Array(ids), pgdialect.Array(ids)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care plan postgres: contract offerings: %w", err)
	}
	result := make([]careplan.ContractOffering, len(rows))
	for i, row := range rows {
		result[i] = careplan.ContractOffering{
			ID: row.ID, PhaseID: row.PhaseID, Name: row.Name, PriceCents: row.PriceCents, IsActive: row.IsActive,
			PickupTimes:    contractPickupTimes(row.PickupTimes),
			DaysOfWeekMode: row.DaysOfWeekMode, AvailableDays: row.AvailableDays,
		}
	}
	return result, nil
}

// contractPickupTimes keeps the weekdays whose end time reads as a clock
// time, as "HH:MM"; anything else has no end time.
func contractPickupTimes(raw string) map[string]string {
	var stored map[string]any
	times := map[string]string{}
	if json.Unmarshal([]byte(raw), &stored) != nil {
		return times
	}
	for day, value := range stored {
		text, ok := value.(string)
		if !ok {
			continue
		}
		if clock, err := time.Parse("15:04", strings.TrimSpace(text)); err == nil {
			times[day] = clock.Format("15:04")
		} else if clock, err := time.Parse("15:04:05", strings.TrimSpace(text)); err == nil {
			times[day] = clock.Format("15:04")
		}
	}
	return times
}
