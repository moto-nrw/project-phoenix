package enrollment

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// OfferingRosterResyncer is the factory-facing view of the offering-source
// roster resync (#2137). The decision service implements it; the factory
// injects it into the schedule layer's TimetableDataDependencies as a func
// hook to avoid an enrollment→schedule package cycle.
type OfferingRosterResyncer interface {
	ResyncTemplateOfferingRoster(ctx context.Context, in scheduleService.OfferingRosterResyncInput) error
}

// sourcedRosterTarget is one wanted roster row of an offering-sourced
// template: the child's student and the draft describing period + weekdays.
type sourcedRosterTarget struct {
	studentID int64
	draft     *careEnrollmentDraft
}

// ResyncTemplateOfferingRoster implements the schedule layer's
// ResyncOfferingRoster hook (#2137): it reconciles the offering-sourced
// student_enrollments rows of one template with the offering's currently
// approved enrollments, applying the template's Jahrgang filter.
//
//   - rows for children that still match keep their row (extended to the
//     phase end when a dated switch had capped it earlier)
//   - rows for children that no longer match are deleted (not yet effective)
//     or capped at EffectiveFrom (already started)
//   - missing children are seeded via the same draft/persist shapes the
//     decision fan-out uses, so both write paths stay byte-compatible
//   - rows fed by a legacy CareOffering.ActivityGroupID link pointing at the
//     same template are protected and never touched
//
// Runs inside the template save's tenant transaction; the caller already
// holds the tenant recurrence lock.
func (s *decisionService) ResyncTemplateOfferingRoster(ctx context.Context, in scheduleService.OfferingRosterResyncInput) error {
	if in.TemplateID <= 0 {
		return fmt.Errorf("%w: template id is required", scheduleService.ErrOfferingSourceInvalid)
	}
	if in.EffectiveFrom.IsZero() {
		return fmt.Errorf("%w: effective_from is required", scheduleService.ErrOfferingSourceInvalid)
	}
	if !s.hasEnrollmentMaterializationDependencies() {
		return fmt.Errorf("offering roster resync: enrollment repositories are not configured")
	}

	var phase *enrollmentModels.Phase
	wanted := make(map[int64]*sourcedRosterTarget)
	if in.OfferingID != nil {
		offering, offeringPhase, err := s.loadOfferingSource(ctx, *in.OfferingID, in.CalendarPeriodID)
		if err != nil {
			return err
		}
		phase = offeringPhase
		wanted, err = s.wantedSourcedRosterTargets(ctx, offering, phase, in)
		if err != nil {
			return err
		}
	}

	protectedChildren, err := s.legacyLinkedChildIDs(ctx, in.TemplateID, in.EffectiveFrom)
	if err != nil {
		return err
	}

	rows, err := s.StudentEnrollmentRepo.FindByGroupID(ctx, in.TemplateID)
	if err != nil {
		return fmt.Errorf("offering roster resync: load template enrollments: %w", err)
	}
	for _, row := range rows {
		if err := s.reconcileSourcedRosterRow(ctx, row, wanted, protectedChildren, phase, in.EffectiveFrom); err != nil {
			return err
		}
	}

	childIDs := make([]int64, 0, len(wanted))
	for childID := range wanted {
		childIDs = append(childIDs, childID)
	}
	sort.Slice(childIDs, func(i, j int) bool { return childIDs[i] < childIDs[j] })
	for _, childID := range childIDs {
		target := wanted[childID]
		row := studentEnrollmentFromCareDraft(childID, target.studentID, phase, target.draft, &in.EffectiveFrom)
		if err := row.Validate(); err != nil {
			return fmt.Errorf("offering roster resync: validate seeded enrollment: %w", err)
		}
		if err := s.StudentEnrollmentRepo.Create(ctx, row); err != nil {
			return fmt.Errorf("offering roster resync: create seeded enrollment: %w", err)
		}
	}
	return nil
}

