package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
)

// ErrCareOfferingNotFound is the sentinel returned by GetByID when the
// row doesn't exist (or the tenant can't see it via RLS).
var ErrCareOfferingNotFound = errors.New("care offering not found")

// ErrCareOfferingGroupRuleConflict is returned by Create/Update when saving
// an offering would leave two offerings in the same selection_group with
// different non-optional selection rules. The invariant is enforced at save
// time so an admin fixes the misconfiguration immediately, instead of every
// parent submit for the phase failing later when the conflict is detected.
var ErrCareOfferingGroupRuleConflict = errors.New("care offerings in the same selection group must share one selection rule")

// CareOfferingService manages the per-tenant care-offering catalog.
// Admin endpoints (PR 6) read + write all offerings; the public form
// fetches the active offerings for a parent-selected phase.
type CareOfferingService interface {
	List(ctx context.Context) ([]*enrollmentModels.CareOffering, error)
	ListByPhase(ctx context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error)
	ListActiveByPhase(ctx context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error)
	GetByID(ctx context.Context, id int64) (*enrollmentModels.CareOffering, error)
	Create(ctx context.Context, offering *enrollmentModels.CareOffering) (*enrollmentModels.CareOffering, error)
	Update(ctx context.Context, offering *enrollmentModels.CareOffering) error
	Delete(ctx context.Context, id int64) error

	// Clone copies an existing offering into a new row scoped to a
	// target phase. All offering-level fields (capacity, days, lunch,
	// price, etc.) are preserved; the source row's ID is reset and
	// phase_id is set to the target. When cloning across phases, linked
	// timetable templates are cleared so admins can relink a template for
	// the new phase. Use case: cloning last year's catalog into this year's
	// phase, then editing what changed.
	Clone(ctx context.Context, sourceID int64, targetPhaseID int64) (*enrollmentModels.CareOffering, error)
}

// CareOfferingServiceConfig is the dep-injection bundle.
type CareOfferingServiceConfig struct {
	Repo                 enrollmentModels.CareOfferingRepository
	ActivityGroupRepo    activitiesModels.GroupRepository
	ActivityScheduleRepo activitiesModels.ScheduleRepository
	CalendarPeriodRepo   scheduleModels.CalendarPeriodRepository
	PhaseRepo            enrollmentModels.PhaseRepository
	Logger               *slog.Logger
}

type careOfferingService struct {
	CareOfferingServiceConfig
}

// NewCareOfferingService builds the service.
func NewCareOfferingService(cfg CareOfferingServiceConfig) CareOfferingService {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &careOfferingService{CareOfferingServiceConfig: cfg}
}

func (s *careOfferingService) List(ctx context.Context) ([]*enrollmentModels.CareOffering, error) {
	return s.Repo.ListByTenant(ctx)
}

func (s *careOfferingService) ListByPhase(ctx context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error) {
	if phaseID <= 0 {
		return nil, fmt.Errorf("phase_id must be positive")
	}
	return s.Repo.ListByPhase(ctx, phaseID)
}

func (s *careOfferingService) ListActiveByPhase(ctx context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error) {
	if phaseID <= 0 {
		return nil, fmt.Errorf("phase_id must be positive")
	}
	return s.Repo.ListActiveByPhase(ctx, phaseID)
}

func (s *careOfferingService) GetByID(ctx context.Context, id int64) (*enrollmentModels.CareOffering, error) {
	if id <= 0 {
		return nil, ErrCareOfferingNotFound
	}
	offering, err := s.Repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrCareOfferingNotFound
	}
	return offering, nil
}

// normalizeSelectionRule maps an empty rule to the "optional" default so a
// group's members can be compared on equal footing.
func normalizeSelectionRule(rule string) string {
	if rule == "" {
		return enrollmentModels.SelectionRuleOptional
	}
	return rule
}

