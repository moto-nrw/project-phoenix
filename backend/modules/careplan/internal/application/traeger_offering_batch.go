package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

type traegerBatchEffects interface {
	ResyncTemplatesSourcedFromOfferings(context.Context, []int64, calendar.Date) error
	DetachTemplatesSourcedFromOfferings(context.Context, []int64, calendar.Date) error
}
type traegerBatchPickup interface {
	ReconcileOfferingPickupForOfferings(context.Context, []int64) error
}
type traegerBatchBookings interface {
	ReferencedOfferingIDs(context.Context, []int64) ([]int64, error)
	CatalogMaterializable(context.Context, []int64, calendar.Date) (map[int64]int, error)
}

type traegerOfferingBatch struct {
	writes    []careplan.TraegerOfferingWrite
	deleteIDs []int64
	applied   map[int64]careplan.TraegerOfferingApplied
	withdrawn map[int64]careplan.TraegerOfferingWithdrawal
	final     []careplan.CareOffering
}

// ApplyTraegerOfferings merges the entire school catalog before checking a
// selection group. Reads, copy persistence and derived effects take ID sets.
func (c *CareOfferingCatalog) ApplyTraegerOfferings(ctx context.Context, copies []careplan.TraegerOfferingCopy, withdraw []int64) (careplan.TraegerOfferingBatchApplied, error) {
	empty := careplan.TraegerOfferingBatchApplied{}
	if c.deps.CopyReferences == nil || c.copyWrites == nil {
		return empty, errors.New("catalog batch: copy references are required")
	}
	if err := c.lockTemplateRecurrence(ctx); err != nil {
		return empty, err
	}
	sources := append([]int64(nil), withdraw...)
	for _, copy := range copies {
		sources = append(sources, copy.SourceID)
	}
	if _, err := c.deps.CopyReferences.TraegerOfferingCopyStates(ctx, sources, true); err != nil {
		return empty, err
	}
	current, err := c.listTenantRecords(ctx)
	if err != nil {
		return empty, err
	}
	batch, err := c.mergeTraegerBatch(ctx, current, copies, withdraw)
	if err != nil {
		return empty, err
	}
	if err = c.validateTraegerBatch(ctx, batch); err != nil {
		return empty, err
	}
	saved, err := c.copyWrites.SaveTraegerOfferingRows(ctx, batch.writes, nil)
	if err != nil {
		return empty, err
	}
	if err = c.resyncTraegerBatch(ctx, saved, batch.deleteIDs); err != nil {
		return empty, err
	}
	if len(batch.deleteIDs) > 0 {
		if _, err = c.copyWrites.SaveTraegerOfferingRows(ctx, nil, batch.deleteIDs); err != nil {
			return empty, err
		}
	}
	return batch.result(saved), nil
}

func (batch traegerOfferingBatch) result(saved []careplan.CareOffering) careplan.TraegerOfferingBatchApplied {
	result := careplan.TraegerOfferingBatchApplied{Applied: []careplan.TraegerOfferingApplied{}, Withdrawn: batch.withdrawn}
	for _, row := range saved {
		if row.Traeger == nil {
			continue
		}
		applied := batch.applied[row.Traeger.SourceID]
		applied.CopyID = row.ID
		result.Applied = append(result.Applied, applied)
	}
	return result
}

func (c *CareOfferingCatalog) mergeTraegerBatch(ctx context.Context, current []careplan.CareOffering, copies []careplan.TraegerOfferingCopy, withdraw []int64) (traegerOfferingBatch, error) {
	batch := traegerOfferingBatch{applied: map[int64]careplan.TraegerOfferingApplied{}, withdrawn: map[int64]careplan.TraegerOfferingWithdrawal{}}
	bySource := map[int64]careplan.CareOffering{}
	for _, row := range current {
		if row.Traeger != nil {
			bySource[row.Traeger.SourceID] = row
		}
	}
	changed := map[int64]careplan.CareOffering{}
	seen := map[int64]bool{}
	if err := batch.mergeCopies(copies, bySource, changed, seen); err != nil {
		return batch, err
	}
	ids := []int64{}
	for _, source := range withdraw {
		if seen[source] {
			return batch, careOfferingInvalidf("träger offering applied and withdrawn together")
		}
		seen[source] = true
		if row, ok := bySource[source]; ok {
			ids = append(ids, row.ID)
		}
	}
	refs, err := c.traegerWithdrawalReferences(ctx, ids)
	if err != nil {
		return batch, err
	}
	return finishTraegerWithdrawals(batch, current, withdraw, bySource, changed, refs), nil
}

func (batch *traegerOfferingBatch) mergeCopies(copies []careplan.TraegerOfferingCopy, bySource, changed map[int64]careplan.CareOffering, seen map[int64]bool) error {
	for _, copy := range copies {
		if copy.SourceID <= 0 || copy.Version <= 0 || copy.PhaseID <= 0 || seen[copy.SourceID] {
			return careOfferingInvalidf("invalid or repeated träger offering")
		}
		seen[copy.SourceID] = true
		currentRow, exists := bySource[copy.SourceID]
		row, applied, err := mergeTraegerCopy(copy, currentRow, exists)
		if err != nil {
			return err
		}
		batch.writes = append(batch.writes, careplan.TraegerOfferingWrite{Offering: row, SourceID: copy.SourceID, Version: copy.Version, Baseline: copy.Fields})
		batch.applied[copy.SourceID] = applied
		if exists {
			changed[row.ID] = row
		} else {
			batch.final = append(batch.final, row)
		}
	}
	return nil
}

