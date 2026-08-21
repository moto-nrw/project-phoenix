package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
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
// template: the child's student, the draft describing period + weekdays, and
// the window the child actually holds the offering in. validUntil is
// exclusive, matching activities.StudentEnrollment. A child with
// non-contiguous offering links (left and later re-joined) yields one target
// per disjoint window — bridging the gap would plan the child on days it does
// not hold the offering.
type sourcedRosterTarget struct {
	studentID  int64
	draft      *careEnrollmentDraft
	validFrom  timezone.Date
	validUntil timezone.Date
}

// ResyncTemplateOfferingRoster implements the schedule layer's
// ResyncOfferingRoster hook (#2137): it reconciles the offering-sourced
// student_enrollments rows of one template with the union of the source
// offerings' currently approved enrollments, applying the template's
// Jahrgang filter.
//
//   - rows for children that still match keep their row when start and
//     weekday set line up with a wanted window; the end is then set exactly
//     to where the child's offering window reaches (extended OR shrunk)
//   - rows for children that no longer match — including rows whose validity
//     no longer fits any wanted window, e.g. after a source switch — are
//     deleted (not yet effective) or capped at EffectiveFrom (already
//     started); the reconcile diffs EVERY tagged row of the template, so no
//     cleanup scoped to the previous source set is needed
//   - several source offerings union per child: the FIRST offering (array
//     order) contributes its windows unchanged, later offerings contribute
//     only the weekday×date coverage the earlier ones do not already plan —
//     a child in "bis 16:00" (Mo–Fr) and "montags Musik" is planned once on
//     Monday, not twice. All offerings must share one enrollment phase.
//   - missing children are seeded via the same draft/persist shapes the
//     decision fan-out uses, so both write paths stay byte-compatible
//   - children fed by a legacy CareOffering.ActivityGroupID link pointing at
//     the same template are owned by that feed WITHIN the link's weekday and
//     validity footprint: rows fully inside it are never touched and the
//     source never seeds a duplicate there. Outside that footprint — other
//     weekdays, or dates before/after the legacy link's window — the child's
//     source contribution is seeded and reconciled normally: protection by
//     child identity alone would let a legacy Monday suppress a source
//     Friday and let stale source rows outlive a filter change (#2147 review)
//   - wanted windows stop at the template's own schedule envelope, so a
//     capped split predecessor is never planned past its segment end
//   - finally, the touched students' already-materialized future occurrences
//     are reconciled (the materializer never revisits existing instances)
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
	wanted := make(map[int64][]*sourcedRosterTarget)
	if len(in.OfferingIDs) > 0 {
		offerings, offeringPhase, dropped, err := s.loadOfferingSources(ctx, in.OfferingIDs, in.CalendarPeriodID, in.TolerateDriftedSources)
		if err != nil {
			return err
		}
		if len(dropped) > 0 {
			// The array carries no FK; an offering deleted without the detach
			// flow leaves a dangling id. Dropping it here (instead of failing)
			// keeps the template editable and lets the resync degrade to the
			// surviving sources, or to a full cleanup when none survive.
			s.Logger.Warn("offering roster resync: ignoring vanished source offerings",
				slog.Int64("template_id", in.TemplateID),
				slog.Any("care_offering_ids", dropped),
			)
		}
		if len(offerings) > 0 {
			phase = offeringPhase
			// The wanted windows are additionally bounded by the template's own
			// schedule envelope: the tenant-wide resync also visits capped split
			// predecessors, whose rows must never be extended past the segment end
			// (#2147 review). Open envelopes leave the windows untouched.
			schedules, err := s.ActivityScheduleRepo.FindByGroupID(ctx, in.TemplateID)
			if err != nil {
				return fmt.Errorf("offering roster resync: load template schedules: %w", err)
			}
			envelopeFrom, envelopeUntil := scheduleValidityBounds(schedules)
			wanted, err = s.unionWantedSourcedRosterTargets(ctx, offerings, phase, in, envelopeFrom, envelopeUntil)
			if err != nil {
				return err
			}
		}
	}

	scope := make(map[int64]bool, len(in.ScopeRequestChildIDs))
	for _, childID := range in.ScopeRequestChildIDs {
		scope[childID] = true
	}
	if len(scope) > 0 {
		for childID := range wanted {
			if !scope[childID] {
				delete(wanted, childID)
			}
		}
	}

	protectedChildren, err := s.legacyLinkedChildCoverage(ctx, in.TemplateID, in.EffectiveFrom)
	if err != nil {
		return err
	}
	// A weekday×date the legacy feed already plans must not get a second,
	// source-fed row — but only that overlapping contribution is dropped: a
	// child legacy-planned on Monday can still be source-fed on Friday, and a
	// legacy link starting mid-phase suppresses nothing before it begins
	// (#2147 review).
	for childID, coverage := range protectedChildren {
		remaining := subtractTargetCoverage(wanted[childID], coverage)
		if len(remaining) == 0 {
			delete(wanted, childID)
			continue
		}
		wanted[childID] = remaining
	}

	// Students whose rows this resync creates, caps, deletes, or resizes. Their
	// already-materialized future occurrences must be reconciled afterwards —
	// scoped to exactly these students so per-occurrence hand edits of
	// untouched children survive (#2147 review).
	touchedStudents := make(map[int64]bool)

	rows, err := s.StudentEnrollmentRepo.FindByGroupID(ctx, in.TemplateID)
	if err != nil {
		return fmt.Errorf("offering roster resync: load template enrollments: %w", err)
	}
	for _, row := range rows {
		if len(scope) > 0 && (row == nil || row.EnrollmentRequestChildID == nil || !scope[*row.EnrollmentRequestChildID]) {
			continue
		}
		changed, err := s.reconcileSourcedRosterRow(ctx, row, wanted, protectedChildren, in.EffectiveFrom)
		if err != nil {
			return err
		}
		if changed {
			touchedStudents[row.StudentID] = true
		}
	}

	childIDs := make([]int64, 0, len(wanted))
	for childID := range wanted {
		childIDs = append(childIDs, childID)
	}
	sort.Slice(childIDs, func(i, j int) bool { return childIDs[i] < childIDs[j] })
	for _, childID := range childIDs {
		for _, target := range wanted[childID] {
			row := studentEnrollmentFromCareDraft(childID, target.studentID, phase, target.draft, &target.validFrom)
			// The child's offering link can start after the template's effective
			// date and end before the phase does (dated switch into or out of the
			// offering). Planning them for the full phase window would place them
			// on days they do not hold the offering.
			validUntil := target.validUntil
			row.ValidUntil = &validUntil
			if err := row.Validate(); err != nil {
				return fmt.Errorf("offering roster resync: validate seeded enrollment: %w", err)
			}
			if err := s.StudentEnrollmentRepo.Create(ctx, row); err != nil {
				return fmt.Errorf("offering roster resync: create seeded enrollment: %w", err)
			}
			touchedStudents[target.studentID] = true
		}
	}
	// `rows` still holds the pre-write state (every write above goes by id):
	// the reconciler uses it to tell newly gained coverage from occurrences a
	// human removed a still-covered child from (#2147 review round 11).
	return s.reconcileSourcedInstanceRosters(ctx, in.TemplateID, touchedStudents, in.EffectiveFrom, rows)
}

