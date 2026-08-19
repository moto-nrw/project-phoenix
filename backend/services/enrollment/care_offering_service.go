package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ErrCareOfferingNotFound is the sentinel returned by GetByID when the
// row doesn't exist (or the tenant can't see it via RLS).
var ErrCareOfferingNotFound = errors.New("care offering not found")

// ErrCareOfferingInvalid classifies validation and compatibility failures an
// administrator can correct. Repository, advisory-lock, and other
// infrastructure errors deliberately do not wrap this sentinel, allowing HTTP
// handlers to return a generic 500 without leaking database details.
var ErrCareOfferingInvalid = enrollmentModels.ErrCareOfferingInvalid

// ErrCareOfferingTemplatePeriodMismatch is returned when a linked timetable
// template's planning period does not fully contain the enrollment phase's
// service dates. The API maps this sentinel to a stable error code so clients
// do not have to parse the human-readable error text.
var ErrCareOfferingTemplatePeriodMismatch = errors.New("care offering phase must be within the linked timetable template period")

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

	// ListBookingStats reports, per offering of the phase, how full it
	// currently is and how its bookings distribute across grade levels.
	// Admin-only: it is what lets the manual-enrollment and offering-
	// adjustment dialogs show occupancy up front instead of failing at save,
	// and what lets the availability-rule editor say how many existing
	// bookings a rule would contradict. Read-only, no child data leaves the
	// backend.
	ListBookingStats(ctx context.Context, phaseID int64) ([]CareOfferingBookingStat, error)
}

// CareOfferingBookingStat is one offering's admin-facing booking summary.
type CareOfferingBookingStat struct {
	OfferingID int64
	// Capacity is nil for an unlimited offering.
	Capacity *int
	// Booked is the peak number of children holding a slot simultaneously
	// inside the phase's remaining capacity window — the exact number the
	// capacity gate compares against Capacity when a new booking is saved.
	Booked int
	// GradeLevels maps a child's target grade level to how many booked
	// children carry it. Bookings whose grade is unknown are counted in
	// UnknownGradeCount instead; an availability rule never matches a missing
	// grade, so they contradict every rule.
	GradeLevels       map[int]int
	UnknownGradeCount int
}

// RolloverOfferingCatalogCloner is the narrow contract the rollover
// service consumes (#2249). Kept separate from CareOfferingService — like
// CareOfferingSeriesValidator — so HTTP-layer mocks never implement an
// operation that is not an enrollment endpoint.
//
// CloneCatalogForRollover copies EVERY care offering of the source phase,
// plus effective legacy bookings that reference an earlier phase, into the
// target phase and returns the source→target offering ID mapping. Unlike the
// admin Clone it keeps the linked timetable template
// (activity_group_id): the link resolution is split-series-aware, so a
// template whose recurrence covers the target phase keeps feeding
// materialization, and one that does not fails validation here —
// aborting the rollover atomically instead of producing a booking that
// only breaks at Freigabe. Auto-add triggers are remapped inside the
// cloned catalog. Sourced templates (source_care_offering_ids, #2137)
// are deliberately NOT rewritten — re-pointing period-bound templates at
// the new phase's offerings is planning work the admin owns.
type RolloverOfferingCatalogCloner interface {
	CloneCatalogForRollover(ctx context.Context, sourcePhaseID int64, targetPhaseID int64, carriedOfferingIDs []int64) (map[int64]int64, error)
}

// CareOfferingSeriesValidator is the narrow cross-domain contract used by the
// timetable split service. Keeping it separate from CareOfferingService avoids
// making HTTP-layer mocks implement an operation that is never exposed as an
// enrollment endpoint.
type CareOfferingSeriesValidator interface {
	ValidateTemplateSeries(ctx context.Context, groupID int64) error
	// ValidateTemplateOfferingSource guards a template's offering-source rule
	// (#2137) against a calendar period: every offering must exist, be
	// active, share one enrollment phase with the others, and that phase's
	// service window must lie within the period (nil skips the period
	// check). The split service calls it before carrying an inherited source
	// onto a successor pinned to a different Planungszeitraum — create/update
	// run the same check via the roster resync, and a split must not be able
	// to persist a state those paths reject. Failures wrap
	// services/schedule.ErrOfferingSourceInvalid.
	//
	// storedOfferingIDs are the ids ALREADY persisted on the template being
	// validated (nil on create). An id that does not resolve is rejected
	// unless it is stored: the jsonb array carries no FK, so a stored id may
	// have legitimately outlived its offering and must not wedge later edits
	// — but a NEW unknown id is client garbage that would otherwise persist
	// as a dead source with a permanently empty roster.
	ValidateTemplateOfferingSource(ctx context.Context, offeringIDs, storedOfferingIDs []int64, calendarPeriodID *int64) error
}

// CareOfferingMaterializationResourceValidator is the narrow cross-domain
// contract used before deleting rooms or timeframes. Both FKs use ON DELETE
// SET NULL, so the owning services must simulate the post-delete timetable
// before the database silently clears those references.
type CareOfferingMaterializationResourceValidator interface {
	ValidateRoomDeletion(ctx context.Context, roomID int64) error
	ValidateTimeframeChange(ctx context.Context, timeframeID int64, replacement *scheduleModels.Timeframe) error
	ValidateTimeframeDeletion(ctx context.Context, timeframeID int64) error
}

// CareOfferingPhaseValidator protects phase service-window updates. Changing
// the window changes the occurrence set every linked offering must cover.
type CareOfferingPhaseValidator interface {
	ValidatePhaseChange(ctx context.Context, phaseID int64, replacement *enrollmentModels.Phase) error
}