// loadOfferingSource resolves and validates the source offering: it must
// exist in this tenant, and its phase's service window must lie within the
// template's calendar period (when the template is period-pinned) — otherwise
// the seeded rows could never materialize and the editor would silently save
// a dead rule.
func (s *decisionService) loadOfferingSource(
	ctx context.Context,
	offeringID int64,
	calendarPeriodID *int64,
) (*enrollmentModels.CareOffering, *enrollmentModels.Phase, error) {
	offering, err := s.CareOfferingRepo.FindByID(ctx, offeringID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, nil, fmt.Errorf("%w: care offering %d not found", scheduleService.ErrOfferingSourceInvalid, offeringID)
		}
		return nil, nil, fmt.Errorf("offering roster resync: load offering: %w", err)
	}
	if offering == nil {
		return nil, nil, fmt.Errorf("%w: care offering %d not found", scheduleService.ErrOfferingSourceInvalid, offeringID)
	}
	phase, err := s.PhaseRepo.FindByID(ctx, offering.PhaseID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, nil, fmt.Errorf("%w: enrollment phase of care offering %d not found", scheduleService.ErrOfferingSourceInvalid, offeringID)
		}
		return nil, nil, fmt.Errorf("offering roster resync: load phase: %w", err)
	}
	if calendarPeriodID != nil {
		period, err := s.CalendarPeriodRepo.FindByID(ctx, *calendarPeriodID)
		if err != nil {
			if modelBase.IsNoRows(err) {
				return nil, nil, fmt.Errorf("%w: calendar period %d not found", scheduleService.ErrOfferingSourceInvalid, *calendarPeriodID)
			}
			return nil, nil, fmt.Errorf("offering roster resync: load calendar period: %w", err)
		}
		if err := validatePhaseWithinTemplatePeriod(phase, period); err != nil {
			return nil, nil, fmt.Errorf("%w: %s", scheduleService.ErrOfferingSourceInvalid, err.Error())
		}
	}
	return offering, phase, nil
}

// wantedSourcedRosterTargets builds the desired roster of the sourced
// template: every approved, grade-matching child of the offering with the
// weekday set their enrollment selects.
func (s *decisionService) wantedSourcedRosterTargets(
	ctx context.Context,
	offering *enrollmentModels.CareOffering,
	phase *enrollmentModels.Phase,
	in scheduleService.OfferingRosterResyncInput,
) (map[int64]*sourcedRosterTarget, error) {
	children, err := s.RequestChildOfferingRepo.ListApprovedChildrenByCareOfferingIDs(ctx, []int64{offering.ID}, in.EffectiveFrom)
	if err != nil {
		return nil, fmt.Errorf("offering roster resync: list approved children: %w", err)
	}
	wanted := make(map[int64]*sourcedRosterTarget, len(children))
	for _, child := range children {
		grade := gradeLevelFromSchoolClass(child.SchoolClass)
		if !gradeFilterMatches(in.GradeLevels, grade) {
			continue
		}
		days, err := effectiveOfferingDaysForEnrollment(offering, child.Link)
		if err != nil {
			return nil, fmt.Errorf("offering roster resync: resolve days for request child %d: %w", child.Link.RequestChildID, err)
		}
		draft, err := sourcedTemplateDraft(in.TemplateID, in.CalendarPeriodID, days)
		if err != nil {
			return nil, err
		}
		// A child can hold several sequential links to the same offering
		// (dated switch). One roster row per child suffices: merge weekday
		// sets, widest wins.
		if existing, ok := wanted[child.Link.RequestChildID]; ok {
			mergeSourcedDraftDays(existing.draft, draft)
			continue
		}
		wanted[child.Link.RequestChildID] = &sourcedRosterTarget{studentID: child.StudentID, draft: draft}
	}
	return wanted, nil
}