// reconcileSourcedInstanceRosters propagates the resync's enrollment changes
// onto the template's already-materialized future occurrences (#2147 review).
// The materializer is insert-only and never revisits an existing instance, so
// without this pass schedule.instance_students — and every children/staffing
// count derived from it — stays stale until a manual re-plan.
func (s *decisionService) reconcileSourcedInstanceRosters(
	ctx context.Context,
	templateID int64,
	students map[int64]bool,
	effectiveFrom timezone.Date,
	priorEnrollments []*activities.StudentEnrollment,
) error {
	if len(students) == 0 {
		return nil
	}
	if s.InstanceRosters == nil {
		// Mock-only wirings (unit tests) may omit the reconciler; production
		// always injects it via the factory.
		s.Logger.Warn("offering roster resync: instance roster reconciler not configured; materialized occurrences may be stale",
			slog.Int64("template_id", templateID))
		return nil
	}
	studentIDs := make([]int64, 0, len(students))
	for studentID := range students {
		studentIDs = append(studentIDs, studentID)
	}
	sort.Slice(studentIDs, func(i, j int) bool { return studentIDs[i] < studentIDs[j] })
	if _, _, err := s.InstanceRosters.ReconcileSourcedTemplateRosters(ctx, templateID, studentIDs, effectiveFrom, priorEnrollments); err != nil {
		return fmt.Errorf("offering roster resync: reconcile materialized occurrences: %w", err)
	}
	return nil
}

// SourcedInstanceRosterReconciler propagates sourced-roster changes onto
// already-materialized future timetable occurrences (#2147 review).
// Implemented by services/schedule.RosterReconciler; injected here to avoid
// widening the decision service's repo surface. priorEnrollments is the
// template's enrollment state before the caller's writes (nil = coverage is
// being established from scratch); see the implementation's doc for how it
// preserves per-occurrence hand removals.
type SourcedInstanceRosterReconciler interface {
	ReconcileSourcedTemplateRosters(ctx context.Context, templateID int64, studentIDs []int64, from timezone.Date, priorEnrollments []*activities.StudentEnrollment) (int, int, error)
}

// scheduleValidityBounds returns the union recurrence envelope of a template's
// schedule rows. A side is nil (open) when any row is open on that side;
// otherwise the widest bound wins. Segment rows share one envelope per the
// split-series invariant — the union keeps drifted rows honest instead of
// trusting an arbitrary one.
func scheduleValidityBounds(schedules []*activities.Schedule) (validFrom, validUntil *timezone.Date) {
	fromOpen, untilOpen := false, false
	for _, sch := range schedules {
		if sch == nil {
			continue
		}
		switch {
		case sch.ValidFrom == nil:
			fromOpen, validFrom = true, nil
		case !fromOpen && (validFrom == nil || sch.ValidFrom.Before(*validFrom)):
			cloned := *sch.ValidFrom
			validFrom = &cloned
		}
		switch {
		case sch.ValidUntil == nil:
			untilOpen, validUntil = true, nil
		case !untilOpen && (validUntil == nil || sch.ValidUntil.After(*validUntil)):
			cloned := *sch.ValidUntil
			validUntil = &cloned
		}
	}
	return validFrom, validUntil
}

// ResyncOfferingSourcedTemplates re-reconciles EVERY offering-sourced
// template of the tenant against its offering's approved children. Grade
// transitions call it (apply AND revert) after rewriting school classes:
// promotions move children between Jahrgänge, so a template with a grade
// filter must lose the children that left its filter and gain the ones that
// entered it — and graduations turn children into alumni, whose rows are
// capped. History before effectiveFrom stays untouched.
//
// A template whose source has drifted invalid (offering deleted mid-flight,
// phase outside the period) is skipped with a warning, mirroring the decision
// fan-out: one broken template must not block a whole grade transition.
// The caller must hold the tenant recurrence lock.
func (s *decisionService) ResyncOfferingSourcedTemplates(ctx context.Context, effectiveFrom timezone.Date) error {
	if !s.hasEnrollmentMaterializationDependencies() {
		s.Logger.Warn("offering roster resync: enrollment repositories not configured; skipping sourced-template resync")
		return nil
	}
	templates, err := s.ActivityGroupRepo.FindTemplatesWithOfferingSource(ctx)
	if err != nil {
		return fmt.Errorf("offering roster resync: list sourced templates: %w", err)
	}
	return s.resyncSourcedTemplateList(ctx, templates, effectiveFrom, true)
}

// ResyncTemplatesSourcedFromOffering re-reconciles every template sourcing
// ONE offering. Care-offering updates and phase service-window updates call
// it (#2147 review): editing the offering's days or phase changes the wanted
// roster of every template fed by it, and without this resync the sourced
// rows and already-materialized occurrences would keep the pre-edit shape
// until an unrelated template save. A template whose source the pending edit
// makes incompatible with its planning period surfaces as
// ErrOfferingSourceInvalid — the caller must reject the edit then (#2147
// review round 7): swallowing it would commit a change whose sourced rows and
// materialized occurrences can never be resynced again. The caller must hold
// the tenant recurrence lock.
func (s *decisionService) ResyncTemplatesSourcedFromOffering(ctx context.Context, offeringID int64, effectiveFrom timezone.Date) error {
	if !s.hasEnrollmentMaterializationDependencies() {
		s.Logger.Warn("offering roster resync: enrollment repositories not configured; skipping sourced-template resync")
		return nil
	}
	templates, err := s.ActivityGroupRepo.FindTemplatesBySourceOffering(ctx, offeringID)
	if err != nil {
		return fmt.Errorf("offering roster resync: list templates sourcing offering %d: %w", offeringID, err)
	}
	return s.resyncSourcedTemplateList(ctx, templates, effectiveFrom, false)
}

// DetachTemplatesSourcedFromOffering removes ONE offering from the source set
// of every template sourcing it. Templates left with other sources are
// resynced against the remaining set — the diff-based reconcile retires the
// departing offering's contribution and keeps (or re-shapes) the rest. A
// template whose LAST source departs runs the cleanup-only resync: all
// offering-derived enrollment rows are deleted (not yet effective) or capped
// at effectiveFrom (already started), legacy-owned rows stay protected, and
// the affected students' already-materialized future occurrences are
// reconciled. Finally the template's source columns themselves are rewritten:
// since the jsonb id array carries no FK, there is no ON DELETE backstop —
// this call is the ONLY thing keeping the array free of deleted ids. The
// care-offering and phase delete flows call it BEFORE the row delete (#2147
// review round 11), while the provenance tags still exist. A remaining
// source that has itself drifted invalid keeps the template's remaining
// sources and skips only the roster resync instead of blocking the delete.
// The caller must hold the tenant recurrence lock.
func (s *decisionService) DetachTemplatesSourcedFromOffering(ctx context.Context, offeringID int64, effectiveFrom timezone.Date) error {
	if !s.hasEnrollmentMaterializationDependencies() {
		s.Logger.Warn("offering roster detach: enrollment repositories not configured; skipping sourced-template detach")
		return nil
	}
	templates, err := s.ActivityGroupRepo.FindTemplatesBySourceOffering(ctx, offeringID)
	if err != nil {
		return fmt.Errorf("offering roster detach: list templates sourcing offering %d: %w", offeringID, err)
	}
	for _, tmpl := range templates {
		if tmpl == nil {
			continue
		}
		remaining := removeOfferingID(tmpl.SourceCareOfferingIDs, offeringID)
		gradeLevels := tmpl.SourceGradeLevels
		schoolClasses := tmpl.SourceSchoolClasses
		if len(remaining) == 0 {
			gradeLevels = nil
			schoolClasses = nil
		}
		err := s.ResyncTemplateOfferingRoster(ctx, scheduleService.OfferingRosterResyncInput{
			TemplateID:       tmpl.ID,
			OfferingIDs:      remaining,
			GradeLevels:      gradeLevels,
			SchoolClasses:    schoolClasses,
			CalendarPeriodID: tmpl.CalendarPeriodID,
			EffectiveFrom:    effectiveFrom,
		})
		if err != nil {
			if len(remaining) > 0 && errors.Is(err, scheduleService.ErrOfferingSourceInvalid) {
				// A remaining source drifted invalid (e.g. it turned inactive or
				// its phase no longer fits the period pin). The delete must not
				// be blocked, and the remaining sources are valid data — only
				// their combination with the period pin failed — so they are
				// kept and the full union resync is skipped. But the DEPARTING
				// offering's rows cannot wait for a later repair: the delete
				// behind this detach cascades their request-child provenance
				// away, after which no resync can ever attribute them again.
				// Retire them now with a drift-tolerant resync scoped to the
				// departing offering's children (see retireDepartingSourcedRows).
				s.Logger.Warn("offering roster detach: remaining sources invalid; keeping sources, retiring only the departing offering's contribution",
					slog.Int64("template_id", tmpl.ID),
					slog.Int64("care_offering_id", offeringID),
					slog.String("error", err.Error()),
				)
				if err := s.retireDepartingSourcedRows(ctx, tmpl, offeringID, remaining, gradeLevels, schoolClasses, effectiveFrom); err != nil {
					return fmt.Errorf("offering roster detach: template %d: %w", tmpl.ID, err)
				}
			} else {
				return fmt.Errorf("offering roster detach: template %d: %w", tmpl.ID, err)
			}
		}
		if err := s.ActivityGroupRepo.UpdateTemplateOfferingSource(ctx, tmpl.ID, remaining, gradeLevels, schoolClasses); err != nil {
			return fmt.Errorf("offering roster detach: rewrite source columns of template %d: %w", tmpl.ID, err)
		}
	}
	return nil
}