// checkGroupRuleConsistency enforces that every offering sharing a phase +
// selection_group declares the SAME selection_rule (treating empty as
// "optional"). The parent submit path counts all members of a group, so a
// mixed group — e.g. one "exactly_one" offering next to an "optional" one —
// produces contradictory UI hints and later backend rejections. Enforced here
// on the admin save path so the misconfiguration surfaces to the admin
// immediately instead of blocking every parent submission for the phase.
func (s *careOfferingService) checkGroupRuleConsistency(ctx context.Context, offering *enrollmentModels.CareOffering) error {
	group := strings.TrimSpace(offering.SelectionGroup)
	if group == "" {
		return nil
	}
	thisRule := normalizeSelectionRule(offering.SelectionRule)
	siblings, err := s.Repo.ListByPhase(ctx, offering.PhaseID)
	if err != nil {
		return fmt.Errorf("check selection group consistency: %w", err)
	}
	for _, sib := range siblings {
		if sib.ID == offering.ID {
			continue
		}
		if strings.TrimSpace(sib.SelectionGroup) != group {
			continue
		}
		sibRule := normalizeSelectionRule(sib.SelectionRule)
		if sibRule != thisRule {
			return fmt.Errorf(
				"%w: group %q already uses %q, cannot also use %q",
				ErrCareOfferingGroupRuleConflict, group, sibRule, thisRule,
			)
		}
	}
	return nil
}

