package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/uptrace/bun"
)

// SaveTraegerOfferingRows persists the complete, validated copy set. Inserts,
// updates, trigger links and deletes each use one statement per operation.
func (s *Store) SaveTraegerOfferingRows(ctx context.Context, writes []careplan.TraegerOfferingWrite, deleteIDs []int64) ([]careplan.CareOffering, domain.OperationStats, error) {
	db, school, err := s.databaseForWrite(ctx, "save träger offering batch")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	started := time.Now()
	stats := domain.OperationStats{}
	inserts, updates, byID, bySource, err := prepareTraegerOfferingRows(writes, school)
	if err != nil {
		return nil, stats, err
	}
	saved, err := saveTraegerOfferingRows(ctx, db, school, inserts, updates, &stats)
	if err != nil {
		return nil, stats, err
	}
	result, err := replaceTraegerOfferingTriggers(ctx, db, school, saved, byID, bySource, &stats)
	if err != nil {
		return nil, stats, err
	}
	if len(deleteIDs) > 0 {
		stats.Queries++
		if _, err = db.NewRaw(`DELETE FROM enrollment.care_offerings WHERE tenant_id = ? AND id IN (?)`, school, bun.List(deleteIDs)).Exec(ctx); err != nil {
			return nil, stats, err
		}
	}
	stats.StatementDuration = time.Since(started)
	stats.Rows = int64(len(saved) + len(deleteIDs))
	return result, stats, nil
}

func prepareTraegerOfferingRows(writes []careplan.TraegerOfferingWrite, school int64) ([]careOfferingRow, []careOfferingRow, map[int64][]int64, map[int64][]int64, error) {
	var err error
	inserts, updates := []careOfferingRow{}, []careOfferingRow{}
	byID := map[int64][]int64{}
	bySource := map[int64][]int64{}
	for _, write := range writes {
		row := careOfferingRow{ID: write.Offering.ID, TenantID: school}
		applyCareOfferingFields(&row, offeringWriteFields(write.Offering))
		if !write.Unlink {
			source, version := write.SourceID, write.Version
			row.TraegerSourceID, row.TraegerSourceVersion = &source, &version
			row.TraegerValues, err = json.Marshal(struct {
				careplan.TraegerOfferingFields
				IsActive bool `json:"is_active"`
			}{write.Baseline, true})
			if err != nil {
				return nil, nil, nil, nil, err
			}
		}
		if row.ID == 0 {
			inserts = append(inserts, row)
			bySource[write.SourceID] = write.Offering.AutoAddTriggerOfferingIDs
		} else {
			updates = append(updates, row)
			byID[row.ID] = write.Offering.AutoAddTriggerOfferingIDs
		}
	}
	return inserts, updates, byID, bySource, nil
}

func saveTraegerOfferingRows(ctx context.Context, db bun.IDB, school int64, inserts, updates []careOfferingRow, stats *domain.OperationStats) ([]careOfferingRow, error) {
	var err error
	saved := []careOfferingRow{}
	if len(inserts) > 0 {
		stats.Queries++
		if err = db.NewInsert().Model(&inserts).ModelTableExpr(`enrollment.care_offerings`).Returning("*").Scan(ctx); err != nil {
			return nil, fmt.Errorf("insert träger offering copies: %w", err)
		}
		saved = append(saved, inserts...)
	}
	if len(updates) > 0 {
		encoded, encodeErr := json.Marshal(updates)
		if encodeErr != nil {
			return nil, encodeErr
		}
		var rows []careOfferingRow
		stats.Queries++
		err = db.NewRaw(`UPDATE enrollment.care_offerings AS target SET
   phase_id=batch.phase_id,activity_group_id=batch.activity_group_id,name=batch.name,description=batch.description,
   days_of_week_mode=batch.days_of_week_mode,available_days=batch.available_days,includes_holiday_care=batch.includes_holiday_care,
   includes_lunch=batch.includes_lunch,capacity=batch.capacity,price_cents=batch.price_cents,is_active=batch.is_active,
   is_required=batch.is_required,counts_as_care=batch.counts_as_care,auto_add_grade_levels=COALESCE(batch.auto_add_grade_levels,'[]'::jsonb),
   availability_rule=batch.availability_rule,sort_order=batch.sort_order,selection_group=batch.selection_group,
   selection_rule=batch.selection_rule,pickup_times=batch.pickup_times,translations=COALESCE(batch.translations,'null'::jsonb),
   traeger_source_id=batch.traeger_source_id,traeger_source_version=batch.traeger_source_version,
   traeger_values=batch.traeger_values,updated_at=NOW()
   FROM jsonb_populate_recordset(NULL::enrollment.care_offerings,?::jsonb) AS batch
   WHERE target.tenant_id = ? AND batch.tenant_id = ? AND target.id=batch.id RETURNING target.*`, string(encoded), school, school).Scan(ctx, &rows)
		if err != nil {
			return nil, fmt.Errorf("update träger offering copies: %w", err)
		}
		if len(rows) != len(updates) {
			return nil, careplan.ErrCareOfferingNotFound
		}
		saved = append(saved, rows...)
	}
	return saved, nil
}