// reconcileSourcedRosterRow applies keep/cap/delete to one existing row.
// Manual rows (no request-child tag), history, and rows protected by a
// legacy offering link are left untouched.
func (s *decisionService) reconcileSourcedRosterRow(
	ctx context.Context,
	row *activities.StudentEnrollment,
	wanted map[int64]*sourcedRosterTarget,
	protectedChildren map[int64]bool,
	phase *enrollmentModels.Phase,
	effectiveFrom timezone.Date,
) error {
	if row == nil || row.EnrollmentRequestChildID == nil {
		return nil
	}
	if row.ValidUntil != nil && !row.ValidUntil.After(effectiveFrom) {
		return nil // history stays
	}
	childID := *row.EnrollmentRequestChildID
	if target, ok := wanted[childID]; ok && careDraftMatchesEnrollment(target.draft, row) {
		delete(wanted, childID)
		if phase != nil {
			phaseEndExclusive := phase.ServiceEndDate.AddDays(1)
			if row.ValidUntil != nil && row.ValidUntil.Before(phaseEndExclusive) {
				if err := s.StudentEnrollmentRepo.SetValidUntilByID(ctx, row.ID, phaseEndExclusive); err != nil {
					return fmt.Errorf("offering roster resync: extend retained enrollment: %w", err)
				}
			}
		}
		return nil
	}
	if _, stillWanted := wanted[childID]; !stillWanted && protectedChildren[childID] {
		return nil
	}
	if !row.ValidFrom.Before(effectiveFrom) {
		if err := s.StudentEnrollmentRepo.Delete(ctx, row.ID); err != nil {
			return fmt.Errorf("offering roster resync: delete enrollment: %w", err)
		}
		return nil
	}
	if err := s.StudentEnrollmentRepo.SetValidUntilByID(ctx, row.ID, effectiveFrom); err != nil {
		return fmt.Errorf("offering roster resync: cap enrollment: %w", err)
	}
	return nil
}

// legacyLinkedChildIDs returns the request children whose roster rows on this
// template are owned by the legacy CareOffering.ActivityGroupID feed (#1651).
// The resync must never rewrite those: their lifecycle belongs to the
// decision/adjustment flows of THAT offering.
func (s *decisionService) legacyLinkedChildIDs(
	ctx context.Context,
	templateID int64,
	onOrAfter timezone.Date,
) (map[int64]bool, error) {
	offerings, err := s.CareOfferingRepo.ListByActivityGroupIDs(ctx, []int64{templateID})
	if err != nil {
		return nil, fmt.Errorf("offering roster resync: list legacy-linked offerings: %w", err)
	}
	if len(offerings) == 0 {
		return map[int64]bool{}, nil
	}
	offeringIDs := make([]int64, 0, len(offerings))
	for _, offering := range offerings {
		if offering != nil {
			offeringIDs = append(offeringIDs, offering.ID)
		}
	}
	children, err := s.RequestChildOfferingRepo.ListApprovedChildrenByCareOfferingIDs(ctx, offeringIDs, onOrAfter)
	if err != nil {
		return nil, fmt.Errorf("offering roster resync: list legacy-linked children: %w", err)
	}
	protected := make(map[int64]bool, len(children))
	for _, child := range children {
		protected[child.Link.RequestChildID] = true
	}
	return protected, nil
}

// sourcedTemplateDraft builds the careEnrollmentDraft for one child on one
// sourced template, mirroring mergeCareEnrollmentDraft's weekday rules: an
// empty day list means every weekday.
func sourcedTemplateDraft(templateID int64, calendarPeriodID *int64, days []string) (*careEnrollmentDraft, error) {
	var periodID *int64
	if calendarPeriodID != nil {
		cloned := *calendarPeriodID
		periodID = &cloned
	}
	draft := &careEnrollmentDraft{
		activityGroupID:  templateID,
		calendarPeriodID: periodID,
		selectedWeekday:  make(map[int]bool, len(days)),
	}
	if len(days) == 0 {
		draft.allWeekdays = true
		return draft, nil
	}
	for _, day := range days {
		weekday, ok := enrollmentDayToISOWeekday(day)
		if !ok {
			return nil, fmt.Errorf("offering roster resync: invalid selected day %q", day)
		}
		draft.selectedWeekday[weekday] = true
	}
	return draft, nil
}

// mergeSourcedDraftDays widens target's weekday set by other's. Any all-days
// draft makes the merge all-days.
func mergeSourcedDraftDays(target, other *careEnrollmentDraft) {
	if target.allWeekdays {
		return
	}
	if other.allWeekdays {
		target.allWeekdays = true
		target.selectedWeekday = make(map[int]bool)
		return
	}
	for weekday := range other.selectedWeekday {
		target.selectedWeekday[weekday] = true
	}
}