// CareOfferingServiceConfig is the dep-injection bundle.
type CareOfferingServiceConfig struct {
	Repo                     enrollmentModels.CareOfferingRepository
	RequestChildOfferingRepo enrollmentModels.RequestChildOfferingRepository
	ActivityGroupRepo        activitiesModels.GroupRepository
	ActivityScheduleRepo     activitiesModels.ScheduleRepository
	CalendarPeriodRepo       scheduleModels.CalendarPeriodRepository
	TimeframeRepo            scheduleModels.TimeframeRepository
	ActivityExceptionRepo    scheduleModels.ActivityExceptionRepository
	PhaseRepo                enrollmentModels.PhaseRepository
	Settings                 interface {
		ResolveInt(context.Context, string) (int, error)
	}
	// LockTemplateRecurrence serializes link validation/writes with template
	// split/end. Production wires the schedule service's transaction-scoped
	// tenant recurrence gate; focused service tests may leave it nil.
	LockTemplateRecurrence func(context.Context) error
	Logger                 *slog.Logger
}

// CareOfferingSourcedTemplateResyncer re-reconciles the rosters of every
// template sourcing an offering (#2137/#2147 review). Implemented by the
// enrollment decision service; injected via SetSourcedTemplateResyncer
// because the decision service is constructed after this one.
// DetachTemplatesSourcedFromOffering is the delete-side counterpart: it
// retires all offering-derived roster rows and reconciled occurrences before
// the offering row (or its phase) is deleted, so the FK's ON DELETE SET NULL
// only ever flips an already-clean template to a manual roster (#2147 review
// round 11).
type CareOfferingSourcedTemplateResyncer interface {
	ResyncTemplatesSourcedFromOffering(ctx context.Context, offeringID int64, effectiveFrom timezone.Date) error
	DetachTemplatesSourcedFromOffering(ctx context.Context, offeringID int64, effectiveFrom timezone.Date) error
}

// CareOfferingSourceResyncBinder is the factory-facing setter for the
// sourced-template resyncer (late-bound wiring, see above).
type CareOfferingSourceResyncBinder interface {
	SetSourcedTemplateResyncer(resyncer CareOfferingSourcedTemplateResyncer)
}

type careOfferingService struct {
	CareOfferingServiceConfig
	sourcedTemplateResyncer CareOfferingSourcedTemplateResyncer
}

// SetSourcedTemplateResyncer implements CareOfferingSourceResyncBinder.
func (s *careOfferingService) SetSourcedTemplateResyncer(resyncer CareOfferingSourcedTemplateResyncer) {
	s.sourcedTemplateResyncer = resyncer
}

// NewCareOfferingService builds the service.
func NewCareOfferingService(cfg CareOfferingServiceConfig) CareOfferingService {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &careOfferingService{CareOfferingServiceConfig: cfg}
}

func (s *careOfferingService) List(ctx context.Context) ([]*enrollmentModels.CareOffering, error) {
	offerings, err := s.Repo.ListByTenant(ctx)
	return validateLoadedAvailabilityRules(offerings, err)
}

func (s *careOfferingService) ListByPhase(ctx context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error) {
	if phaseID <= 0 {
		return nil, careOfferingInvalidf("phase_id must be positive")
	}
	offerings, err := s.Repo.ListByPhase(ctx, phaseID)
	return validateLoadedAvailabilityRules(offerings, err)
}

func (s *careOfferingService) ListActiveByPhase(ctx context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error) {
	if phaseID <= 0 {
		return nil, careOfferingInvalidf("phase_id must be positive")
	}
	offerings, err := s.Repo.ListActiveByPhase(ctx, phaseID)
	return validateLoadedAvailabilityRules(offerings, err)
}

func (s *careOfferingService) GetByID(ctx context.Context, id int64) (*enrollmentModels.CareOffering, error) {
	if id <= 0 {
		return nil, ErrCareOfferingNotFound
	}
	offering, err := s.Repo.FindByID(ctx, id)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, ErrCareOfferingNotFound
		}
		return nil, fmt.Errorf("load care offering: %w", err)
	}
	if offering.AvailabilityRule != nil {
		if err := offering.AvailabilityRule.NormalizeAndValidate(); err != nil {
			return nil, fmt.Errorf("care offering %d has an invalid persisted availability rule: %w", offering.ID, err)
		}
	}
	return offering, nil
}

func validateLoadedAvailabilityRules(offerings []*enrollmentModels.CareOffering, loadErr error) ([]*enrollmentModels.CareOffering, error) {
	if loadErr != nil {
		return nil, loadErr
	}
	for _, offering := range offerings {
		if offering.AvailabilityRule != nil {
			if err := offering.AvailabilityRule.NormalizeAndValidate(); err != nil {
				return nil, fmt.Errorf("care offering %d has an invalid persisted availability rule: %w", offering.ID, err)
			}
		}
	}
	return offerings, nil
}

func (s *careOfferingService) validateAvailabilityRule(ctx context.Context, offering *enrollmentModels.CareOffering) error {
	if offering.AvailabilityRule == nil || len(offering.AvailabilityRule.Conditions) == 0 {
		return nil
	}
	if s.Settings == nil {
		return fmt.Errorf("care offering availability validation requires settings service")
	}
	gradeMax, err := s.Settings.ResolveInt(ctx, configModels.KeyEnrollmentGradeLevelMax)
	if err != nil {
		return fmt.Errorf("resolve tenant grade range: %w", err)
	}
	if gradeMax < schoolclass.MinGradeLevel || gradeMax > schoolclass.MaxGradeLevel {
		return fmt.Errorf("tenant grade range is invalid: maximum %d", gradeMax)
	}
	for i, condition := range offering.AvailabilityRule.Conditions {
		for _, grade := range condition.Value {
			if grade > gradeMax {
				return careOfferingInvalidf("availability_rule condition %d contains grade %d outside tenant range 1-%d", i+1, grade, gradeMax)
			}
		}
	}
	return nil
}

func careOfferingInvalidf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrCareOfferingInvalid, fmt.Sprintf(format, args...))
}

