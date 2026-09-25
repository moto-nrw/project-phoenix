package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CareOfferingCatalogDependencies are the ports the catalog reads and the
// collaborators it keeps in step. SourcedTemplates and Pickup resolve late:
// the composition binds Enrollment's decision service after the catalog.
type CareOfferingCatalogDependencies struct {
	Records      ports.CatalogRecords
	Phases       ports.CatalogPhases
	Timetable    ports.CatalogTimetable
	Calendar     ports.CatalogCalendar
	Bookings     ports.CatalogBookings
	Settings     ports.CatalogSettings
	Translations ports.CatalogTranslations
	SourceRules  ports.OfferingSourceRules

	SourcedTemplates func() ports.SourcedTemplateResyncer
	Pickup           func() ports.PickupResyncer

	// LockTemplateRecurrence serializes link validation and writes with
	// template split/end. Focused tests may leave it nil.
	LockTemplateRecurrence func(context.Context) error
	// MarkRollback marks the ambient tenant transaction for rollback, so a
	// rejected update is discarded although the 4xx response commits.
	MarkRollback func(context.Context)
	Today        func() calendar.Date
	Logger       *slog.Logger
}

// CareOfferingCatalog is the Care Plan care-offering catalog (#3559).
type CareOfferingCatalog struct {
	deps CareOfferingCatalogDependencies
}

var _ careplan.CareOfferingCatalogCapability = (*CareOfferingCatalog)(nil)