func mergeTraegerCopy(copy careplan.TraegerOfferingCopy, row careplan.CareOffering, exists bool) (careplan.CareOffering, careplan.TraegerOfferingApplied, error) {
	baseline := traegerOfferingValues(copy.Fields, copy.Fields.Active())
	applied := careplan.TraegerOfferingApplied{SourceID: copy.SourceID, Created: !exists}
	if exists {
		merged, changes := mergeTraegerOfferingValues(baseline, traegerOfferingValues(traegerOfferingFieldsOf(row), row.IsActive), row.Traeger.Deviations)
		if err := applyTraegerOfferingValues(&row, merged); err != nil {
			return row, applied, err
		}
		applied.Fields = changes
	} else {
		row = traegerOfferingOf(copy.Fields, copy.PhaseID)
		for _, field := range sortedTraegerKeys(baseline) {
			applied.Fields = append(applied.Fields, careplan.TraegerFieldChange{Field: field, State: traegerFieldNew, Traeger: baseline[field]})
		}
	}
	return row, applied, nil
}

func finishTraegerWithdrawals(batch traegerOfferingBatch, current []careplan.CareOffering, withdraw []int64, bySource, changed map[int64]careplan.CareOffering, refs map[int64]bool) traegerOfferingBatch {
	deleted := map[int64]bool{}
	for _, source := range withdraw {
		row, exists := bySource[source]
		batch.withdrawn[source] = careplan.TraegerOfferingWithdrawal{Found: exists, Deleted: exists && !refs[row.ID]}
		if !exists {
			continue
		}
		if refs[row.ID] {
			row.IsActive = false
			batch.writes = append(batch.writes, careplan.TraegerOfferingWrite{Offering: row, Unlink: true})
			changed[row.ID] = row
		} else {
			deleted[row.ID] = true
			batch.deleteIDs = append(batch.deleteIDs, row.ID)
		}
	}
	for _, row := range current {
		if deleted[row.ID] {
			continue
		}
		if replacement, ok := changed[row.ID]; ok {
			row = replacement
		}
		batch.final = append(batch.final, row)
	}
	return pruneDeletedTraegerTriggers(batch)
}

func (c *CareOfferingCatalog) traegerWithdrawalReferences(ctx context.Context, ids []int64) (map[int64]bool, error) {
	refs, err := c.deps.CopyReferences.BookedOfferingIDs(ctx, ids)
	if err != nil || len(ids) == 0 {
		return refs, err
	}
	reads, ok := c.deps.Bookings.(traegerBatchBookings)
	if !ok {
		return nil, errors.New("catalog batch: Enrollment batch reads are required")
	}
	selections, err := reads.ReferencedOfferingIDs(ctx, ids)
	for _, id := range selections {
		refs[id] = true
	}
	return refs, err
}

func (c *CareOfferingCatalog) resyncTraegerBatch(ctx context.Context, saved []careplan.CareOffering, deleted []int64) error {
	effects, ok := c.deps.SourcedTemplates.(traegerBatchEffects)
	if !ok {
		return errors.New("catalog batch: batch roster effects are required")
	}
	pickups, ok := c.deps.Pickup.(traegerBatchPickup)
	if !ok {
		return errors.New("catalog batch: batch pickup effects are required")
	}
	ids := make([]int64, 0, len(saved))
	for _, row := range saved {
		ids = append(ids, row.ID)
	}
	if len(deleted) > 0 {
		if err := effects.DetachTemplatesSourcedFromOfferings(ctx, deleted, c.todayDate()); err != nil {
			return err
		}
	}
	if len(ids) > 0 {
		if err := effects.ResyncTemplatesSourcedFromOfferings(ctx, ids, c.todayDate()); err != nil {
			if c.deps.SourceRules.IsRejection(err) {
				c.deps.MarkRollback(ctx)
				return fmt.Errorf("%w: %w", careplan.ErrCareOfferingConfigInvalid, err)
			}
			return err
		}
		return pickups.ReconcileOfferingPickupForOfferings(ctx, ids)
	}
	return nil
}

// Deleted source rows cascade their trigger links in the database. Validate
// and save the same final links before that cascade runs.
func pruneDeletedTraegerTriggers(batch traegerOfferingBatch) traegerOfferingBatch {
	if len(batch.deleteIDs) == 0 {
		return batch
	}
	deleted := make(map[int64]bool, len(batch.deleteIDs))
	for _, id := range batch.deleteIDs {
		deleted[id] = true
	}
	for index := range batch.writes {
		batch.writes[index].Offering.AutoAddTriggerOfferingIDs = survivingTraegerTriggers(batch.writes[index].Offering.AutoAddTriggerOfferingIDs, deleted)
	}
	for index := range batch.final {
		batch.final[index].AutoAddTriggerOfferingIDs = survivingTraegerTriggers(batch.final[index].AutoAddTriggerOfferingIDs, deleted)
	}
	return batch
}

func survivingTraegerTriggers(ids []int64, deleted map[int64]bool) []int64 {
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !deleted[id] {
			result = append(result, id)
		}
	}
	return result
}