func wrapCareOfferingInvalid(err error, format string, args ...any) error {
	return fmt.Errorf("%w: %s: %w", ErrCareOfferingInvalid, fmt.Sprintf(format, args...), err)
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
		return careOfferingInvalidf("an automatically added care offering must allow parent day selection")
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
			return careOfferingInvalidf("automatic trigger offering %d must belong to the same phase", triggerID)
		}
		if autoAddViolatesExclusiveGroup(offering, trigger) {
			return careOfferingInvalidf("automatic trigger offering %d cannot auto-add offering %d in exclusive selection group %q", triggerID, offering.ID, strings.TrimSpace(offering.SelectionGroup))
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
		return nil, careOfferingInvalidf("activity_group_id must be positive when set")
	}
	if err := deps.validate(); err != nil {
		return nil, err
	}

	group, err := deps.activityGroupRepo.FindByID(ctx, activityGroupID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, careOfferingInvalidf("activity_group_id does not reference a template in this tenant")
		}
		return nil, fmt.Errorf("load linked activity group: %w", err)
	}
	if group == nil {
		return nil, careOfferingInvalidf("activity_group_id does not reference a template in this tenant")
	}
	if !group.IsTemplate {
		return nil, careOfferingInvalidf("activity_group_id must reference a timetable template")
	}
	if group.ArchivedAt != nil {
		return nil, careOfferingInvalidf("activity_group_id references an archived timetable template")
	}

	return resolveTemplatePeriodForGroup(ctx, deps, group)
}

func resolveTemplatePeriodForGroup(
	ctx context.Context,
	deps careOfferingTemplateDeps,
	group *activitiesModels.Group,
) (*scheduleModels.CalendarPeriod, error) {
	if group == nil || !group.IsTemplate {
		return nil, careOfferingInvalidf("activity group must be a timetable template")
	}
	schedules, err := deps.activityScheduleRepo.FindByGroupID(ctx, group.ID)
	if err != nil {
		return nil, fmt.Errorf("load timetable template schedules: %w", err)
	}
	if len(schedules) == 0 {
		return nil, careOfferingInvalidf("timetable template must have at least one schedule")
	}

	periodID, err := resolveTemplateSchedulePeriodID(group, schedules)
	if err != nil {
		return nil, err
	}

	period, err := deps.calendarPeriodRepo.FindByID(ctx, periodID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, careOfferingInvalidf("calendar period for timetable template not found")
		}
		return nil, fmt.Errorf("load timetable template calendar period: %w", err)
	}
	if period == nil {
		return nil, careOfferingInvalidf("calendar period for timetable template not found")
	}
	return period, nil
}

func resolveTemplateSchedulePeriodID(
	group *activitiesModels.Group,
	schedules []*activitiesModels.Schedule,
) (int64, error) {
	var periodID *int64
	for _, schedule := range schedules {
		resolvedPeriodID := schedule.CalendarPeriodID
		if resolvedPeriodID == nil {
			resolvedPeriodID = group.CalendarPeriodID
		}
		if resolvedPeriodID == nil {
			return 0, careOfferingInvalidf("timetable template schedules must resolve one calendar_period_id from the schedule or template")
		}
		if periodID == nil {
			id := *resolvedPeriodID
			periodID = &id
			continue
		}
		if *periodID != *resolvedPeriodID {
			return 0, careOfferingInvalidf("timetable template schedules must use one calendar_period_id")
		}
	}
	return *periodID, nil
}

type linkedCareOfferingGroup struct {
	group     *activitiesModels.Group
	period    *scheduleModels.CalendarPeriod
	schedules []*activitiesModels.Schedule
}

// resolveCareOfferingLinkedGroupsForPhase expands a template link to every
// live split-series segment whose recurrence window can produce an occurrence
// during the enrollment phase. A non-template activity remains one group.
func resolveCareOfferingLinkedGroupsForPhase(
	ctx context.Context,
	deps careOfferingTemplateDeps,
	activityGroupID int64,
	phase *enrollmentModels.Phase,
) ([]linkedCareOfferingGroup, error) {
	group, period, err := resolveCareOfferingLinkedGroupPeriod(ctx, deps, activityGroupID)
	if err != nil {
		return nil, err
	}
	if !group.IsTemplate {
		return []linkedCareOfferingGroup{{group: group}}, nil
	}

	series, err := deps.activityGroupRepo.FindTemplateSeries(ctx, activityGroupID)
	if err != nil {
		return nil, fmt.Errorf("load timetable template split series: %w", err)
	}
	segments := make([]linkedCareOfferingGroup, 0, len(series))
	for _, segment := range series {
		linked, include, resolveErr := resolveCareOfferingSeriesSegment(
			ctx,
			deps,
			group,
			period,
			segment,
			phase,
		)
		if resolveErr != nil {
			return nil, resolveErr
		}
		if include {
			segments = append(segments, linked)
		}
	}
	if len(segments) == 0 {
		return nil, careOfferingInvalidf("timetable template has no recurrence segment during the enrollment phase")
	}
	return segments, nil
}

func resolveCareOfferingSeriesSegment(
	ctx context.Context,
	deps careOfferingTemplateDeps,
	root *activitiesModels.Group,
	rootPeriod *scheduleModels.CalendarPeriod,
	segment *activitiesModels.Group,
	phase *enrollmentModels.Phase,
) (linkedCareOfferingGroup, bool, error) {
	if segment == nil {
		return linkedCareOfferingGroup{}, false, nil
	}
	schedules, err := deps.activityScheduleRepo.FindByGroupID(ctx, segment.ID)
	if err != nil {
		return linkedCareOfferingGroup{}, false, fmt.Errorf("load split-series segment %d schedules: %w", segment.ID, err)
	}
	if !schedulesOverlapEnrollmentPhase(schedules, phase) {
		return linkedCareOfferingGroup{}, false, nil
	}

	period := rootPeriod
	if segment.ID != root.ID {
		period, err = resolveTemplatePeriodForGroup(ctx, deps, segment)
		if err != nil {
			return linkedCareOfferingGroup{}, false, fmt.Errorf("resolve split-series segment %d period: %w", segment.ID, err)
		}
	}
	if err := validatePhaseWithinTemplatePeriod(phase, period); err != nil {
		return linkedCareOfferingGroup{}, false, err
	}
	return linkedCareOfferingGroup{
		group:     segment,
		period:    period,
		schedules: schedules,
	}, true, nil
}