// NewCareOfferingCatalog builds the catalog over its ports.
func NewCareOfferingCatalog(deps CareOfferingCatalogDependencies) (*CareOfferingCatalog, error) {
	if deps.Records == nil || deps.Phases == nil || deps.Timetable == nil || deps.Calendar == nil ||
		deps.Bookings == nil || deps.Settings == nil || deps.Translations == nil || deps.SourceRules == nil ||
		deps.MarkRollback == nil {
		return nil, errors.New("care offering catalog: records, phases, timetable, calendar, bookings, settings, translations, offering-source rules and rollback marker are required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.Today == nil {
		deps.Today = calendar.TodayDate
	}
	return &CareOfferingCatalog{deps: deps}, nil
}

func (c *CareOfferingCatalog) ListCatalog(ctx context.Context) ([]careplan.CareOffering, error) {
	return c.listValidated(ctx, careplan.CareOfferingFilter{Order: careplan.OfferingOrderCatalog}, "failed to list care offerings")
}

func (c *CareOfferingCatalog) ListByPhase(ctx context.Context, phaseID int64) ([]careplan.CareOffering, error) {
	if phaseID <= 0 {
		return nil, careOfferingInvalidf("phase_id must be positive")
	}
	return c.listValidated(ctx, phaseFilter(phaseID), "failed to list care offerings by phase")
}

func (c *CareOfferingCatalog) FindOffering(ctx context.Context, id int64) (careplan.CareOffering, error) {
	if id <= 0 {
		return careplan.CareOffering{}, careplan.ErrCareOfferingNotFound
	}
	offering, err := c.findRecord(ctx, id)
	if err != nil {
		if errors.Is(err, careplan.ErrCareOfferingNotFound) {
			return careplan.CareOffering{}, careplan.ErrCareOfferingNotFound
		}
		return careplan.CareOffering{}, fmt.Errorf("load care offering: %w", err)
	}
	if err := normalizeLoadedAvailabilityRule(&offering); err != nil {
		return careplan.CareOffering{}, err
	}
	return offering, nil
}

func (c *CareOfferingCatalog) CreateOffering(ctx context.Context, offering careplan.CareOffering) (careplan.CareOffering, error) {
	if err := c.prepareWrite(ctx, &offering); err != nil {
		return careplan.CareOffering{}, err
	}
	created, err := c.createRecord(ctx, offering)
	if err != nil {
		return careplan.CareOffering{}, err
	}
	if err := c.replaceTriggers(ctx, created.ID, offering.AutoAddTriggerOfferingIDs); err != nil {
		return careplan.CareOffering{}, err
	}
	c.deps.Logger.Info("care offering created",
		slog.Int64("offering_id", created.ID),
		slog.String("name", created.Name))
	return created, nil
}

func (c *CareOfferingCatalog) UpdateOffering(ctx context.Context, offering careplan.CareOffering) (careplan.CareOffering, error) {
	if offering.ID <= 0 {
		return careplan.CareOffering{}, careOfferingInvalidf("offering with valid id is required")
	}
	if err := c.prepareWrite(ctx, &offering); err != nil {
		return careplan.CareOffering{}, err
	}
	updated, err := c.updateRecord(ctx, offering)
	if err != nil {
		return careplan.CareOffering{}, err
	}
	if err := c.replaceTriggers(ctx, offering.ID, offering.AutoAddTriggerOfferingIDs); err != nil {
		return careplan.CareOffering{}, err
	}
	if err := c.resyncSourcedTemplates(ctx, offering.ID); err != nil {
		return careplan.CareOffering{}, err
	}
	if err := c.resyncPickupProjection(ctx, offering.ID); err != nil {
		return careplan.CareOffering{}, err
	}
	c.deps.Logger.Info("care offering updated", slog.Int64("offering_id", offering.ID))
	return updated, nil
}

// prepareWrite runs every admin-save check in the order the catalog has
// always applied them; the recurrence lock is taken before the timetable
// link is read.
func (c *CareOfferingCatalog) prepareWrite(ctx context.Context, offering *careplan.CareOffering) error {
	if err := c.validateWrite(offering); err != nil {
		return err
	}
	if err := c.validateAvailabilityRule(ctx, *offering); err != nil {
		return err
	}
	if err := c.lockTemplateRecurrence(ctx); err != nil {
		return err
	}
	if err := c.validateLinkedTemplate(ctx, *offering); err != nil {
		return err
	}
	if err := c.checkGroupRuleConsistency(ctx, *offering); err != nil {
		return err
	}
	return c.validateAutoAddConfig(ctx, offering)
}

func (c *CareOfferingCatalog) validateWrite(offering *careplan.CareOffering) error {
	if err := validateOfferingFields(offering); err != nil {
		return wrapCareOfferingInvalid(err, "validate care offering")
	}
	if err := c.normalizeTranslations(offering); err != nil {
		return err
	}
	missing := missingPickupWeekdays(*offering)
	if len(missing) == 0 {
		return nil
	}
	return wrapCareOfferingInvalid(
		fmt.Errorf("%w: %s", careplan.ErrCareOfferingPickupTimesRequired, strings.Join(missing, ", ")),
		"validate care offering pickup times",
	)
}

// normalizeTranslations validates the submitted translation document and
// stores its canonical form, or nothing when no translation remains.
func (c *CareOfferingCatalog) normalizeTranslations(offering *careplan.CareOffering) error {
	normalized, ok, err := c.deps.Translations.NormalizeTranslations(offering.Translations)
	if !ok {
		return careOfferingInvalidf("translations must be a valid translation document")
	}
	if err != nil {
		return wrapCareOfferingInvalid(err, "validate care offering translations")
	}
	offering.Translations = normalized
	return nil
}

func (c *CareOfferingCatalog) validateAvailabilityRule(ctx context.Context, offering careplan.CareOffering) error {
	rule, err := decodeAvailabilityRule(offering.AvailabilityRule)
	if err != nil {
		return err
	}
	if rule == nil || len(rule.Conditions) == 0 {
		return nil
	}
	gradeMax, err := c.deps.Settings.GradeLevelMax(ctx)
	if err != nil {
		return fmt.Errorf("resolve tenant grade range: %w", err)
	}
	if gradeMax < minGradeLevel || gradeMax > maxGradeLevel {
		return fmt.Errorf("tenant grade range is invalid: maximum %d", gradeMax)
	}
	for i, condition := range rule.Conditions {
		for _, grade := range condition.Value {
			if grade > gradeMax {
				return careOfferingInvalidf("availability_rule condition %d contains grade %d outside tenant range 1-%d", i+1, grade, gradeMax)
			}
		}
	}
	return nil
}

// checkGroupRuleConsistency enforces that every offering sharing a phase and
// selection group declares the same selection rule (empty = "optional"). A
// mixed group produces contradictory parent hints, so the admin save path
// refuses it instead of every parent submission failing later.
func (c *CareOfferingCatalog) checkGroupRuleConsistency(ctx context.Context, offering careplan.CareOffering) error {
	group := strings.TrimSpace(offering.SelectionGroup)
	if group == "" {
		return nil
	}
	thisRule := normalizeSelectionRule(offering.SelectionRule)
	siblings, err := c.listRecords(ctx, phaseFilter(offering.PhaseID), "failed to list care offerings by phase")
	if err != nil {
		return fmt.Errorf("check selection group consistency: %w", err)
	}
	for _, sibling := range siblings {
		if sibling.ID == offering.ID || strings.TrimSpace(sibling.SelectionGroup) != group {
			continue
		}
		if sibRule := normalizeSelectionRule(sibling.SelectionRule); sibRule != thisRule {
			return fmt.Errorf(
				"%w: group %q already uses %q, cannot also use %q",
				careplan.ErrCareOfferingGroupRuleConflict, group, sibRule, thisRule,
			)
		}
	}
	return nil
}

func (c *CareOfferingCatalog) validateAutoAddConfig(ctx context.Context, offering *careplan.CareOffering) error {
	offering.AutoAddTriggerOfferingIDs = normalizeTriggerOfferingIDs(offering.ID, offering.AutoAddTriggerOfferingIDs)
	if len(offering.AutoAddTriggerOfferingIDs) == 0 {
		return nil
	}
	if offering.DaysOfWeekMode != daysOfWeekModeParentChoice {
		return careOfferingInvalidf("an automatically added care offering must allow parent day selection")
	}
	siblings, err := c.listRecords(ctx, phaseFilter(offering.PhaseID), "failed to list care offerings by phase")
	if err != nil {
		return fmt.Errorf("check automatic offering triggers: %w", err)
	}
	triggerByID := make(map[int64]careplan.CareOffering, len(siblings))
	for _, sibling := range siblings {
		if sibling.ID != offering.ID {
			triggerByID[sibling.ID] = sibling
		}
	}
	for _, triggerID := range offering.AutoAddTriggerOfferingIDs {
		trigger, ok := triggerByID[triggerID]
		if !ok {
			return careOfferingInvalidf("automatic trigger offering %d must belong to the same phase", triggerID)
		}
		if autoAddViolatesExclusiveGroup(*offering, trigger) {
			return careOfferingInvalidf("automatic trigger offering %d cannot auto-add offering %d in exclusive selection group %q", triggerID, offering.ID, strings.TrimSpace(offering.SelectionGroup))
		}
	}
	return nil
}

func (c *CareOfferingCatalog) DeleteOffering(ctx context.Context, id int64) error {
	if id <= 0 {
		return careOfferingInvalidf("id must be positive")
	}
	// Retire sourced rosters BEFORE the row delete: the FK's ON DELETE SET
	// NULL only degrades the templates to manual rosters and would leave
	// their offering-derived enrollment rows and materialized occurrences
	// behind (#2147 review round 11). The recurrence lock serializes the
	// retirement with concurrent approvals and template saves.
	if err := c.lockTemplateRecurrence(ctx); err != nil {
		return err
	}
	if err := c.detachSourcedTemplates(ctx, id); err != nil {
		return err
	}
	if err := c.deleteRecord(ctx, id); err != nil {
		return err
	}
	c.deps.Logger.Info("care offering deleted", slog.Int64("offering_id", id))
	return nil
}

// Clone copies a care offering into a new row of the target phase.
// Offering-level fields are preserved except a cross-phase timetable link.
func (c *CareOfferingCatalog) Clone(ctx context.Context, sourceID, targetPhaseID int64) (careplan.CareOffering, error) {
	if sourceID <= 0 {
		return careplan.CareOffering{}, careOfferingInvalidf("source id must be positive")
	}
	if targetPhaseID <= 0 {
		return careplan.CareOffering{}, careOfferingInvalidf("target phase id must be positive")
	}
	if err := c.lockTemplateRecurrence(ctx); err != nil {
		return careplan.CareOffering{}, err
	}
	source, err := c.findRecord(ctx, sourceID)
	if err != nil {
		if errors.Is(err, careplan.ErrCareOfferingNotFound) {
			return careplan.CareOffering{}, careOfferingInvalidf("source care offering does not exist")
		}
		return careplan.CareOffering{}, fmt.Errorf("clone: source lookup: %w", err)
	}
	clone := cloneOffering(source, targetPhaseID)
	if source.PhaseID != targetPhaseID {
		clone.ActivityGroupID = nil
	}
	if err := c.validateAvailabilityRule(ctx, clone); err != nil {
		return careplan.CareOffering{}, fmt.Errorf("clone: validate availability rule: %w", err)
	}
	if err := c.validateLinkedTemplate(ctx, clone); err != nil {
		return careplan.CareOffering{}, fmt.Errorf("clone: validate linked template: %w", err)
	}
	if err := c.checkGroupRuleConsistency(ctx, clone); err != nil {
		return careplan.CareOffering{}, fmt.Errorf("clone: check selection group consistency: %w", err)
	}
	created, err := c.createRecord(ctx, clone)
	if err != nil {
		return careplan.CareOffering{}, fmt.Errorf("clone: create: %w", err)
	}
	c.deps.Logger.Info("care offering cloned",
		slog.Int64("source_id", sourceID),
		slog.Int64("clone_id", created.ID),
		slog.Int64("target_phase_id", targetPhaseID))
	return created, nil
}

// cloneOffering copies an offering for a new row of the target phase. The
// row identity is reset and triggers are dropped: they name offering ids.
func cloneOffering(source careplan.CareOffering, targetPhaseID int64) careplan.CareOffering {
	clone := source
	clone.ID = 0
	clone.CreatedAt = time.Time{}
	clone.UpdatedAt = time.Time{}
	clone.PhaseID = targetPhaseID
	clone.AutoAddTriggerOfferingIDs = nil
	return clone
}

func (c *CareOfferingCatalog) todayDate() calendar.Date {
	return c.deps.Today()
}

func (c *CareOfferingCatalog) lockTemplateRecurrence(ctx context.Context) error {
	if c.deps.LockTemplateRecurrence == nil {
		return nil
	}
	if err := c.deps.LockTemplateRecurrence(ctx); err != nil {
		return fmt.Errorf("care offering: lock template recurrence: %w", err)
	}
	return nil
}

func (c *CareOfferingCatalog) resyncPickupProjection(ctx context.Context, offeringID int64) error {
	resyncer := c.pickupResyncer()
	if resyncer == nil {
		return errors.New("care offering update: pickup resyncer not configured")
	}
	if err := resyncer.ReconcileOfferingPickupForOffering(ctx, offeringID); err != nil {
		return fmt.Errorf("care offering update: resync pickup projection: %w", err)
	}
	return nil
}

// resyncSourcedTemplates re-reconciles every template sourcing this offering
// after an update (#2147 review): changed days or a moved phase change the
// wanted roster, and the sourced rows plus already-materialized occurrences
// follow immediately. An edit that makes the source incompatible with a
// template's planning period is rejected (#2147 review round 7). Runs under
// the recurrence lock the update holds; history before today stays untouched.
func (c *CareOfferingCatalog) resyncSourcedTemplates(ctx context.Context, offeringID int64) error {
	resyncer := c.sourcedTemplateResyncer()
	if resyncer == nil {
		// Focused tests may run without the decision wiring; the
		// composition always binds it.
		c.deps.Logger.Warn("care offering update: sourced-template resyncer not configured; sourced rosters may be stale",
			slog.Int64("offering_id", offeringID))
		return nil
	}
	if err := resyncer.ResyncTemplatesSourcedFromOffering(ctx, offeringID, c.todayDate()); err != nil {
		if c.deps.SourceRules.IsRejection(err) {
			// The tenant transaction commits ordinary 4xx responses. Mark it
			// so the already-written offering update is discarded together
			// with the rejection.
			c.deps.MarkRollback(ctx)
			return fmt.Errorf("%w: %w", careplan.ErrCareOfferingConfigInvalid, err)
		}
		return fmt.Errorf("care offering update: resync sourced templates: %w", err)
	}
	return nil
}

// detachSourcedTemplates retires the rosters of every template sourcing the
// offering ahead of its deletion.
func (c *CareOfferingCatalog) detachSourcedTemplates(ctx context.Context, offeringID int64) error {
	resyncer := c.sourcedTemplateResyncer()
	if resyncer == nil {
		c.deps.Logger.Warn("care offering delete: sourced-template resyncer not configured; sourced rosters may be orphaned",
			slog.Int64("offering_id", offeringID))
		return nil
	}
	if err := resyncer.DetachTemplatesSourcedFromOffering(ctx, offeringID, c.todayDate()); err != nil {
		return fmt.Errorf("care offering delete: detach sourced templates: %w", err)
	}
	return nil
}

func (c *CareOfferingCatalog) sourcedTemplateResyncer() ports.SourcedTemplateResyncer {
	if c.deps.SourcedTemplates == nil {
		return nil
	}
	return c.deps.SourcedTemplates()
}

func (c *CareOfferingCatalog) pickupResyncer() ports.PickupResyncer {
	if c.deps.Pickup == nil {
		return nil
	}
	return c.deps.Pickup()
}