func replaceTraegerOfferingTriggers(ctx context.Context, db bun.IDB, school int64, saved []careOfferingRow, byID, bySource map[int64][]int64, stats *domain.OperationStats) ([]careplan.CareOffering, error) {
	var err error
	result := make([]careplan.CareOffering, 0, len(saved))
	ids := make([]int64, 0, len(saved))
	triggers := []autoTriggerRow{}
	for _, row := range saved {
		value := careOfferingToDomain(row)
		links := byID[row.ID]
		if row.TraegerSourceID != nil && links == nil {
			links = bySource[*row.TraegerSourceID]
		}
		value.AutoAddTriggerOfferingIDs = links
		result = append(result, value)
		ids = append(ids, row.ID)
		for _, trigger := range links {
			triggers = append(triggers, autoTriggerRow{TenantID: school, TargetCareOfferingID: row.ID, TriggerCareOfferingID: trigger})
		}
	}
	if len(ids) > 0 {
		stats.Queries++
		if _, err = db.NewRaw(`DELETE FROM enrollment.care_offering_auto_triggers WHERE tenant_id = ? AND target_care_offering_id IN (?)`, school, bun.List(ids)).Exec(ctx); err != nil {
			return nil, err
		}
	}
	if len(triggers) > 0 {
		stats.Queries++
		if _, err = db.NewInsert().Model(&triggers).ModelTableExpr(`enrollment.care_offering_auto_triggers`).Exec(ctx); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func offeringWriteFields(o careplan.CareOffering) careplan.CareOfferingFields {
	return careplan.CareOfferingFields{PhaseID: o.PhaseID, ActivityGroupID: o.ActivityGroupID, Name: o.Name, Description: o.Description,
		DaysOfWeekMode: o.DaysOfWeekMode, AvailableDays: o.AvailableDays, IncludesHolidayCare: o.IncludesHolidayCare,
		IncludesLunch: o.IncludesLunch, Capacity: o.Capacity, PriceCents: o.PriceCents, IsActive: o.IsActive, IsRequired: o.IsRequired,
		CountsAsCare: o.CountsAsCare, AutoAddGradeLevels: o.AutoAddGradeLevels, AvailabilityRule: o.AvailabilityRule, SortOrder: o.SortOrder,
		SelectionGroup: o.SelectionGroup, SelectionRule: o.SelectionRule, PickupTimes: o.PickupTimes, Translations: o.Translations, AutoAddTriggerOfferingIDs: o.AutoAddTriggerOfferingIDs}
}

// BookedOfferingIDs reports every FK-protected booking, including ended ones.
func (s *Store) BookedOfferingIDs(ctx context.Context, ids []int64) (map[int64]bool, error) {
	db, school, err := s.databaseForWrite(ctx, "read offering booking references")
	if err != nil {
		return nil, err
	}
	result := map[int64]bool{}
	if len(ids) == 0 {
		return result, nil
	}
	var found []int64
	if err = db.NewRaw(`SELECT DISTINCT care_offering_id FROM enrollment.care_offering_bookings WHERE tenant_id = ? AND care_offering_id IN (?)`, school, bun.List(ids)).Scan(ctx, &found); err != nil {
		return nil, err
	}
	for _, id := range found {
		result[id] = true
	}
	return result, nil
}