// retireDepartingSourcedRows removes the departing offering's contribution
// from the template when the remaining sources are too drifted for the full
// union resync: the offering delete behind the detach is about to cascade
// the departing children's provenance away, so their rows must be retired
// NOW or they linger as unattributable orphans (#2147 round 11's pre-delete
// ordering). It runs the union resync against the REMAINING sources with
// drift-tolerant loading, scoped to the departing offering's children: a
// child only the departing offering fed loses their rows, and a shared
// child's rows are reshaped to exactly the remaining sources' link coverage
// — skipping shared children wholesale would leave the departing offering's
// exclusive weekday×date footprint (e.g. A's Di–Fr next to B's Mo) planned
// forever. Children of only the remaining sources are outside the scope and
// stay untouched.
func (s *decisionService) retireDepartingSourcedRows(
	ctx context.Context,
	tmpl *activities.Group,
	offeringID int64,
	remaining []int64,
	gradeLevels []int,
	schoolClasses []string,
	effectiveFrom timezone.Date,
) error {
	departing, err := s.RequestChildOfferingRepo.ListApprovedChildrenByCareOfferingIDs(ctx, []int64{offeringID}, effectiveFrom)
	if err != nil {
		return fmt.Errorf("list departing offering children: %w", err)
	}
	scope := make([]int64, 0, len(departing))
	seen := make(map[int64]bool, len(departing))
	for _, child := range departing {
		if child == nil || child.Link == nil || seen[child.Link.RequestChildID] {
			continue
		}
		seen[child.Link.RequestChildID] = true
		scope = append(scope, child.Link.RequestChildID)
	}
	if len(scope) == 0 {
		return nil
	}
	slices.Sort(scope)
	return s.ResyncTemplateOfferingRoster(ctx, scheduleService.OfferingRosterResyncInput{
		TemplateID:             tmpl.ID,
		OfferingIDs:            remaining,
		GradeLevels:            gradeLevels,
		SchoolClasses:          schoolClasses,
		CalendarPeriodID:       tmpl.CalendarPeriodID,
		EffectiveFrom:          effectiveFrom,
		ScopeRequestChildIDs:   scope,
		TolerateDriftedSources: true,
	})
}

// removeOfferingID returns ids without the given offering, preserving order.
func removeOfferingID(ids []int64, offeringID int64) []int64 {
	remaining := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id != offeringID {
			remaining = append(remaining, id)
		}
	}
	if len(remaining) == 0 {
		return nil
	}
	return remaining
}

// resyncSourcedTemplateList reconciles each template's sourced roster.
// skipInvalid controls what a drifted-invalid source does: the tenant-wide
// resync skips the template with a warning (one broken template must not
// block a whole grade transition), while the offering-scoped resync behind
// phase/offering edits propagates ErrOfferingSourceInvalid so the caller can
// reject the edit that would strand the template's rows (#2147 review).
func (s *decisionService) resyncSourcedTemplateList(ctx context.Context, templates []*activities.Group, effectiveFrom timezone.Date, skipInvalid bool) error {
	for _, tmpl := range templates {
		if tmpl == nil || len(tmpl.SourceCareOfferingIDs) == 0 {
			continue
		}
		err := s.ResyncTemplateOfferingRoster(ctx, scheduleService.OfferingRosterResyncInput{
			TemplateID:       tmpl.ID,
			OfferingIDs:      tmpl.SourceCareOfferingIDs,
			GradeLevels:      tmpl.SourceGradeLevels,
			SchoolClasses:    tmpl.SourceSchoolClasses,
			CalendarPeriodID: tmpl.CalendarPeriodID,
			EffectiveFrom:    effectiveFrom,
		})
		if err != nil {
			if errors.Is(err, scheduleService.ErrOfferingSourceInvalid) {
				if skipInvalid {
					s.logSkippedSourcedTemplate(tmpl.ID, tmpl.SourceCareOfferingIDs, "tenant-wide resync: source invalid", err)
					continue
				}
				return fmt.Errorf("offering roster resync: template %d (%q) would lose its offering source: %w", tmpl.ID, tmpl.Name, err)
			}
			return fmt.Errorf("offering roster resync: template %d: %w", tmpl.ID, err)
		}
	}
	return nil
}

// loadOfferingSources resolves and validates every source offering: each must
// be active, share ONE enrollment phase with the others, and that phase's
// service window must lie within the template's calendar period (when the
// template is period-pinned) — otherwise the seeded rows could never
// materialize and the editor would silently save a dead rule. Vanished
// offerings are dropped, not rejected (see loadValidatedOfferingSources).
func (s *decisionService) loadOfferingSources(
	ctx context.Context,
	offeringIDs []int64,
	calendarPeriodID *int64,
	tolerateDrift bool,
) ([]*enrollmentModels.CareOffering, *enrollmentModels.Phase, []int64, error) {
	return loadValidatedOfferingSources(ctx, s.CareOfferingRepo, s.PhaseRepo, s.CalendarPeriodRepo, offeringIDs, calendarPeriodID, tolerateDrift)
}