// gradeLevelFromSchoolClass derives the numeric Jahrgang from the free-text
// school class ("3a" -> 3). Classes without a grade number ("Bienen") yield
// nil — a set filter then never matches, mirroring
// Group.MatchesSourceGradeFilter.
func gradeLevelFromSchoolClass(schoolClass string) *int16 {
	prefix := schoolclass.GradePrefix(schoolClass)
	if prefix == "" {
		return nil
	}
	parsed, err := strconv.Atoi(prefix)
	if err != nil || parsed < schoolclass.MinGradeLevel || parsed > schoolclass.MaxGradeLevel {
		return nil
	}
	grade := int16(parsed)
	return &grade
}

// gradeFilterMatches mirrors activities.Group.MatchesSourceGradeFilter for a
// raw filter slice.
func gradeFilterMatches(levels []int, grade *int16) bool {
	if len(levels) == 0 {
		return true
	}
	if grade == nil {
		return false
	}
	for _, level := range levels {
		if level == int(*grade) {
			return true
		}
	}
	return false
}

// logSkippedSourcedTemplate reports a sourced template the decision fan-out
// had to skip (misconfigured period/schedules). Approvals must not fail on a
// template drifted since its save; the editor surfaces the mismatch.
func (s *decisionService) logSkippedSourcedTemplate(templateID, offeringID int64, reason string, err error) {
	attrs := []any{
		slog.Int64("template_id", templateID),
		slog.Int64("care_offering_id", offeringID),
		slog.String("reason", reason),
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}
	s.Logger.Warn("decision: skipping offering-sourced template", attrs...)
}

// OfferingSourcedTemplate is one template already sourcing an offering,
// exposed so the editor can warn about overlapping Jahrgang subsets before
// the admin saves a second Regeltermin over the same children.
type OfferingSourcedTemplate struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	GradeLevels []int  `json:"grade_levels,omitempty"`
}

// OfferingSourceOption is one selectable Betreuungsangebot in the Regeltermin
// editor (#2137), with per-grade counts of approved children (live filter
// preview) and the templates already sourcing it (overlap Hinweis).
type OfferingSourceOption struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	PhaseID   int64  `json:"phase_id"`
	PhaseName string `json:"phase_name"`
	// TotalCount is the number of distinct approved children currently or
	// prospectively enrolled in the offering.
	TotalCount int `json:"total_count"`
	// GradeCounts maps Jahrgang → approved children; key 0 collects children
	// whose school class carries no derivable grade number.
	GradeCounts            map[int]int               `json:"grade_counts"`
	SourcedTemplates       []OfferingSourcedTemplate `json:"sourced_templates"`
	LegacyLinkedTemplateID *int64                    `json:"legacy_linked_template_id,omitempty"`
}

// OfferingSourceOptionLister is the api/timetable-facing view of the editor
// support endpoint.
type OfferingSourceOptionLister interface {
	ListOfferingSourceOptions(ctx context.Context, calendarPeriodID *int64) ([]OfferingSourceOption, error)
}