func validateCareOfferingTemplateSegments(
	segments []linkedCareOfferingGroup,
	phase *enrollmentModels.Phase,
	days []string,
	requireActivePeriod bool,
) error {
	if len(segments) == 0 || phase == nil || segments[0].group == nil || !segments[0].group.IsTemplate {
		return nil
	}
	weekdays, err := parseCareOfferingWeekdays(days)
	if err != nil {
		return err
	}
	for weekday := activitiesModels.WeekdayMonday; weekday <= activitiesModels.WeekdaySunday; weekday++ {
		if !weekdays[weekday] {
			continue
		}
		if err := validateCareOfferingWeekdayCoverage(segments, phase, weekday, requireActivePeriod); err != nil {
			return err
		}
	}
	return nil
}

func parseCareOfferingWeekdays(days []string) (map[int]bool, error) {
	weekdays := make(map[int]bool, len(days))
	for _, day := range days {
		weekday, ok := enrollmentDayToISOWeekday(day)
		if !ok {
			return nil, careOfferingInvalidf("care offering day %q is invalid", day)
		}
		weekdays[weekday] = true
	}
	if len(weekdays) == 0 {
		return nil, careOfferingInvalidf("a timetable-linked care offering must define at least one available day")
	}
	return weekdays, nil
}

func validateCareOfferingWeekdayCoverage(
	segments []linkedCareOfferingGroup,
	phase *enrollmentModels.Phase,
	weekday int,
	requireActivePeriod bool,
) error {
	occurrences := 0
	for date := phase.ServiceStartDate; !date.After(phase.ServiceEndDate); date = date.AddDays(1) {
		if careOfferingISOWeekday(date) != weekday {
			continue
		}
		occurrences++
		covered, inactiveOnly := careOfferingOccurrenceCovered(segments, date, weekday, requireActivePeriod)
		if covered {
			continue
		}
		if inactiveOnly {
			return careOfferingInvalidf("an active timetable-linked care offering requires an active calendar period")
		}
		return careOfferingInvalidf(
			"timetable recurrence does not cover care offering weekday %d on %s",
			weekday,
			date.String(),
		)
	}
	if occurrences == 0 {
		return careOfferingInvalidf(
			"care offering weekday %d has no occurrence during the enrollment phase",
			weekday,
		)
	}
	return nil
}

func careOfferingISOWeekday(date timezone.Date) int {
	weekday := int(date.Weekday())
	if weekday == 0 {
		return activitiesModels.WeekdaySunday
	}
	return weekday
}

func careOfferingOccurrenceCovered(
	segments []linkedCareOfferingGroup,
	date timezone.Date,
	weekday int,
	requireActivePeriod bool,
) (covered bool, inactiveOnly bool) {
	for _, segment := range segments {
		for _, schedule := range segment.schedules {
			if schedule == nil || schedule.Weekday != weekday || !scheduleCoversDate(schedule, date) ||
				!scheduleService.ShouldMaterializeWeekPattern(schedule.WeekPattern, date, segment.period) {
				continue
			}
			if requireActivePeriod && (segment.period == nil || !segment.period.IsActive) {
				inactiveOnly = true
				continue
			}
			return true, false
		}
	}
	return false, inactiveOnly
}

func scheduleCoversDate(schedule *activitiesModels.Schedule, date timezone.Date) bool {
	if schedule == nil || (schedule.ValidFrom != nil && schedule.ValidFrom.After(date)) {
		return false
	}
	return schedule.ValidUntil == nil || schedule.ValidUntil.After(date)
}

func resolveCareOfferingLinkedGroupPeriod(
	ctx context.Context,
	deps careOfferingTemplateDeps,
	activityGroupID int64,
) (*activitiesModels.Group, *scheduleModels.CalendarPeriod, error) {
	if activityGroupID <= 0 {
		return nil, nil, careOfferingInvalidf("activity_group_id must be positive when set")
	}
	if err := deps.validateActivityGroupLookup(); err != nil {
		return nil, nil, err
	}

	group, err := deps.activityGroupRepo.FindByID(ctx, activityGroupID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, nil, careOfferingInvalidf("activity_group_id does not reference a group in this tenant")
		}
		return nil, nil, fmt.Errorf("load linked activity group: %w", err)
	}
	if group == nil {
		return nil, nil, careOfferingInvalidf("activity_group_id does not reference a group in this tenant")
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
		return wrapCareOfferingInvalid(ErrCareOfferingTemplatePeriodMismatch, "linked timetable template period does not contain the enrollment phase")
	}
	return nil
}

func (s *careOfferingService) validateLinkedTemplate(ctx context.Context, offering *enrollmentModels.CareOffering) error {
	if offering == nil || offering.ActivityGroupID == nil {
		return nil
	}
	deps := careOfferingTemplateDeps{
		activityGroupRepo:    s.ActivityGroupRepo,
		activityScheduleRepo: s.ActivityScheduleRepo,
		calendarPeriodRepo:   s.CalendarPeriodRepo,
	}
	// Preserve the admin catalog's template-only contract. Decision
	// materialization separately supports historical non-template links.
	if _, err := resolveCareOfferingTemplatePeriod(ctx, deps, *offering.ActivityGroupID); err != nil {
		return err
	}
	if s.PhaseRepo == nil {
		return errors.New("phase validation dependency is not configured")
	}
	phase, err := s.PhaseRepo.FindByID(ctx, offering.PhaseID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return careOfferingInvalidf("phase_id does not reference a phase in this tenant")
		}
		return fmt.Errorf("load care offering phase: %w", err)
	}
	if phase == nil {
		return careOfferingInvalidf("phase_id does not reference a phase in this tenant")
	}
	requiresMaterialization, err := s.offeringRequiresMaterialization(ctx, offering)
	if err != nil {
		return err
	}
	return s.validateLinkedTemplateForMaterialization(ctx, offering, phase, deps, requiresMaterialization)
}

func (s *careOfferingService) validateLinkedTemplateForMaterialization(
	ctx context.Context,
	offering *enrollmentModels.CareOffering,
	phase *enrollmentModels.Phase,
	deps careOfferingTemplateDeps,
	requiresMaterialization bool,
) error {
	segments, err := resolveCareOfferingLinkedGroupsForPhase(ctx, deps, *offering.ActivityGroupID, phase)
	if err != nil {
		return err
	}
	if err := validateCareOfferingTemplateSegments(segments, phase, offering.AvailableDays, requiresMaterialization); err != nil {
		return err
	}
	if !requiresMaterialization {
		return nil
	}
	return validateCareOfferingMaterializability(
		ctx,
		s.materializationDeps(),
		segments,
		phase,
		offering.AvailableDays,
		careOfferingMaterializationChange{},
	)
}

