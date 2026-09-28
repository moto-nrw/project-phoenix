package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// bookingStatsWindowOn is the half-open date range ListBookingStats counts
// in. It reproduces the capacity gate's window, so the displayed occupancy is
// the number the gate applies at save time: from today (or the phase start,
// if the phase has not begun) through the last service day inclusive. A
// phase whose window has ended collapses onto its final service day, so the
// dialog shows the phase's end state instead of a meaningless zero.
func bookingStatsWindowOn(phase careplan.OfferingPhase, today calendar.Date) (from, until calendar.Date) {
	if phase.ServiceEnd.IsZero() {
		return today, today.AddDays(1)
	}
	from = today
	if phase.ServiceStart.After(from) {
		from = phase.ServiceStart
	}
	until = phase.ServiceEnd.AddDays(1)
	if !from.Before(until) {
		return phase.ServiceEnd, phase.ServiceEnd.AddDays(1)
	}
	return from, until
}

func (c *CareOfferingCatalog) ListBookingStats(ctx context.Context, phaseID int64) ([]careplan.CareOfferingBookingStat, error) {
	if phaseID <= 0 {
		return nil, careOfferingInvalidf("phase_id must be positive")
	}
	phase, err := c.deps.Phases.Phase(ctx, phaseID)
	if err != nil {
		if errors.Is(err, ports.ErrCatalogRowNotFound) {
			return nil, careOfferingInvalidf("phase does not exist")
		}
		return nil, fmt.Errorf("booking stats: phase lookup: %w", err)
	}
	offerings, err := c.listRecords(ctx, phaseFilter(phaseID), "failed to list care offerings by phase")
	if err != nil {
		return nil, fmt.Errorf("booking stats: list offerings: %w", err)
	}
	stats := make([]careplan.CareOfferingBookingStat, 0, len(offerings))
	if len(offerings) == 0 {
		return stats, nil
	}
	from, until := bookingStatsWindowOn(phase, c.todayDate())
	ids := make([]int64, 0, len(offerings))
	for _, offering := range offerings {
		ids = append(ids, offering.ID)
	}
	gradeCounts, err := c.deps.Bookings.OfferingGradeCounts(ctx, ids, from, until)
	if err != nil {
		return nil, fmt.Errorf("booking stats: count grade levels: %w", err)
	}
	grades, unknown := bookingGradeTotals(gradeCounts, len(offerings))
	peaks, err := c.deps.Bookings.OfferingCapacityPeaks(ctx, ids, from, until)
	if err != nil {
		return nil, fmt.Errorf("booking stats: count peak occupancy: %w", err)
	}
	for _, offering := range offerings {
		byGrade := grades[offering.ID]
		if byGrade == nil {
			byGrade = map[int]int{}
		}
		// An absent peak means no booking overlaps the window.
		stats = append(stats, careplan.CareOfferingBookingStat{
			OfferingID:        offering.ID,
			Capacity:          offering.Capacity,
			Booked:            peaks[offering.ID],
			GradeLevels:       byGrade,
			UnknownGradeCount: unknown[offering.ID],
		})
	}
	return stats, nil
}

func bookingGradeTotals(rows []ports.OfferingGradeCount, size int) (map[int64]map[int]int, map[int64]int) {
	grades := make(map[int64]map[int]int, size)
	unknown := make(map[int64]int, size)
	for _, row := range rows {
		if row.GradeLevel == nil {
			unknown[row.CareOfferingID] += row.Count
			continue
		}
		if grades[row.CareOfferingID] == nil {
			grades[row.CareOfferingID] = make(map[int]int)
		}
		grades[row.CareOfferingID][int(*row.GradeLevel)] += row.Count
	}
	return grades, unknown
}

// CloneCatalogForRollover copies the catalog into the follow-up phase; see
// careplan.CareOfferingRollover. It runs inside the rollover's tenant
// transaction, so a validation failure on any offering rolls back the whole
// follow-up phase.
func (c *CareOfferingCatalog) CloneCatalogForRollover(ctx context.Context, sourcePhaseID, targetPhaseID int64, carriedOfferingIDs []int64) (map[int64]int64, error) {
	if err := validateRolloverPhases(sourcePhaseID, targetPhaseID); err != nil {
		return nil, err
	}
	if err := c.lockTemplateRecurrence(ctx); err != nil {
		return nil, err
	}
	sources, err := c.rolloverSources(ctx, sourcePhaseID, carriedOfferingIDs)
	if err != nil {
		return nil, err
	}
	if err := validateCatalogGroupRuleConsistency(sources); err != nil {
		return nil, fmt.Errorf("rollover catalog clone: %w", err)
	}
	carriedByID := make(map[int64]bool, len(carriedOfferingIDs))
	for _, offeringID := range carriedOfferingIDs {
		carriedByID[offeringID] = true
	}
	mapping := make(map[int64]int64, len(sources))
	for _, source := range sources {
		cloneID, err := c.cloneRolloverOffering(ctx, source, targetPhaseID, source.IsActive || carriedByID[source.ID])
		if err != nil {
			return nil, err
		}
		mapping[source.ID] = cloneID
	}
	if err := c.remapRolloverTriggers(ctx, sources, mapping); err != nil {
		return nil, err
	}
	c.deps.Logger.Info("care offering catalog cloned for rollover",
		slog.Int64("source_phase_id", sourcePhaseID),
		slog.Int64("target_phase_id", targetPhaseID),
		slog.Int("offering_count", len(mapping)))
	return mapping, nil
}