// loadValidatedOfferingSources runs the offering-source guard (#2137) per id
// and additionally rejects duplicate ids and mixed enrollment phases: the
// union's seeded rows carry ONE phase (window bounds, provenance via the
// phase's request children), and a child holding two offerings from different
// phases would surface under two different request-child tags — the reconcile
// could then plan the child twice without either row being wrong on its own.
//
// Per offering: it must exist in this tenant and be active — the editor's
// selector only offers active offerings, and a direct request must not wire a
// retired offering either — and the shared phase's service window must lie
// within the given calendar period (nil skips the period check). Every
// declaration path uses this loader — template create/update via the resync
// and its pre-check, the split via
// CareOfferingSeriesValidator.ValidateTemplateOfferingSource, and the
// combined-counts preview — so no path can persist a source rule the others
// would reject. Deactivating an offering that still feeds templates is
// consequently rejected like any other incompatible offering edit (the update
// flow's resync runs this guard).
//
// A VANISHED offering (row gone without the detach flow running — the jsonb
// array carries no FK, so nothing nulls it) is not a hard reject: the id is
// dropped and returned in droppedIDs. Rejecting would wedge the template —
// every later edit re-validates the stored array and would 400 forever, and
// the tenant-wide resync would skip the template until someone hand-edits the
// row. Dropping restores the old FK semantics: the template degrades
// gracefully to the surviving sources (or to a manual roster when none
// survive). Callers log droppedIDs where a logger is available.
//
// The list is capped (MaxOfferingSourcesPerTemplate) and the calendar period
// is loaded once, not per id — the ids arrive from unauthenticated-shaped
// query strings (combined counts) and from the stored array on every resync,
// so the per-id work must stay bounded.
//
// tolerateDrift skips the IsActive and phase-within-period rejections (the
// existence, duplicate, cap, and same-phase checks stay). Only the detach
// fallback sets it: retiring a deleted offering's contribution needs the
// remaining sources' link coverage as a fact, even while their drifted state
// blocks the regular resync.
func loadValidatedOfferingSources(
	ctx context.Context,
	offeringRepo enrollmentModels.CareOfferingRepository,
	phaseRepo enrollmentModels.PhaseRepository,
	periodRepo scheduleModels.CalendarPeriodRepository,
	offeringIDs []int64,
	calendarPeriodID *int64,
	tolerateDrift bool,
) (offerings []*enrollmentModels.CareOffering, phase *enrollmentModels.Phase, droppedIDs []int64, err error) {
	if len(offeringIDs) == 0 {
		return nil, nil, nil, fmt.Errorf("%w: at least one care offering is required", scheduleService.ErrOfferingSourceInvalid)
	}
	if len(offeringIDs) > scheduleService.MaxOfferingSourcesPerTemplate {
		return nil, nil, nil, fmt.Errorf(
			"%w: at most %d source offerings are supported (%d given)",
			scheduleService.ErrOfferingSourceInvalid, scheduleService.MaxOfferingSourcesPerTemplate, len(offeringIDs),
		)
	}
	var period *scheduleModels.CalendarPeriod
	if calendarPeriodID != nil {
		period, err = periodRepo.FindByID(ctx, *calendarPeriodID)
		if err != nil {
			if modelBase.IsNoRows(err) {
				return nil, nil, nil, fmt.Errorf("%w: calendar period %d not found", scheduleService.ErrOfferingSourceInvalid, *calendarPeriodID)
			}
			return nil, nil, nil, fmt.Errorf("offering roster resync: load calendar period: %w", err)
		}
	}
	offerings = make([]*enrollmentModels.CareOffering, 0, len(offeringIDs))
	seen := make(map[int64]bool, len(offeringIDs))
	for _, offeringID := range offeringIDs {
		if seen[offeringID] {
			return nil, nil, nil, fmt.Errorf("%w: care offering %d is listed twice", scheduleService.ErrOfferingSourceInvalid, offeringID)
		}
		seen[offeringID] = true
		offering, err := offeringRepo.FindByID(ctx, offeringID)
		if err != nil {
			if modelBase.IsNoRows(err) {
				droppedIDs = append(droppedIDs, offeringID)
				continue
			}
			return nil, nil, nil, fmt.Errorf("offering roster resync: load offering: %w", err)
		}
		if offering == nil {
			droppedIDs = append(droppedIDs, offeringID)
			continue
		}
		if !offering.IsActive && !tolerateDrift {
			return nil, nil, nil, fmt.Errorf("%w: care offering %d is inactive", scheduleService.ErrOfferingSourceInvalid, offeringID)
		}
		if phase != nil && offering.PhaseID != phase.ID {
			return nil, nil, nil, fmt.Errorf(
				"%w: all source offerings must belong to the same enrollment phase (offering %d belongs to %q)",
				scheduleService.ErrOfferingSourceInvalid, offeringID, offeringPhaseName(ctx, phaseRepo, offering.PhaseID),
			)
		}
		if phase == nil {
			// The first surviving offering establishes the shared phase; the
			// window check runs once since every later offering must match it.
			phase, err = phaseRepo.FindByID(ctx, offering.PhaseID)
			if err != nil {
				if modelBase.IsNoRows(err) {
					return nil, nil, nil, fmt.Errorf("%w: enrollment phase of care offering %d not found", scheduleService.ErrOfferingSourceInvalid, offeringID)
				}
				return nil, nil, nil, fmt.Errorf("offering roster resync: load phase: %w", err)
			}
			if period != nil && !tolerateDrift {
				if err := validatePhaseWithinTemplatePeriod(phase, period); err != nil {
					return nil, nil, nil, fmt.Errorf("%w: %s", scheduleService.ErrOfferingSourceInvalid, err.Error())
				}
			}
		}
		offerings = append(offerings, offering)
	}
	return offerings, phase, droppedIDs, nil
}

// offeringPhaseName resolves a phase name for the mixed-phase error message;
// error path only, so a failed lookup falls back to the id.
func offeringPhaseName(ctx context.Context, phaseRepo enrollmentModels.PhaseRepository, phaseID int64) string {
	phase, err := phaseRepo.FindByID(ctx, phaseID)
	if err != nil || phase == nil {
		return fmt.Sprintf("phase %d", phaseID)
	}
	return phase.Name
}

// unionWantedSourcedRosterTargets builds the union roster across the source
// offerings in array order: the first offering's targets are taken as-is,
// every later offering contributes only what the accumulated coverage does
// not already plan (subtractTargetCoverage — the same segment-splitting
// machinery the legacy-feed protection uses). The order dependence is
// deliberate and stable: the stored id array is the order, so repeated
// resyncs produce byte-identical rows and the row reuse in
// reconcileSourcedRosterRow keeps matching.
func (s *decisionService) unionWantedSourcedRosterTargets(
	ctx context.Context,
	offerings []*enrollmentModels.CareOffering,
	phase *enrollmentModels.Phase,
	in scheduleService.OfferingRosterResyncInput,
	envelopeFrom, envelopeUntil *timezone.Date,
) (map[int64][]*sourcedRosterTarget, error) {
	wanted := make(map[int64][]*sourcedRosterTarget)
	for _, offering := range offerings {
		perOffering, err := s.wantedSourcedRosterTargets(ctx, offering, phase, in, envelopeFrom, envelopeUntil)
		if err != nil {
			return nil, err
		}
		childIDs := make([]int64, 0, len(perOffering))
		for childID := range perOffering {
			childIDs = append(childIDs, childID)
		}
		sort.Slice(childIDs, func(i, j int) bool { return childIDs[i] < childIDs[j] })
		for _, childID := range childIDs {
			targets := perOffering[childID]
			existing := wanted[childID]
			if len(existing) > 0 {
				targets = subtractTargetCoverage(targets, coverageFromSourcedTargets(existing))
				if len(targets) == 0 {
					continue
				}
			}
			wanted[childID] = coalesceSourcedRosterTargets(append(existing, targets...))
		}
	}
	return wanted, nil
}

// coverageFromSourcedTargets converts a child's already-accepted union
// targets into the coverage shape the subtraction machinery consumes: one
// window per target, carrying the target's weekday set.
func coverageFromSourcedTargets(targets []*sourcedRosterTarget) *legacyChildCoverage {
	coverage := &legacyChildCoverage{windows: make([]legacyCoverageWindow, 0, len(targets))}
	for _, target := range targets {
		from := target.validFrom
		until := target.validUntil
		window := legacyCoverageWindow{from: &from, until: &until}
		if target.draft.allWeekdays || len(target.draft.selectedWeekday) == 0 {
			window.allWeekdays = true
		} else {
			window.weekdays = target.draft.selectedWeekday
		}
		coverage.windows = append(coverage.windows, window)
	}
	return coverage
}