// offeringRequiresMaterialization protects both catalog-visible offerings and
// inactive offerings still selected by a non-terminal enrollment request.
// Deactivation is not a cancellation: Decision materializes the persisted
// selection later, so recurrence mutations must keep that path valid.
func (s *careOfferingService) offeringRequiresMaterialization(
	ctx context.Context,
	offering *enrollmentModels.CareOffering,
) (bool, error) {
	if offering == nil {
		return false, errors.New("care offering is required")
	}
	if offering.IsActive {
		return true, nil
	}
	// A not-yet-created inactive draft cannot have request selections.
	if offering.ID <= 0 {
		return false, nil
	}
	if s.RequestChildOfferingRepo == nil {
		return false, errors.New("request child offering repository is not configured")
	}
	count, err := s.RequestChildOfferingRepo.CountMaterializableByCareOffering(ctx, offering.ID)
	if err != nil {
		return false, fmt.Errorf("count materializable care offering selections: %w", err)
	}
	return count > 0, nil
}

func (s *careOfferingService) lockTemplateRecurrence(ctx context.Context) error {
	if s.LockTemplateRecurrence == nil {
		return nil
	}
	if err := s.LockTemplateRecurrence(ctx); err != nil {
		return fmt.Errorf("care offering: lock template recurrence: %w", err)
	}
	return nil
}

// ValidateTemplateOfferingSource implements the pre-write offering-source
// guard declared on CareOfferingSeriesValidator (#2137). Vanished ids are
// tolerated ONLY when they are already stored on the template (rejecting
// those would wedge every edit of a template whose array carries a dangling
// id); a newly submitted unknown id is rejected — without the FK of the
// single-source era, accepting it would persist a dead source with a
// permanently empty roster.
func (s *careOfferingService) ValidateTemplateOfferingSource(ctx context.Context, offeringIDs, storedOfferingIDs []int64, calendarPeriodID *int64) error {
	if s.Repo == nil || s.PhaseRepo == nil || s.CalendarPeriodRepo == nil {
		return errors.New("offering source validation dependencies are not configured")
	}
	_, _, dropped, err := loadValidatedOfferingSources(ctx, s.Repo, s.PhaseRepo, s.CalendarPeriodRepo, offeringIDs, calendarPeriodID, false)
	if err != nil {
		return err
	}
	if len(dropped) == 0 {
		return nil
	}
	stored := make(map[int64]bool, len(storedOfferingIDs))
	for _, id := range storedOfferingIDs {
		stored[id] = true
	}
	for _, id := range dropped {
		if !stored[id] {
			return fmt.Errorf("%w: care offering %d not found", scheduleService.ErrOfferingSourceInvalid, id)
		}
	}
	s.Logger.Warn("offering source validation: ignoring vanished stored source offerings",
		slog.Any("care_offering_ids", dropped),
	)
	return nil
}

// ValidateTemplateSeries checks every care offering linked to any live segment
// in groupID's split lineage against the complete post-split series. The split
// service calls this after creating the successor but before committing, so a
// recurrence edit cannot turn an accepted catalog link into a later approval
// failure.
func (s *careOfferingService) ValidateTemplateSeries(ctx context.Context, groupID int64) error {
	if groupID <= 0 {
		return careOfferingInvalidf("template group id must be positive")
	}
	if s.ActivityGroupRepo == nil || s.Repo == nil || s.PhaseRepo == nil {
		return errors.New("care offering series validation dependencies are not configured")
	}
	series, err := s.ActivityGroupRepo.FindTemplateSeries(ctx, groupID)
	if err != nil {
		return fmt.Errorf("load template series for care offering validation: %w", err)
	}
	// Include groupID even when it was provisionally archived in the current
	// transaction. FindTemplateSeries intentionally returns only live segments,
	// but an offering linked to the row being archived must still be found and
	// rejected before commit.
	groupIDs := careOfferingSeriesGroupIDs(groupID, series)
	offerings, err := s.Repo.ListByActivityGroupIDs(ctx, groupIDs)
	if err != nil {
		return fmt.Errorf("list care offerings linked to template series: %w", err)
	}
	deps := careOfferingTemplateDeps{
		activityGroupRepo:    s.ActivityGroupRepo,
		activityScheduleRepo: s.ActivityScheduleRepo,
		calendarPeriodRepo:   s.CalendarPeriodRepo,
	}
	phases := make(map[int64]*enrollmentModels.Phase)
	for _, offering := range offerings {
		if err := s.validateTemplateSeriesOffering(ctx, deps, phases, offering); err != nil {
			return err
		}
	}
	return nil
}

func careOfferingSeriesGroupIDs(groupID int64, series []*activitiesModels.Group) []int64 {
	groupIDs := make([]int64, 0, len(series)+1)
	groupIDs = append(groupIDs, groupID)
	seen := map[int64]bool{groupID: true}
	for _, segment := range series {
		if segment == nil || seen[segment.ID] {
			continue
		}
		groupIDs = append(groupIDs, segment.ID)
		seen[segment.ID] = true
	}
	return groupIDs
}