func normalizeTriggerOfferingIDs(targetID int64, ids []int64) []int64 {
	if len(ids) == 0 {
		return []int64{}
	}
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || id == targetID || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func (s *careOfferingService) validateAutoAddConfig(ctx context.Context, offering *enrollmentModels.CareOffering) error {
	offering.AutoAddTriggerOfferingIDs = normalizeTriggerOfferingIDs(offering.ID, offering.AutoAddTriggerOfferingIDs)
	if len(offering.AutoAddTriggerOfferingIDs) == 0 {
		return nil
	}
	if offering.DaysOfWeekMode != enrollmentModels.DaysOfWeekModeParentChoice {
		return fmt.Errorf("an automatically added care offering must allow parent day selection")
	}
	siblings, err := s.Repo.ListByPhase(ctx, offering.PhaseID)
	if err != nil {
		return fmt.Errorf("check automatic offering triggers: %w", err)
	}
	triggerByID := make(map[int64]*enrollmentModels.CareOffering, len(siblings))
	for _, sibling := range siblings {
		if sibling.ID == offering.ID {
			continue
		}
		triggerByID[sibling.ID] = sibling
	}
	for _, triggerID := range offering.AutoAddTriggerOfferingIDs {
		trigger := triggerByID[triggerID]
		if trigger == nil {
			return fmt.Errorf("automatic trigger offering %d must belong to the same phase", triggerID)
		}
		if autoAddViolatesExclusiveGroup(offering, trigger) {
			return fmt.Errorf("automatic trigger offering %d cannot auto-add offering %d in exclusive selection group %q", triggerID, offering.ID, strings.TrimSpace(offering.SelectionGroup))
		}
	}
	return nil
}

func autoAddViolatesExclusiveGroup(target, trigger *enrollmentModels.CareOffering) bool {
	if target == nil || trigger == nil {
		return false
	}
	group := strings.TrimSpace(target.SelectionGroup)
	if group == "" || strings.TrimSpace(trigger.SelectionGroup) != group {
		return false
	}
	switch normalizeSelectionRule(target.SelectionRule) {
	case enrollmentModels.SelectionRuleExactlyOne, enrollmentModels.SelectionRuleAtMostOne:
		return true
	default:
		return false
	}
}

type careOfferingTemplateDeps struct {
	activityGroupRepo    activitiesModels.GroupRepository
	activityScheduleRepo activitiesModels.ScheduleRepository
	calendarPeriodRepo   scheduleModels.CalendarPeriodRepository
}

func (d careOfferingTemplateDeps) validateActivityGroupLookup() error {
	if d.activityGroupRepo == nil {
		return errors.New("activity group validation dependency is not configured")
	}
	return nil
}

func (d careOfferingTemplateDeps) validate() error {
	if d.activityGroupRepo == nil || d.activityScheduleRepo == nil || d.calendarPeriodRepo == nil {
		return errors.New("template validation dependencies are not configured")
	}
	return nil
}

func resolveCareOfferingTemplatePeriod(
	ctx context.Context,
	deps careOfferingTemplateDeps,
	activityGroupID int64,
) (*scheduleModels.CalendarPeriod, error) {
	if activityGroupID <= 0 {
		return nil, errors.New("activity_group_id must be positive when set")
	}
	if err := deps.validate(); err != nil {
		return nil, err
	}

	group, err := deps.activityGroupRepo.FindByID(ctx, activityGroupID)
	if err != nil || group == nil {
		return nil, fmt.Errorf("activity_group_id does not reference a template in this tenant")
	}
	if !group.IsTemplate {
		return nil, fmt.Errorf("activity_group_id must reference a timetable template")
	}
	if group.ArchivedAt != nil {
		return nil, fmt.Errorf("activity_group_id references an archived timetable template")
	}

	schedules, err := deps.activityScheduleRepo.FindByGroupID(ctx, activityGroupID)
	if err != nil {
		return nil, fmt.Errorf("load timetable template schedules: %w", err)
	}
	if len(schedules) == 0 {
		return nil, fmt.Errorf("timetable template must have at least one schedule")
	}

	var periodID *int64
	for _, schedule := range schedules {
		if schedule.CalendarPeriodID == nil {
			return nil, fmt.Errorf("timetable template schedules must all have a calendar_period_id")
		}
		if periodID == nil {
			id := *schedule.CalendarPeriodID
			periodID = &id
			continue
		}
		if *periodID != *schedule.CalendarPeriodID {
			return nil, fmt.Errorf("timetable template schedules must use one calendar_period_id")
		}
	}

	period, err := deps.calendarPeriodRepo.FindByID(ctx, *periodID)
	if err != nil || period == nil {
		return nil, fmt.Errorf("calendar period for timetable template not found")
	}
	return period, nil
}

func resolveCareOfferingLinkedGroupPeriod(
	ctx context.Context,
	deps careOfferingTemplateDeps,
	activityGroupID int64,
) (*activitiesModels.Group, *scheduleModels.CalendarPeriod, error) {
	if activityGroupID <= 0 {
		return nil, nil, errors.New("activity_group_id must be positive when set")
	}
	if err := deps.validateActivityGroupLookup(); err != nil {
		return nil, nil, err
	}

	group, err := deps.activityGroupRepo.FindByID(ctx, activityGroupID)
	if err != nil || group == nil {
		return nil, nil, fmt.Errorf("activity_group_id does not reference a group in this tenant")
	}
	if !group.IsTemplate {
		return group, nil, nil
	}

	period, err := resolveCareOfferingTemplatePeriod(ctx, deps, activityGroupID)
	if err != nil {
		return nil, nil, err
	}
	return group, period, nil
}

func validatePhaseWithinTemplatePeriod(phase *enrollmentModels.Phase, period *scheduleModels.CalendarPeriod) error {
	if phase == nil || period == nil {
		return nil
	}
	if phase.ServiceStartDate.Before(period.StartDate) || phase.ServiceEndDate.After(period.EndDate) {
		return fmt.Errorf("care offering phase must be within the linked timetable template period")
	}
	return nil
}

func (s *careOfferingService) validateLinkedTemplate(ctx context.Context, offering *enrollmentModels.CareOffering) error {
	if offering == nil || offering.ActivityGroupID == nil {
		return nil
	}
	period, err := resolveCareOfferingTemplatePeriod(ctx, careOfferingTemplateDeps{
		activityGroupRepo:    s.ActivityGroupRepo,
		activityScheduleRepo: s.ActivityScheduleRepo,
		calendarPeriodRepo:   s.CalendarPeriodRepo,
	}, *offering.ActivityGroupID)
	if err != nil {
		return err
	}
	if s.PhaseRepo == nil {
		return errors.New("phase validation dependency is not configured")
	}
	phase, err := s.PhaseRepo.FindByID(ctx, offering.PhaseID)
	if err != nil || phase == nil {
		return fmt.Errorf("phase_id does not reference a phase in this tenant")
	}
	return validatePhaseWithinTemplatePeriod(phase, period)
}

func (s *careOfferingService) Create(ctx context.Context, offering *enrollmentModels.CareOffering) (*enrollmentModels.CareOffering, error) {
	if offering == nil {
		return nil, fmt.Errorf("offering is required")
	}
	if err := s.validateLinkedTemplate(ctx, offering); err != nil {
		return nil, err
	}
	if err := s.checkGroupRuleConsistency(ctx, offering); err != nil {
		return nil, err
	}
	if err := s.validateAutoAddConfig(ctx, offering); err != nil {
		return nil, err
	}
	if err := s.Repo.Create(ctx, offering); err != nil {
		return nil, err
	}
	if err := s.Repo.ReplaceAutoAddTriggers(ctx, offering.ID, offering.AutoAddTriggerOfferingIDs); err != nil {
		return nil, err
	}
	s.Logger.Info("care offering created",
		slog.Int64("offering_id", offering.ID),
		slog.String("name", offering.Name))
	return offering, nil
}

func (s *careOfferingService) Update(ctx context.Context, offering *enrollmentModels.CareOffering) error {
	if offering == nil || offering.ID <= 0 {
		return fmt.Errorf("offering with valid id is required")
	}
	if err := s.validateLinkedTemplate(ctx, offering); err != nil {
		return err
	}
	if err := s.checkGroupRuleConsistency(ctx, offering); err != nil {
		return err
	}
	if err := s.validateAutoAddConfig(ctx, offering); err != nil {
		return err
	}
	if err := s.Repo.Update(ctx, offering); err != nil {
		return err
	}
	if err := s.Repo.ReplaceAutoAddTriggers(ctx, offering.ID, offering.AutoAddTriggerOfferingIDs); err != nil {
		return err
	}
	s.Logger.Info("care offering updated", slog.Int64("offering_id", offering.ID))
	return nil
}

func (s *careOfferingService) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("id must be positive")
	}
	if err := s.Repo.Delete(ctx, id); err != nil {
		return err
	}
	s.Logger.Info("care offering deleted", slog.Int64("offering_id", id))
	return nil
}