// wantedSourcedRosterTargets builds the desired roster of the sourced
// template: every approved, grade-matching child of the offering with the
// weekday set their enrollment selects, bounded by the window their offering
// link is actually valid in. Each child maps to one target per disjoint
// link window (see coalesceSourcedRosterTargets).
func (s *decisionService) wantedSourcedRosterTargets(
	ctx context.Context,
	offering *enrollmentModels.CareOffering,
	phase *enrollmentModels.Phase,
	in scheduleService.OfferingRosterResyncInput,
	envelopeFrom, envelopeUntil *timezone.Date,
) (map[int64][]*sourcedRosterTarget, error) {
	children, err := s.RequestChildOfferingRepo.ListApprovedChildrenByCareOfferingIDs(ctx, []int64{offering.ID}, in.EffectiveFrom)
	if err != nil {
		return nil, fmt.Errorf("offering roster resync: list approved children: %w", err)
	}
	wanted := make(map[int64][]*sourcedRosterTarget, len(children))
	for _, child := range children {
		grade := gradeLevelFromSchoolClass(child.SchoolClass)
		if !gradeFilterMatches(in.GradeLevels, grade) {
			continue
		}
		// The two filters are mutually exclusive by validation, so at most one
		// of the two guards is ever active (#2482).
		if !activities.SourceClassFilterMatches(in.SchoolClasses, child.SchoolClass) {
			continue
		}
		validFrom, validUntil, ok := sourcedRosterWindow(child.Link, phase, in.EffectiveFrom, envelopeFrom, envelopeUntil)
		if !ok {
			// A link that only covers days before the template takes effect
			// (or outside the phase) has nothing left to plan.
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
		wanted[child.Link.RequestChildID] = append(wanted[child.Link.RequestChildID], &sourcedRosterTarget{
			studentID:  child.StudentID,
			draft:      draft,
			validFrom:  validFrom,
			validUntil: validUntil,
		})
	}
	for childID := range wanted {
		wanted[childID] = coalesceSourcedRosterTargets(wanted[childID])
	}
	return wanted, nil
}

// coalesceSourcedRosterTargets reduces a child's per-link windows to the
// minimal row set. Windows that overlap or touch AND select the same weekday
// set collapse into one row — the common dated-switch shape of consecutive
// links with unchanged days. Everything else stays a separate row: bridging a
// gap between two links would plan the child in a window it left the offering
// for, and unifying different weekday sets would plan days only one of the
// links selects.
func coalesceSourcedRosterTargets(targets []*sourcedRosterTarget) []*sourcedRosterTarget {
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].validFrom != targets[j].validFrom {
			return targets[i].validFrom.Before(targets[j].validFrom)
		}
		return targets[i].validUntil.Before(targets[j].validUntil)
	})
	merged := make([]*sourcedRosterTarget, 0, len(targets))
	for _, target := range targets {
		if len(merged) > 0 {
			last := merged[len(merged)-1]
			if !target.validFrom.After(last.validUntil) && sameSourcedDraftDays(last.draft, target.draft) {
				if target.validUntil.After(last.validUntil) {
					last.validUntil = target.validUntil
				}
				continue
			}
		}
		merged = append(merged, target)
	}
	return merged
}

// sameSourcedDraftDays reports whether two drafts select the same weekday set.
func sameSourcedDraftDays(left, right *careEnrollmentDraft) bool {
	if left.allWeekdays != right.allWeekdays {
		return false
	}
	if left.allWeekdays {
		return true
	}
	if len(left.selectedWeekday) != len(right.selectedWeekday) {
		return false
	}
	for weekday := range left.selectedWeekday {
		if !right.selectedWeekday[weekday] {
			return false
		}
	}
	return true
}

// sourcedRosterWindow intersects four windows into the validity of one
// seeded roster row: the phase's service window, the template's effective
// date (history is never rewritten), the template's own schedule envelope
// (a capped split segment must not plan past its end nor before its start,
// #2147 review), and the child's offering link. The link bound is what keeps
// a dated switch honest — a selection starting in September must not plan the
// child from the template's save date onward, and one ending in September
// must not outlive it. ok is false when the intersection is empty.
func sourcedRosterWindow(
	link *enrollmentModels.RequestChildOffering,
	phase *enrollmentModels.Phase,
	effectiveFrom timezone.Date,
	envelopeFrom, envelopeUntil *timezone.Date,
) (validFrom, validUntil timezone.Date, ok bool) {
	validFrom = phase.ServiceStartDate
	if effectiveFrom.After(validFrom) {
		validFrom = effectiveFrom
	}
	if envelopeFrom != nil && envelopeFrom.After(validFrom) {
		validFrom = *envelopeFrom
	}
	validUntil = phase.ServiceEndDate.AddDays(1)
	if envelopeUntil != nil && envelopeUntil.Before(validUntil) {
		validUntil = *envelopeUntil
	}
	if link != nil {
		if link.ValidFrom != nil && link.ValidFrom.After(validFrom) {
			validFrom = *link.ValidFrom
		}
		if link.ValidUntil != nil && link.ValidUntil.Before(validUntil) {
			validUntil = *link.ValidUntil
		}
	}
	if !validFrom.Before(validUntil) {
		return timezone.Date{}, timezone.Date{}, false
	}
	return validFrom, validUntil, true
}

// reconcileSourcedRosterRow applies keep/cap/delete to one existing row.
// Manual rows (no request-child tag), history, and legacy-owned rows (see
// legacyChildCoverage.coversRow) are left untouched.
//
// A row is only reused for a target whose window it can actually serve: the
// weekday/period draft must match AND the validity start must line up (either
// exactly, or the row already started before the rewrite boundary and the
// target begins at that boundary). Matching on the draft alone would let a
// row survive a source switch with a start or end outside the new offering
// link's validity. The end is then set EXACTLY — shrinking is as necessary as
// extending, because the child's offering window may have contracted.
//
// The returned bool reports whether the row was actually written (capped,
// deleted, or resized) — the callers feed those students into the
// materialized-instance reconcile (#2147 review).
func (s *decisionService) reconcileSourcedRosterRow(
	ctx context.Context,
	row *activities.StudentEnrollment,
	wanted map[int64][]*sourcedRosterTarget,
	protectedChildren map[int64]*legacyChildCoverage,
	effectiveFrom timezone.Date,
) (bool, error) {
	if row == nil || row.EnrollmentRequestChildID == nil {
		return false, nil
	}
	if row.ValidUntil != nil && !row.ValidUntil.After(effectiveFrom) {
		return false, nil // history stays
	}
	childID := *row.EnrollmentRequestChildID
	if coverage := protectedChildren[childID]; coverage.coversRow(row) {
		// The row plans nothing the legacy CareOffering.ActivityGroupID feed
		// would not plan itself: its lifecycle belongs to THAT offering's
		// flows and is never rewritten here. A row of the same child reaching
		// outside the legacy footprint — other weekdays or dates the legacy
		// links do not cover — falls through: it carries a source
		// contribution the resync must reconcile (#2147 review).
		return false, nil
	}
	if idx := indexOfServableTarget(wanted[childID], row, effectiveFrom); idx >= 0 {
		target := wanted[childID][idx]
		wanted[childID] = append(wanted[childID][:idx], wanted[childID][idx+1:]...)
		if len(wanted[childID]) == 0 {
			delete(wanted, childID)
		}
		if row.ValidUntil == nil || *row.ValidUntil != target.validUntil {
			if err := s.StudentEnrollmentRepo.SetValidUntilByID(ctx, row.ID, target.validUntil); err != nil {
				return false, fmt.Errorf("offering roster resync: adjust retained enrollment: %w", err)
			}
			return true, nil
		}
		return false, nil
	}
	if !row.ValidFrom.Before(effectiveFrom) {
		if err := s.StudentEnrollmentRepo.Delete(ctx, row.ID); err != nil {
			return false, fmt.Errorf("offering roster resync: delete enrollment: %w", err)
		}
		return true, nil
	}
	if err := s.StudentEnrollmentRepo.SetValidUntilByID(ctx, row.ID, effectiveFrom); err != nil {
		return false, fmt.Errorf("offering roster resync: cap enrollment: %w", err)
	}
	return true, nil
}

// indexOfServableTarget returns the first target the row can serve without
// misrepresenting any day: draft (weekdays + period) equal and validity start
// aligned. A row whose ValidFrom lies before effectiveFrom carries immutable
// history, so it may serve a target starting exactly at the rewrite boundary;
// any other start mismatch means cap-and-reseed.
func indexOfServableTarget(
	targets []*sourcedRosterTarget,
	row *activities.StudentEnrollment,
	effectiveFrom timezone.Date,
) int {
	for i, target := range targets {
		if !careDraftMatchesEnrollment(target.draft, row) {
			continue
		}
		if row.ValidFrom == target.validFrom {
			return i
		}
		if row.ValidFrom.Before(effectiveFrom) && target.validFrom == effectiveFrom {
			return i
		}
	}
	return -1
}