func (s *careOfferingService) validateTemplateSeriesOffering(
	ctx context.Context,
	deps careOfferingTemplateDeps,
	phases map[int64]*enrollmentModels.Phase,
	offering *enrollmentModels.CareOffering,
) error {
	if offering == nil || offering.ActivityGroupID == nil {
		return nil
	}
	requiresMaterialization, err := s.offeringRequiresMaterialization(ctx, offering)
	if err != nil {
		return fmt.Errorf("inspect care offering %d request selections: %w", offering.ID, err)
	}
	if !requiresMaterialization {
		return nil
	}
	phase, err := s.careOfferingSeriesPhase(ctx, phases, offering)
	if err != nil {
		return err
	}
	segments, err := resolveCareOfferingLinkedGroupsForPhase(ctx, deps, *offering.ActivityGroupID, phase)
	if err != nil {
		return fmt.Errorf("care offering %d is incompatible with the split template series: %w", offering.ID, err)
	}
	if err := validateCareOfferingTemplateSegments(segments, phase, offering.AvailableDays, requiresMaterialization); err != nil {
		return fmt.Errorf("care offering %d is incompatible with the split template series: %w", offering.ID, err)
	}
	if err := validateCareOfferingMaterializability(
		ctx,
		s.materializationDeps(),
		segments,
		phase,
		offering.AvailableDays,
		careOfferingMaterializationChange{},
	); err != nil {
		return fmt.Errorf("care offering %d is incompatible with the split template series: %w", offering.ID, err)
	}
	return nil
}

func (s *careOfferingService) careOfferingSeriesPhase(
	ctx context.Context,
	phases map[int64]*enrollmentModels.Phase,
	offering *enrollmentModels.CareOffering,
) (*enrollmentModels.Phase, error) {
	if phase := phases[offering.PhaseID]; phase != nil {
		return phase, nil
	}
	phase, err := s.PhaseRepo.FindByID(ctx, offering.PhaseID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, careOfferingInvalidf("care offering %d references an unavailable phase", offering.ID)
		}
		return nil, fmt.Errorf("load phase for care offering %d: %w", offering.ID, err)
	}
	if phase == nil {
		return nil, careOfferingInvalidf("care offering %d references an unavailable phase", offering.ID)
	}
	phases[offering.PhaseID] = phase
	return phase, nil
}

func (s *careOfferingService) Create(ctx context.Context, offering *enrollmentModels.CareOffering) (*enrollmentModels.CareOffering, error) {
	if offering == nil {
		return nil, careOfferingInvalidf("offering is required")
	}
	if err := offering.Validate(); err != nil {
		return nil, wrapCareOfferingInvalid(err, "validate care offering")
	}
	if err := s.validateAvailabilityRule(ctx, offering); err != nil {
		return nil, err
	}
	if err := s.lockTemplateRecurrence(ctx); err != nil {
		return nil, err
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
		return careOfferingInvalidf("offering with valid id is required")
	}
	if err := offering.Validate(); err != nil {
		return wrapCareOfferingInvalid(err, "validate care offering")
	}
	if err := s.validateAvailabilityRule(ctx, offering); err != nil {
		return err
	}
	if err := s.lockTemplateRecurrence(ctx); err != nil {
		return err
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
	if err := s.resyncSourcedTemplates(ctx, offering.ID); err != nil {
		return err
	}
	s.Logger.Info("care offering updated", slog.Int64("offering_id", offering.ID))
	return nil
}

// resyncSourcedTemplates re-reconciles every template sourcing this offering
// after an update (#2147 review): changed days or a moved phase change the
// wanted roster, and the sourced enrollment rows plus already-materialized
// occurrences must follow immediately, not at the next unrelated template
// save. An edit that makes the source incompatible with a template's planning
// period is rejected outright (#2147 review round 7): committing it would
// leave that template's sourced rows and materialized occurrences stranded,
// with every later resync skipping the template. Runs under the recurrence
// lock the update already holds; history before today stays untouched.
func (s *careOfferingService) resyncSourcedTemplates(ctx context.Context, offeringID int64) error {
	if s.sourcedTemplateResyncer == nil {
		// Focused service tests may run without the enrollment decision
		// wiring; the factory always injects it.
		s.Logger.Warn("care offering update: sourced-template resyncer not configured; sourced rosters may be stale",
			slog.Int64("offering_id", offeringID))
		return nil
	}
	if err := s.sourcedTemplateResyncer.ResyncTemplatesSourcedFromOffering(ctx, offeringID, timezone.TodayDate()); err != nil {
		if errors.Is(err, scheduleService.ErrOfferingSourceInvalid) {
			// TenantTxMiddleware commits ordinary 4xx responses. Mark the
			// ambient transaction so the already-written offering update is
			// discarded together with the rejection.
			tenant.MarkRollback(ctx)
			return fmt.Errorf("%w: %w", ErrCareOfferingInvalid, err)
		}
		return fmt.Errorf("care offering update: resync sourced templates: %w", err)
	}
	return nil
}

func (s *careOfferingService) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return careOfferingInvalidf("id must be positive")
	}
	// Retire sourced rosters BEFORE the row delete: the FK's ON DELETE SET
	// NULL only degrades the templates to manual rosters and would leave
	// their bounded offering-derived enrollment rows and materialized
	// occurrences behind (#2147 review round 11). The recurrence lock
	// serializes the retirement with concurrent approvals/template saves.
	if err := s.lockTemplateRecurrence(ctx); err != nil {
		return err
	}
	if err := s.detachSourcedTemplates(ctx, id); err != nil {
		return err
	}
	if err := s.Repo.Delete(ctx, id); err != nil {
		return err
	}
	s.Logger.Info("care offering deleted", slog.Int64("offering_id", id))
	return nil
}

// detachSourcedTemplates retires the rosters of every template sourcing the
// offering ahead of its deletion; see CareOfferingSourcedTemplateResyncer.
func (s *careOfferingService) detachSourcedTemplates(ctx context.Context, offeringID int64) error {
	if s.sourcedTemplateResyncer == nil {
		// Focused service tests may run without the enrollment decision
		// wiring; the factory always injects it.
		s.Logger.Warn("care offering delete: sourced-template resyncer not configured; sourced rosters may be orphaned",
			slog.Int64("offering_id", offeringID))
		return nil
	}
	if err := s.sourcedTemplateResyncer.DetachTemplatesSourcedFromOffering(ctx, offeringID, timezone.TodayDate()); err != nil {
		return fmt.Errorf("care offering delete: detach sourced templates: %w", err)
	}
	return nil
}