// Clone copies a care offering into a new row scoped to a target phase.
// Offering-level fields are preserved except cross-phase timetable template
// links; ID is reset so the DB assigns a fresh BIGSERIAL, and phase_id is
// repointed at the target.
func (s *careOfferingService) Clone(ctx context.Context, sourceID int64, targetPhaseID int64) (*enrollmentModels.CareOffering, error) {
	if sourceID <= 0 {
		return nil, fmt.Errorf("source id must be positive")
	}
	if targetPhaseID <= 0 {
		return nil, fmt.Errorf("target phase id must be positive")
	}

	source, err := s.Repo.FindByID(ctx, sourceID)
	if err != nil {
		return nil, fmt.Errorf("clone: source lookup: %w", err)
	}

	clone := *source
	clone.ID = 0 // BIGSERIAL - let the DB assign
	clone.PhaseID = targetPhaseID
	if source.PhaseID != targetPhaseID && clone.ActivityGroupID != nil {
		clone.ActivityGroupID = nil
	}
	clone.AutoAddTriggerOfferingIDs = nil
	if err := s.checkGroupRuleConsistency(ctx, &clone); err != nil {
		return nil, fmt.Errorf("clone: check selection group consistency: %w", err)
	}
	if err := s.Repo.Create(ctx, &clone); err != nil {
		return nil, fmt.Errorf("clone: create: %w", err)
	}
	s.Logger.Info("care offering cloned",
		slog.Int64("source_id", sourceID),
		slog.Int64("clone_id", clone.ID),
		slog.Int64("target_phase_id", targetPhaseID))
	return &clone, nil
}