// legacyChildCoverage is what the legacy CareOffering.ActivityGroupID feed
// (#1651) plans for one child on the template being resynced, as one window
// per approved link. The resync uses it to tell legacy-owned rows (protected)
// from source-owned contributions of the SAME child (reconciled normally).
// Both the weekday set AND each link's validity window matter (#2147 review):
// weekday-only protection would let a legacy Monday suppress a source Friday,
// and window-less protection would let a future or partial legacy link guard
// dates it never plans.
type legacyChildCoverage struct {
	windows []legacyCoverageWindow
}

// legacyCoverageWindow is one legacy link's contribution: the weekdays it
// selects during [from, until). Open sides are nil; allWeekdays marks a link
// whose day selection resolves to every weekday — or could not be resolved,
// which protects conservatively.
type legacyCoverageWindow struct {
	from        *timezone.Date
	until       *timezone.Date // exclusive
	allWeekdays bool
	weekdays    map[int]bool
}

func (w *legacyCoverageWindow) coversWeekday(weekday int) bool {
	return w.allWeekdays || w.weekdays[weekday]
}

func (w *legacyCoverageWindow) activeOn(date timezone.Date) bool {
	if w.from != nil && w.from.After(date) {
		return false
	}
	return w.until == nil || w.until.After(date)
}

func allISOWeekdays() []int { return []int{1, 2, 3, 4, 5, 6, 7} }

// coversRow reports whether a roster row's weekday selection AND validity lie
// fully inside what the legacy feed plans. Such a row plans nothing the
// legacy feed would not plan itself, so the resync leaves it alone; a row
// reaching outside the legacy windows carries a source contribution and is
// reconciled normally (#2147 review).
func (c *legacyChildCoverage) coversRow(row *activities.StudentEnrollment) bool {
	if c == nil || len(c.windows) == 0 {
		return false
	}
	weekdays := row.SelectedWeekdays
	if len(weekdays) == 0 {
		// An empty selection means every weekday.
		weekdays = allISOWeekdays()
	}
	for _, weekday := range weekdays {
		if !c.coversSpan(weekday, row.ValidFrom, row.ValidUntil) {
			return false
		}
	}
	return true
}

// coversSpan reports whether the union of the legacy windows selecting the
// weekday covers all of [from, until). An open row end (until == nil) is only
// covered by a chain reaching an open-ended window. The sweep extends a
// cursor through overlapping windows; it terminates because every extension
// strictly advances the cursor to one of finitely many window ends.
func (c *legacyChildCoverage) coversSpan(weekday int, from timezone.Date, until *timezone.Date) bool {
	cursor := from
	for progressed := true; progressed; {
		progressed = false
		for i := range c.windows {
			w := &c.windows[i]
			if !w.coversWeekday(weekday) || (w.from != nil && w.from.After(cursor)) {
				continue
			}
			if w.until == nil {
				return true
			}
			if until != nil && !w.until.Before(*until) {
				return true
			}
			if w.until.After(cursor) {
				cursor = *w.until
				progressed = true
			}
		}
	}
	return false
}

// subtractTargetCoverage removes an existing weekday×date coverage from a
// child's wanted source targets, splitting each target at the coverage
// windows' boundaries. Segments outside every window keep the original draft;
// segments with a partial weekday overlap keep only the weekdays the coverage
// does not plan there; fully covered segments drop. Touching segments that
// end up with the same weekday set coalesce back into one row. Two coverages
// consume it: the legacy CareOffering.ActivityGroupID feed's footprint, and —
// with several source offerings — the union coverage the earlier offerings
// already contributed (multi-source follow-up to #2137).
func subtractTargetCoverage(targets []*sourcedRosterTarget, coverage *legacyChildCoverage) []*sourcedRosterTarget {
	if coverage == nil || len(coverage.windows) == 0 {
		return targets
	}
	remaining := make([]*sourcedRosterTarget, 0, len(targets))
	for _, target := range targets {
		remaining = append(remaining, subtractTargetCoverageFromTarget(target, coverage)...)
	}
	return coalesceSourcedRosterTargets(remaining)
}

func subtractTargetCoverageFromTarget(target *sourcedRosterTarget, coverage *legacyChildCoverage) []*sourcedRosterTarget {
	bounds := legacySegmentBoundaries(target.validFrom, target.validUntil, coverage)
	out := make([]*sourcedRosterTarget, 0, len(bounds)-1)
	for i := 0; i+1 < len(bounds); i++ {
		segFrom, segUntil := bounds[i], bounds[i+1]
		draft, ok := draftMinusLegacyWeekdays(target.draft, coverage, segFrom)
		if !ok {
			continue
		}
		out = append(out, &sourcedRosterTarget{
			studentID:  target.studentID,
			draft:      draft,
			validFrom:  segFrom,
			validUntil: segUntil,
		})
	}
	return out
}

// legacySegmentBoundaries returns the sorted cut points of [from, until):
// both ends plus every legacy window edge falling strictly inside. Between
// two consecutive points the set of active legacy windows is constant.
func legacySegmentBoundaries(from, until timezone.Date, coverage *legacyChildCoverage) []timezone.Date {
	set := map[timezone.Date]bool{from: true, until: true}
	for _, w := range coverage.windows {
		for _, edge := range []*timezone.Date{w.from, w.until} {
			if edge != nil && edge.After(from) && edge.Before(until) {
				set[*edge] = true
			}
		}
	}
	bounds := make([]timezone.Date, 0, len(set))
	for bound := range set {
		bounds = append(bounds, bound)
	}
	sort.Slice(bounds, func(i, j int) bool { return bounds[i].Before(bounds[j]) })
	return bounds
}

// draftMinusLegacyWeekdays reduces a draft by the weekdays the legacy windows
// active on `at` plan there. ok is false when nothing remains. A draft none
// of whose weekdays overlap is returned untouched — including its all-weekday
// shape — so unaffected rows keep matching byte-for-byte.
func draftMinusLegacyWeekdays(draft *careEnrollmentDraft, coverage *legacyChildCoverage, at timezone.Date) (*careEnrollmentDraft, bool) {
	legacyAll := false
	legacyDays := map[int]bool{}
	for i := range coverage.windows {
		w := &coverage.windows[i]
		if !w.activeOn(at) {
			continue
		}
		if w.allWeekdays {
			legacyAll = true
			break
		}
		for weekday := range w.weekdays {
			legacyDays[weekday] = true
		}
	}
	if legacyAll {
		return nil, false
	}
	if len(legacyDays) == 0 {
		return draft, true
	}
	draftAllDays := draft.allWeekdays || len(draft.selectedWeekday) == 0
	remaining := make(map[int]bool)
	overlapped := false
	for _, weekday := range allISOWeekdays() {
		if !draftAllDays && !draft.selectedWeekday[weekday] {
			continue
		}
		if legacyDays[weekday] {
			overlapped = true
			continue
		}
		remaining[weekday] = true
	}
	if !overlapped {
		return draft, true
	}
	if len(remaining) == 0 {
		return nil, false
	}
	return &careEnrollmentDraft{
		activityGroupID:  draft.activityGroupID,
		calendarPeriodID: draft.calendarPeriodID,
		selectedWeekday:  remaining,
	}, true
}