// ListOfferingSourceOptions returns the offerings an admin may pick as a
// Regeltermin source. With a calendar period given, only offerings whose
// phase's service window lies within that period qualify — a source outside
// the Planungszeitraum could never materialize a single occurrence.
func (s *decisionService) ListOfferingSourceOptions(ctx context.Context, calendarPeriodID *int64) ([]OfferingSourceOption, error) {
	if s.CareOfferingRepo == nil || s.PhaseRepo == nil || s.RequestChildOfferingRepo == nil || s.ActivityGroupRepo == nil {
		return nil, fmt.Errorf("offering source options: repositories are not configured")
	}
	offerings, err := s.CareOfferingRepo.ListByTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("offering source options: list offerings: %w", err)
	}
	phases, err := s.offeringSourcePhases(ctx, calendarPeriodID)
	if err != nil {
		return nil, err
	}

	selected := make([]*enrollmentModels.CareOffering, 0, len(offerings))
	offeringIDs := make([]int64, 0, len(offerings))
	for _, offering := range offerings {
		if offering == nil || !offering.IsActive {
			continue
		}
		if _, ok := phases[offering.PhaseID]; !ok {
			continue
		}
		selected = append(selected, offering)
		offeringIDs = append(offeringIDs, offering.ID)
	}
	children, err := s.RequestChildOfferingRepo.ListApprovedChildrenByCareOfferingIDs(ctx, offeringIDs, timezone.TodayDate())
	if err != nil {
		return nil, fmt.Errorf("offering source options: list approved children: %w", err)
	}
	counts := groupOfferingGradeCounts(children)

	options := make([]OfferingSourceOption, 0, len(selected))
	for _, offering := range selected {
		option := OfferingSourceOption{
			ID:                     offering.ID,
			Name:                   offering.Name,
			PhaseID:                offering.PhaseID,
			GradeCounts:            map[int]int{},
			SourcedTemplates:       []OfferingSourcedTemplate{},
			LegacyLinkedTemplateID: offering.ActivityGroupID,
		}
		if phase := phases[offering.PhaseID]; phase != nil {
			option.PhaseName = phase.Name
		}
		if c := counts[offering.ID]; c != nil {
			option.TotalCount = c.total
			option.GradeCounts = c.byGrade
		}
		templates, err := s.ActivityGroupRepo.FindTemplatesBySourceOffering(ctx, offering.ID)
		if err != nil {
			return nil, fmt.Errorf("offering source options: list sourced templates: %w", err)
		}
		for _, tmpl := range templates {
			if tmpl == nil {
				continue
			}
			option.SourcedTemplates = append(option.SourcedTemplates, OfferingSourcedTemplate{
				ID:          tmpl.ID,
				Name:        tmpl.Name,
				GradeLevels: tmpl.SourceGradeLevels,
			})
		}
		options = append(options, option)
	}
	return options, nil
}

// offeringSourcePhases returns the tenant's phases keyed by id, restricted to
// those fitting the calendar period when one is given.
func (s *decisionService) offeringSourcePhases(ctx context.Context, calendarPeriodID *int64) (map[int64]*enrollmentModels.Phase, error) {
	phases, err := s.PhaseRepo.ListByTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("offering source options: list phases: %w", err)
	}
	var period *scheduleModels.CalendarPeriod
	if calendarPeriodID != nil {
		if s.CalendarPeriodRepo == nil {
			return nil, fmt.Errorf("offering source options: calendar period repository is not configured")
		}
		period, err = s.CalendarPeriodRepo.FindByID(ctx, *calendarPeriodID)
		if err != nil {
			if modelBase.IsNoRows(err) {
				return nil, fmt.Errorf("%w: calendar period %d not found", scheduleService.ErrOfferingSourceInvalid, *calendarPeriodID)
			}
			return nil, fmt.Errorf("offering source options: load calendar period: %w", err)
		}
	}
	byID := make(map[int64]*enrollmentModels.Phase, len(phases))
	for _, phase := range phases {
		if phase == nil {
			continue
		}
		if period != nil && validatePhaseWithinTemplatePeriod(phase, period) != nil {
			continue
		}
		byID[phase.ID] = phase
	}
	return byID, nil
}

type offeringGradeCount struct {
	total   int
	byGrade map[int]int
}

// groupOfferingGradeCounts aggregates approved children per offering into
// distinct-child totals and per-grade buckets (0 = no derivable grade).
func groupOfferingGradeCounts(children []*enrollmentModels.ApprovedOfferingChild) map[int64]*offeringGradeCount {
	counts := make(map[int64]*offeringGradeCount)
	seen := make(map[int64]map[int64]bool)
	for _, child := range children {
		if child == nil || child.Link == nil {
			continue
		}
		offeringID := child.Link.CareOfferingID
		if seen[offeringID] == nil {
			seen[offeringID] = make(map[int64]bool)
		}
		if seen[offeringID][child.Link.RequestChildID] {
			continue
		}
		seen[offeringID][child.Link.RequestChildID] = true
		count := counts[offeringID]
		if count == nil {
			count = &offeringGradeCount{byGrade: map[int]int{}}
			counts[offeringID] = count
		}
		count.total++
		bucket := 0
		if grade := gradeLevelFromSchoolClass(child.SchoolClass); grade != nil {
			bucket = int(*grade)
		}
		count.byGrade[bucket]++
	}
	return counts
}