// Clone copies a care offering into a new row scoped to a target phase.
// Offering-level fields are preserved except cross-phase timetable template
// links; ID is reset so the DB assigns a fresh BIGSERIAL, and phase_id is
// repointed at the target.
func (s *careOfferingService) Clone(ctx context.Context, sourceID int64, targetPhaseID int64) (*enrollmentModels.CareOffering, error) {
	if sourceID <= 0 {
		return nil, careOfferingInvalidf("source id must be positive")
	}
	if targetPhaseID <= 0 {
		return nil, careOfferingInvalidf("target phase id must be positive")
	}
	if err := s.lockTemplateRecurrence(ctx); err != nil {
		return nil, err
	}

	source, err := s.Repo.FindByID(ctx, sourceID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, careOfferingInvalidf("source care offering does not exist")
		}
		return nil, fmt.Errorf("clone: source lookup: %w", err)
	}

	clone := *source
	clone.ID = 0 // BIGSERIAL - let the DB assign
	clone.CreatedAt = time.Time{}
	clone.UpdatedAt = time.Time{}
	clone.PhaseID = targetPhaseID
	if source.PhaseID != targetPhaseID && clone.ActivityGroupID != nil {
		clone.ActivityGroupID = nil
	}
	clone.AutoAddTriggerOfferingIDs = nil
	if err := s.validateAvailabilityRule(ctx, &clone); err != nil {
		return nil, fmt.Errorf("clone: validate availability rule: %w", err)
	}
	if err := s.validateLinkedTemplate(ctx, &clone); err != nil {
		return nil, fmt.Errorf("clone: validate linked template: %w", err)
	}
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

// bookingStatsWindow is the half-open date range ListBookingStats counts in.
// It reproduces applyCapacityOverflowCore's window so the displayed occupancy
// is the same number the capacity gate will apply at save time: from today
// (or the phase start, if the phase has not begun) through the last service
// day inclusive.
//
// A phase whose service window has already ended would otherwise yield an
// empty range. Rather than reporting a meaningless zero, the window collapses
// onto the final service day so the dialog shows the phase's end state.
func bookingStatsWindow(phase *enrollmentModels.Phase) (from, until timezone.Date) {
	today := timezone.TodayDate()
	if phase == nil || phase.ServiceEndDate.IsZero() {
		return today, today.AddDays(1)
	}
	from = today
	if phase.ServiceStartDate.After(from) {
		from = phase.ServiceStartDate
	}
	until = phase.ServiceEndDate.AddDays(1)
	if !from.Before(until) {
		return phase.ServiceEndDate, phase.ServiceEndDate.AddDays(1)
	}
	return from, until
}

func (s *careOfferingService) ListBookingStats(ctx context.Context, phaseID int64) ([]CareOfferingBookingStat, error) {
	if phaseID <= 0 {
		return nil, careOfferingInvalidf("phase_id must be positive")
	}
	if s.RequestChildOfferingRepo == nil {
		return nil, errors.New("request child offering repository is not configured")
	}
	if s.PhaseRepo == nil {
		return nil, errors.New("phase repository is not configured")
	}
	phase, err := s.PhaseRepo.FindByID(ctx, phaseID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, careOfferingInvalidf("phase does not exist")
		}
		return nil, fmt.Errorf("booking stats: phase lookup: %w", err)
	}
	offerings, err := s.Repo.ListByPhase(ctx, phaseID)
	if err != nil {
		return nil, fmt.Errorf("booking stats: list offerings: %w", err)
	}
	stats := make([]CareOfferingBookingStat, 0, len(offerings))
	if len(offerings) == 0 {
		return stats, nil
	}

	from, until := bookingStatsWindow(phase)
	ids := make([]int64, 0, len(offerings))
	for _, offering := range offerings {
		ids = append(ids, offering.ID)
	}
	gradeCounts, err := s.RequestChildOfferingRepo.CountActiveGradeLevelsByCareOfferingIDs(ctx, ids, from, until)
	if err != nil {
		return nil, fmt.Errorf("booking stats: count grade levels: %w", err)
	}
	grades := make(map[int64]map[int]int, len(offerings))
	unknown := make(map[int64]int, len(offerings))
	for _, row := range gradeCounts {
		if row == nil {
			continue
		}
		if row.GradeLevel == nil {
			unknown[row.CareOfferingID] += row.Count
			continue
		}
		if grades[row.CareOfferingID] == nil {
			grades[row.CareOfferingID] = make(map[int]int)
		}
		grades[row.CareOfferingID][int(*row.GradeLevel)] += row.Count
	}

	peaks, err := s.RequestChildOfferingRepo.CountMaxActiveByCareOfferingIDsInRange(ctx, ids, from, until)
	if err != nil {
		return nil, fmt.Errorf("booking stats: count peak occupancy: %w", err)
	}

	for _, offering := range offerings {
		// Absent key = no booking overlaps the window = zero occupancy.
		booked := peaks[offering.ID]
		byGrade := grades[offering.ID]
		if byGrade == nil {
			byGrade = map[int]int{}
		}
		stats = append(stats, CareOfferingBookingStat{
			OfferingID:        offering.ID,
			Capacity:          offering.Capacity,
			Booked:            booked,
			GradeLevels:       byGrade,
			UnknownGradeCount: unknown[offering.ID],
		})
	}
	return stats, nil
}