// legacyLinkedChildCoverage returns, per request child, the windows the
// legacy CareOffering.ActivityGroupID feed (#1651) plans on this template.
// The resync must never rewrite rows inside that footprint: their lifecycle
// belongs to the decision/adjustment flows of THAT offering.
//
// The lookup follows the template's whole split lineage, not just the direct
// link: the legacy fan-out writes rows on EVERY overlapping series segment
// (addLegacyLinkedGroupDrafts), so a link pointing at a predecessor still
// plans children on the successor being resynced. Matching only the direct
// link would let a resync treat those rows as source-owned and delete them
// (#2147 review).
func (s *decisionService) legacyLinkedChildCoverage(
	ctx context.Context,
	templateID int64,
	onOrAfter timezone.Date,
) (map[int64]*legacyChildCoverage, error) {
	series, err := s.ActivityGroupRepo.FindTemplateSeries(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("offering roster resync: load template series for legacy coverage: %w", err)
	}
	offerings, err := s.CareOfferingRepo.ListByActivityGroupIDs(ctx, careOfferingSeriesGroupIDs(templateID, series))
	if err != nil {
		return nil, fmt.Errorf("offering roster resync: list legacy-linked offerings: %w", err)
	}
	if len(offerings) == 0 {
		return map[int64]*legacyChildCoverage{}, nil
	}
	offeringByID := make(map[int64]*enrollmentModels.CareOffering, len(offerings))
	offeringIDs := make([]int64, 0, len(offerings))
	for _, offering := range offerings {
		if offering != nil {
			offeringByID[offering.ID] = offering
			offeringIDs = append(offeringIDs, offering.ID)
		}
	}
	children, err := s.RequestChildOfferingRepo.ListApprovedChildrenByCareOfferingIDs(ctx, offeringIDs, onOrAfter)
	if err != nil {
		return nil, fmt.Errorf("offering roster resync: list legacy-linked children: %w", err)
	}
	phases := make(map[int64]*enrollmentModels.Phase)
	protected := make(map[int64]*legacyChildCoverage, len(children))
	for _, child := range children {
		coverage := protected[child.Link.RequestChildID]
		if coverage == nil {
			coverage = &legacyChildCoverage{}
			protected[child.Link.RequestChildID] = coverage
		}
		window, err := s.legacyCoverageWindowForLink(ctx, phases, offeringByID[child.Link.CareOfferingID], child.Link, templateID)
		if err != nil {
			return nil, err
		}
		if window != nil {
			coverage.windows = append(coverage.windows, *window)
		}
	}
	return protected, nil
}

// legacyCoverageWindowForLink resolves one legacy link into its coverage
// window: the link's validity clipped to its offering's phase service window
// (the rows the legacy feed writes never reach outside either), carrying the
// weekday set the link selects. Unresolvable offerings, phases, or day
// selections protect conservatively — an open, all-weekday window — instead
// of failing the whole resync; ambiguity must never remove a child the
// legacy link still plans. nil without error means the link plans nothing.
func (s *decisionService) legacyCoverageWindowForLink(
	ctx context.Context,
	phases map[int64]*enrollmentModels.Phase,
	offering *enrollmentModels.CareOffering,
	link *enrollmentModels.RequestChildOffering,
	templateID int64,
) (*legacyCoverageWindow, error) {
	if offering == nil {
		return &legacyCoverageWindow{allWeekdays: true}, nil
	}
	phase, err := s.legacyCoveragePhase(ctx, phases, offering)
	if err != nil {
		return nil, err
	}
	if phase == nil {
		s.Logger.Warn("offering roster resync: cannot resolve legacy-linked phase; protecting the full window",
			slog.Int64("template_id", templateID),
			slog.Int64("care_offering_id", offering.ID),
			slog.Int64("request_child_id", link.RequestChildID),
		)
		return &legacyCoverageWindow{allWeekdays: true}, nil
	}
	window := &legacyCoverageWindow{
		from:  cloneOptionalDraftDate(link.ValidFrom),
		until: cloneOptionalDraftDate(link.ValidUntil),
	}
	serviceStart := phase.ServiceStartDate
	if window.from == nil || serviceStart.After(*window.from) {
		window.from = &serviceStart
	}
	serviceEnd := phase.ServiceEndDate.AddDays(1)
	if window.until == nil || serviceEnd.Before(*window.until) {
		window.until = &serviceEnd
	}
	if !window.from.Before(*window.until) {
		return nil, nil
	}
	days, err := effectiveOfferingDaysForEnrollment(offering, link)
	if err != nil || len(days) == 0 {
		// mergeCareEnrollmentDraft treats an empty day list as every weekday;
		// an unresolvable selection is protected the same, conservative way
		// instead of failing the whole resync.
		if err != nil {
			s.Logger.Warn("offering roster resync: cannot resolve legacy-linked days; protecting all weekdays",
				slog.Int64("template_id", templateID),
				slog.Int64("care_offering_id", offering.ID),
				slog.Int64("request_child_id", link.RequestChildID),
				slog.String("error", err.Error()),
			)
		}
		window.allWeekdays = true
		return window, nil
	}
	window.weekdays = make(map[int]bool, len(days))
	for _, day := range days {
		if weekday, ok := enrollmentDayToISOWeekday(day); ok {
			window.weekdays[weekday] = true
		}
	}
	if len(window.weekdays) == 0 {
		window.allWeekdays = true
	}
	return window, nil
}

// legacyCoveragePhase loads (and memoizes) the phase of a legacy-linked
// offering. A missing phase row yields nil so the caller can protect
// conservatively instead of failing the resync.
func (s *decisionService) legacyCoveragePhase(
	ctx context.Context,
	phases map[int64]*enrollmentModels.Phase,
	offering *enrollmentModels.CareOffering,
) (*enrollmentModels.Phase, error) {
	if phase, ok := phases[offering.PhaseID]; ok {
		return phase, nil
	}
	phase, err := s.PhaseRepo.FindByID(ctx, offering.PhaseID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			phases[offering.PhaseID] = nil
			return nil, nil
		}
		return nil, fmt.Errorf("offering roster resync: load legacy-linked phase: %w", err)
	}
	phases[offering.PhaseID] = phase
	return phase, nil
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
func (s *decisionService) logSkippedSourcedTemplate(templateID int64, offeringIDs []int64, reason string, err error) {
	attrs := []any{
		slog.Int64("template_id", templateID),
		slog.Any("care_offering_ids", offeringIDs),
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
	// SchoolClasses is the template's Klassenfilter (#2482); mutually
	// exclusive with GradeLevels.
	SchoolClasses []string `json:"school_classes,omitempty"`
}

// OfferingSourceOption is one selectable Betreuungsangebot in the Regeltermin
// editor (#2137), with per-grade counts of approved children (live filter
// preview) and the templates already sourcing it (overlap Hinweis).
type OfferingSourceOption struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	PhaseID   int64  `json:"phase_id"`
	PhaseName string `json:"phase_name"`
	// PhaseServiceStart is the phase's service_start_date. Sourced
	// enrollments never start earlier, so materialised occurrences before this
	// day stay empty of offering-fed children (OGS am Berg, 2026-08).
	PhaseServiceStart timezone.Date
	// TotalCount is the number of distinct approved children currently or
	// prospectively enrolled in the offering.
	TotalCount int `json:"total_count"`
	// GradeCounts maps Jahrgang → approved children; key 0 collects children
	// whose school class carries no derivable grade number.
	GradeCounts            map[int]int               `json:"grade_counts"`
	SourcedTemplates       []OfferingSourcedTemplate `json:"sourced_templates"`
	LegacyLinkedTemplateID *int64                    `json:"legacy_linked_template_id,omitempty"`
}

// OfferingSourceCombinedCounts is the deduplicated child count across a
// SELECTION of offerings (multi-source follow-up to #2137): a child enrolled
// in two of the selected offerings counts once, so the editor's Jahrgang
// preview shows the exact roster size the union resync would seed.
type OfferingSourceCombinedCounts struct {
	// TotalCount is the number of DISTINCT approved children across the
	// selected offerings.
	TotalCount int `json:"total_count"`
	// GradeCounts maps Jahrgang → distinct approved children; key 0 collects
	// children whose school class carries no derivable grade number.
	GradeCounts map[int]int `json:"grade_counts"`
	// Students is the deduplicated child list behind the counts (#2482), each
	// with the school class the filters match on. The editor derives the
	// Klassen options and, when an admin converts a manually curated
	// Regeltermin into a sourced one, the named diff against the previous
	// roster — "keine stille Übernahme falscher Kinderlisten". Names are NOT
	// carried here: the editor already holds the tenant's student list for its
	// roster picker and resolves them locally, so this payload stays free of
	// child identities beyond the ids it may already show.
	Students []OfferingSourceStudent `json:"students"`
}