func validateRolloverPhases(sourcePhaseID, targetPhaseID int64) error {
	switch {
	case sourcePhaseID <= 0:
		return careOfferingInvalidf("source phase id must be positive")
	case targetPhaseID <= 0:
		return careOfferingInvalidf("target phase id must be positive")
	case sourcePhaseID == targetPhaseID:
		return careOfferingInvalidf("source and target phase must differ")
	default:
		return nil
	}
}

// rolloverSources are the source phase's offerings plus the carried
// offerings of earlier phases, in that order.
func (c *CareOfferingCatalog) rolloverSources(ctx context.Context, sourcePhaseID int64, carriedOfferingIDs []int64) ([]careplan.CareOffering, error) {
	sources, err := c.listRecords(ctx, phaseFilter(sourcePhaseID), "failed to list care offerings by phase")
	if err != nil {
		return nil, fmt.Errorf("rollover catalog clone: list source offerings: %w", err)
	}
	known := make(map[int64]bool, len(sources)+len(carriedOfferingIDs))
	for _, source := range sources {
		known[source.ID] = true
	}
	missingIDs := make([]int64, 0)
	for _, offeringID := range carriedOfferingIDs {
		if !known[offeringID] {
			missingIDs = append(missingIDs, offeringID)
		}
	}
	carried, err := c.listByIDs(ctx, missingIDs)
	if err != nil {
		return nil, fmt.Errorf("rollover catalog clone: load carried offerings: %w", err)
	}
	for _, source := range carried {
		sources = append(sources, source)
		known[source.ID] = true
	}
	for _, offeringID := range missingIDs {
		if !known[offeringID] {
			return nil, fmt.Errorf("rollover catalog clone: carried offering %d not found", offeringID)
		}
	}
	return sources, nil
}

func (c *CareOfferingCatalog) cloneRolloverOffering(ctx context.Context, source careplan.CareOffering, targetPhaseID int64, requiresMaterialization bool) (int64, error) {
	// Triggers reference offering ids; they are remapped once every clone
	// has its id.
	clone := cloneOffering(source, targetPhaseID)
	if err := c.validateAvailabilityRule(ctx, clone); err != nil {
		return 0, fmt.Errorf("rollover catalog clone: offering %q (%d): %w", source.Name, source.ID, err)
	}
	if err := c.validateRolloverCloneLinkedGroup(ctx, clone, requiresMaterialization); err != nil {
		return 0, fmt.Errorf("rollover catalog clone: offering %q (%d): %w", source.Name, source.ID, err)
	}
	// Group-rule consistency was checked across the complete clone set
	// before the first row was inserted.
	created, err := c.createRecord(ctx, clone)
	if err != nil {
		return 0, fmt.Errorf("rollover catalog clone: offering %q (%d): %w", source.Name, source.ID, err)
	}
	return created.ID, nil
}

func (c *CareOfferingCatalog) remapRolloverTriggers(ctx context.Context, sources []careplan.CareOffering, mapping map[int64]int64) error {
	for _, source := range sources {
		remapped := make([]int64, 0, len(source.AutoAddTriggerOfferingIDs))
		for _, triggerID := range source.AutoAddTriggerOfferingIDs {
			cloneTriggerID, ok := mapping[triggerID]
			if !ok {
				// Saves pin triggers to the same phase, so only legacy rows
				// get here. Carrying a cross-phase trigger forward would
				// create the mixed-phase state the rollover exists to prevent.
				c.deps.Logger.Warn("rollover catalog clone: dropping trigger outside the source phase",
					slog.Int64("offering_id", source.ID),
					slog.Int64("trigger_offering_id", triggerID))
				continue
			}
			remapped = append(remapped, cloneTriggerID)
		}
		if len(remapped) == 0 {
			continue
		}
		if err := c.replaceTriggers(ctx, mapping[source.ID], remapped); err != nil {
			return fmt.Errorf("rollover catalog clone: remap triggers for offering %d: %w", source.ID, err)
		}
	}
	return nil
}

// validateRolloverCloneLinkedGroup validates a clone's activity-group link
// with the tolerance decision materialization applies: a linked TEMPLATE
// must cover the target phase, while a historical non-template link — which
// the admin catalog no longer allows but decisions still materialize — is
// carried verbatim. Rejecting those would wedge every rollover of a phase
// carrying pre-template-era links.
func (c *CareOfferingCatalog) validateRolloverCloneLinkedGroup(ctx context.Context, clone careplan.CareOffering, requiresMaterialization bool) error {
	if clone.ActivityGroupID == nil {
		return nil
	}
	group, err := c.deps.Timetable.FindGroup(ctx, *clone.ActivityGroupID)
	if err != nil {
		if errors.Is(err, ports.ErrCatalogRowNotFound) {
			return careOfferingInvalidf("activity_group_id does not reference a group in this tenant")
		}
		return fmt.Errorf("load linked activity group: %w", err)
	}
	if !group.IsTemplate {
		return nil
	}
	phase, err := c.deps.Phases.Phase(ctx, clone.PhaseID)
	if err != nil {
		if errors.Is(err, ports.ErrCatalogRowNotFound) {
			return careOfferingInvalidf("phase_id does not reference a phase in this tenant")
		}
		return fmt.Errorf("load care offering phase: %w", err)
	}
	return c.validateLinkedTemplateForMaterialization(ctx, clone, phase, requiresMaterialization)
}