// CloneCatalogForRollover implements the rollover-facing catalog copy.
// See the interface doc for the contract. Callers run it inside the
// rollover's tenant transaction, so a validation failure on any single
// offering rolls back the whole follow-up phase.
func (s *careOfferingService) CloneCatalogForRollover(ctx context.Context, sourcePhaseID int64, targetPhaseID int64, carriedOfferingIDs []int64) (map[int64]int64, error) {
	if sourcePhaseID <= 0 {
		return nil, careOfferingInvalidf("source phase id must be positive")
	}
	if targetPhaseID <= 0 {
		return nil, careOfferingInvalidf("target phase id must be positive")
	}
	if sourcePhaseID == targetPhaseID {
		return nil, careOfferingInvalidf("source and target phase must differ")
	}
	if err := s.lockTemplateRecurrence(ctx); err != nil {
		return nil, err
	}

	sources, err := s.Repo.ListByPhase(ctx, sourcePhaseID)
	if err != nil {
		return nil, fmt.Errorf("rollover catalog clone: list source offerings: %w", err)
	}
	sourceByID := make(map[int64]*enrollmentModels.CareOffering, len(sources)+len(carriedOfferingIDs))
	carriedByID := make(map[int64]bool, len(carriedOfferingIDs))
	for _, offeringID := range carriedOfferingIDs {
		carriedByID[offeringID] = true
	}
	for _, source := range sources {
		sourceByID[source.ID] = source
	}
	for _, offeringID := range carriedOfferingIDs {
		if _, ok := sourceByID[offeringID]; ok {
			continue
		}
		source, findErr := s.Repo.FindByID(ctx, offeringID)
		if findErr != nil {
			return nil, fmt.Errorf("rollover catalog clone: load carried offering %d: %w", offeringID, findErr)
		}
		sources = append(sources, source)
		sourceByID[offeringID] = source
	}
	if err := validateCatalogGroupRuleConsistency(sources); err != nil {
		return nil, fmt.Errorf("rollover catalog clone: %w", err)
	}

	mapping := make(map[int64]int64, len(sources))
	for _, source := range sources {
		requiresMaterialization := source.IsActive || carriedByID[source.ID]
		clone := *source
		clone.ID = 0 // BIGSERIAL — let the DB assign
		clone.CreatedAt = time.Time{}
		clone.UpdatedAt = time.Time{}
		clone.PhaseID = targetPhaseID
		// Triggers reference offering IDs; they are remapped and written
		// in the second pass once every clone has its ID.
		clone.AutoAddTriggerOfferingIDs = nil
		if err := s.validateAvailabilityRule(ctx, &clone); err != nil {
			return nil, fmt.Errorf("rollover catalog clone: offering %q (%d): %w", source.Name, source.ID, err)
		}
		if err := s.validateRolloverCloneLinkedGroup(ctx, &clone, requiresMaterialization); err != nil {
			return nil, fmt.Errorf("rollover catalog clone: offering %q (%d): %w", source.Name, source.ID, err)
		}
		// Group-rule consistency was checked across the complete clone set
		// before the first row was inserted.
		if err := s.Repo.Create(ctx, &clone); err != nil {
			return nil, fmt.Errorf("rollover catalog clone: offering %q (%d): %w", source.Name, source.ID, err)
		}
		mapping[source.ID] = clone.ID
	}

	for _, source := range sources {
		if len(source.AutoAddTriggerOfferingIDs) == 0 {
			continue
		}
		remapped := make([]int64, 0, len(source.AutoAddTriggerOfferingIDs))
		for _, triggerID := range source.AutoAddTriggerOfferingIDs {
			cloneTriggerID, ok := mapping[triggerID]
			if !ok {
				// Save-path validation pins triggers to the same phase,
				// so this only fires for legacy rows. Carrying a
				// cross-phase trigger forward would create exactly the
				// mixed-phase state the rollover exists to prevent.
				s.Logger.Warn("rollover catalog clone: dropping trigger outside the source phase",
					slog.Int64("offering_id", source.ID),
					slog.Int64("trigger_offering_id", triggerID))
				continue
			}
			remapped = append(remapped, cloneTriggerID)
		}
		if len(remapped) == 0 {
			continue
		}
		if err := s.Repo.ReplaceAutoAddTriggers(ctx, mapping[source.ID], remapped); err != nil {
			return nil, fmt.Errorf("rollover catalog clone: remap triggers for offering %d: %w", source.ID, err)
		}
	}

	s.Logger.Info("care offering catalog cloned for rollover",
		slog.Int64("source_phase_id", sourcePhaseID),
		slog.Int64("target_phase_id", targetPhaseID),
		slog.Int("offering_count", len(mapping)))
	return mapping, nil
}

// validateRolloverCloneLinkedGroup validates a clone's activity-group link
// with the same tolerance the decision materialization applies: a linked
// TEMPLATE must cover the target phase (full validateLinkedTemplate), while
// a historical non-template link — which the admin catalog no longer allows
// but Decide still materializes — is carried verbatim. Rejecting those would
// wedge every rollover of a phase carrying pre-template-era links.
func (s *careOfferingService) validateRolloverCloneLinkedGroup(ctx context.Context, clone *enrollmentModels.CareOffering, requiresMaterialization bool) error {
	if clone == nil || clone.ActivityGroupID == nil {
		return nil
	}
	deps := careOfferingTemplateDeps{
		activityGroupRepo:    s.ActivityGroupRepo,
		activityScheduleRepo: s.ActivityScheduleRepo,
		calendarPeriodRepo:   s.CalendarPeriodRepo,
	}
	if err := deps.validateActivityGroupLookup(); err != nil {
		return err
	}
	group, err := s.ActivityGroupRepo.FindByID(ctx, *clone.ActivityGroupID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return careOfferingInvalidf("activity_group_id does not reference a group in this tenant")
		}
		return fmt.Errorf("load linked activity group: %w", err)
	}
	if group == nil || !group.IsTemplate {
		return nil
	}
	if s.PhaseRepo == nil {
		return errors.New("phase validation dependency is not configured")
	}
	phase, err := s.PhaseRepo.FindByID(ctx, clone.PhaseID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return careOfferingInvalidf("phase_id does not reference a phase in this tenant")
		}
		return fmt.Errorf("load care offering phase: %w", err)
	}
	if phase == nil {
		return careOfferingInvalidf("phase_id does not reference a phase in this tenant")
	}
	return s.validateLinkedTemplateForMaterialization(ctx, clone, phase, deps, requiresMaterialization)
}

func validateCatalogGroupRuleConsistency(offerings []*enrollmentModels.CareOffering) error {
	rules := make(map[string]string)
	for _, offering := range offerings {
		group := strings.TrimSpace(offering.SelectionGroup)
		if group == "" {
			continue
		}
		rule := normalizeSelectionRule(offering.SelectionRule)
		if existing, ok := rules[group]; ok && existing != rule {
			return fmt.Errorf("%w: group %q uses both %q and %q", ErrCareOfferingGroupRuleConflict, group, existing, rule)
		}
		rules[group] = rule
	}
	return nil
}