// OfferingSourceStudent is one deduplicated child of a source selection with
// the school class the Jahrgang/Klassen filters are evaluated against.
type OfferingSourceStudent struct {
	StudentID   int64  `json:"student_id"`
	SchoolClass string `json:"school_class,omitempty"`
}

const (
	EmptyOfferingRosterBeforeServiceStart = "before_offering_start"
	EmptyOfferingRosterSourceEmpty        = "offering_source_empty"
)

// EmptyOfferingRosterExplanation describes why an occurrence backed by care
// offerings currently has no concrete children. After the service start the
// explanation deliberately stays neutral: an empty occurrence can result
// from weekday/filter rules, changed enrollments, or a stale materialization.
type EmptyOfferingRosterExplanation struct {
	Kind             string
	PhaseName        string
	ServiceStartDate timezone.Date
}

// ExplainEmptyOfferingRoster classifies an empty sourced occurrence from the
// authoritative offering-phase metadata. The API layer only maps this domain
// result onto its wire representation.
func ExplainEmptyOfferingRoster(
	options []OfferingSourceOption,
	selectedOfferingIDs []int64,
	date timezone.Date,
) *EmptyOfferingRosterExplanation {
	if len(selectedOfferingIDs) == 0 {
		return nil
	}
	selected := make(map[int64]struct{}, len(selectedOfferingIDs))
	for _, offeringID := range selectedOfferingIDs {
		selected[offeringID] = struct{}{}
	}
	explanation := &EmptyOfferingRosterExplanation{Kind: EmptyOfferingRosterSourceEmpty}
	for _, option := range options {
		if _, ok := selected[option.ID]; !ok {
			continue
		}
		explanation.PhaseName = option.PhaseName
		explanation.ServiceStartDate = option.PhaseServiceStart
		if !option.PhaseServiceStart.IsZero() && date.Before(option.PhaseServiceStart) {
			explanation.Kind = EmptyOfferingRosterBeforeServiceStart
		}
		return explanation
	}
	return explanation
}

// OfferingSourceOptionLister is the api/timetable-facing view of the editor
// support endpoint.
type OfferingSourceOptionLister interface {
	ListOfferingSourceOptions(ctx context.Context, calendarPeriodID *int64) ([]OfferingSourceOption, error)
	CombinedOfferingSourceCounts(ctx context.Context, offeringIDs []int64, calendarPeriodID *int64) (*OfferingSourceCombinedCounts, error)
}

// CombinedOfferingSourceCounts validates the selection exactly like a save
// would (existence, active, one shared phase, phase-within-period) and then
// counts distinct approved children across the offerings. Dedup key is the
// request child — within one phase a child holds one request-child identity,
// which is also the provenance tag the union resync seeds rows under.
func (s *decisionService) CombinedOfferingSourceCounts(ctx context.Context, offeringIDs []int64, calendarPeriodID *int64) (*OfferingSourceCombinedCounts, error) {
	if s.CareOfferingRepo == nil || s.PhaseRepo == nil || s.RequestChildOfferingRepo == nil {
		return nil, fmt.Errorf("offering source counts: repositories are not configured")
	}
	offerings, _, _, err := loadValidatedOfferingSources(ctx, s.CareOfferingRepo, s.PhaseRepo, s.CalendarPeriodRepo, offeringIDs, calendarPeriodID, false)
	if err != nil {
		return nil, err
	}
	// Vanished ids were dropped by the loader; count only the survivors, the
	// same set a save's resync would union. None surviving means zero counts.
	countedIDs := make([]int64, 0, len(offerings))
	for _, offering := range offerings {
		countedIDs = append(countedIDs, offering.ID)
	}
	if len(countedIDs) == 0 {
		return &OfferingSourceCombinedCounts{GradeCounts: map[int]int{}, Students: []OfferingSourceStudent{}}, nil
	}
	// Same boundary rule as ListOfferingSourceOptions: mirror what a resync
	// for the selected period would seed.
	countedFrom := timezone.TodayDate()
	if calendarPeriodID != nil {
		period, err := s.CalendarPeriodRepo.FindByID(ctx, *calendarPeriodID)
		if err != nil {
			if modelBase.IsNoRows(err) {
				return nil, fmt.Errorf("%w: calendar period %d not found", scheduleService.ErrOfferingSourceInvalid, *calendarPeriodID)
			}
			return nil, fmt.Errorf("offering source counts: load calendar period: %w", err)
		}
		if period.StartDate.After(countedFrom) {
			countedFrom = period.StartDate
		}
	}
	children, err := s.RequestChildOfferingRepo.ListApprovedChildrenByCareOfferingIDs(ctx, countedIDs, countedFrom)
	if err != nil {
		return nil, fmt.Errorf("offering source counts: list approved children: %w", err)
	}
	counts := &OfferingSourceCombinedCounts{
		GradeCounts: map[int]int{},
		Students:    make([]OfferingSourceStudent, 0, len(children)),
	}
	seen := make(map[int64]bool, len(children))
	for _, child := range children {
		if child == nil || child.Link == nil || seen[child.Link.RequestChildID] {
			continue
		}
		seen[child.Link.RequestChildID] = true
		counts.TotalCount++
		bucket := 0
		if grade := gradeLevelFromSchoolClass(child.SchoolClass); grade != nil {
			bucket = int(*grade)
		}
		counts.GradeCounts[bucket]++
		counts.Students = append(counts.Students, OfferingSourceStudent{
			StudentID:   child.StudentID,
			SchoolClass: child.SchoolClass,
		})
	}
	return counts, nil
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
	phases, period, err := s.offeringSourcePhases(ctx, calendarPeriodID)
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
	// The counts must mirror what a resync for the selected period would seed:
	// its effective date never lies before the period starts, so for a future
	// period a link that ends before the period begins contributes nothing.
	// For a running (or past) period today stays the boundary — links that
	// already ended are no longer plannable either way.
	countedFrom := timezone.TodayDate()
	if period != nil && period.StartDate.After(countedFrom) {
		countedFrom = period.StartDate
	}
	children, err := s.RequestChildOfferingRepo.ListApprovedChildrenByCareOfferingIDs(ctx, offeringIDs, countedFrom)
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
			option.PhaseServiceStart = phase.ServiceStartDate
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
				ID:            tmpl.ID,
				Name:          tmpl.Name,
				GradeLevels:   tmpl.SourceGradeLevels,
				SchoolClasses: tmpl.SourceSchoolClasses,
			})
		}
		options = append(options, option)
	}
	return options, nil
}

// offeringSourcePhases returns the tenant's phases keyed by id, restricted to
// those fitting the calendar period when one is given. The loaded period is
// returned alongside so the caller can scope its child counts to it.
func (s *decisionService) offeringSourcePhases(ctx context.Context, calendarPeriodID *int64) (map[int64]*enrollmentModels.Phase, *scheduleModels.CalendarPeriod, error) {
	phases, err := s.PhaseRepo.ListByTenant(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("offering source options: list phases: %w", err)
	}
	var period *scheduleModels.CalendarPeriod
	if calendarPeriodID != nil {
		if s.CalendarPeriodRepo == nil {
			return nil, nil, fmt.Errorf("offering source options: calendar period repository is not configured")
		}
		period, err = s.CalendarPeriodRepo.FindByID(ctx, *calendarPeriodID)
		if err != nil {
			if modelBase.IsNoRows(err) {
				return nil, nil, fmt.Errorf("%w: calendar period %d not found", scheduleService.ErrOfferingSourceInvalid, *calendarPeriodID)
			}
			return nil, nil, fmt.Errorf("offering source options: load calendar period: %w", err)
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
	return byID, period, nil
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
