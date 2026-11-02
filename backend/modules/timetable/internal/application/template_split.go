package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Template split (WP-B3, "Dieser und alle folgenden").
//
// Splits a recurring template at an effective date: the old template's
// schedules and rosters are capped (valid_until = effective date, exclusive),
// a successor template is created with the updated fields, the old template's
// still-planned future instances inside the materialization horizon are
// deleted, and (optionally) the window is re-materialized so the successor's
// instances appear on the grid immediately.
//
// Invariants:
//
//   - The old template row itself is never mutated beyond its schedule/roster
//     valid_until caps — history (started/completed/cancelled/spontaneous
//     instances) stays attached to the old template untouched.
//   - Only status='planned' AND is_spontaneous=false instances of the OLD
//     template are deleted, and only from the effective date onward.
//   - The successor's schedules start at the effective date (valid_from)
//     and end open (valid_until = NULL); its roster rows start at the
//     effective date.
//
// The service reuses an ambient TenantTxMiddleware transaction or opens an
// RLS-aware tenant transaction for direct callers. Any failure rolls back the
// whole split.

// TemplateSplitDependencies aggregates wiring. Every field is required except
// Logger (falls back to slog.Default), Staffing (nil announces nothing) and
// Today (defaults to the Berlin calendar day). ResyncOfferingRoster is
// Enrollment's offering-source resync (#2137); a split that changes the
// offering feed fails without it.
type TemplateSplitDependencies struct {
	GroupRepo            ports.GroupRows
	CategoryRepo         timetable.CategoryQuery
	PlanningTracks       ports.PlanningTrackAssignments
	ScheduleRepo         timetable.ScheduleCapability
	EnrollmentRepo       timetable.StudentEnrollmentCapability
	SupervisorRepo       timetable.PlannedSupervisorCapability
	Directories          ports.RepositoryDirectories
	InstanceRepo         ports.InstanceRows
	TimeframeRepo        timetable.TimeframeCapability
	Materialization      timetable.MaterializationCapability
	Deviations           ports.SeriesDeviations
	CareOfferings        ports.CareOfferingChecks
	ResyncOfferingRoster func(context.Context, timetable.OfferingRosterResyncInput) error
	RecurrenceLock       timetable.RecurrenceWriteLock
	SchoolClasses        ports.SchoolClassRules
	Staffing             ports.StaffingAnnouncer
	Logger               *slog.Logger
	Transactions         ports.TenantTransactions
	Today                func() calendar.Date
}

// TemplateSplitService performs recurring-template scope operations.
type TemplateSplitService struct {
	deps                 TemplateSplitDependencies
	lockTenantRecurrence func(context.Context) error
	runInTx              func(context.Context, func(context.Context) error) error
}

// NewTemplateSplitService constructs a TemplateSplitService. The split has
// no sensible degraded mode, so every required dependency must be wired.
func NewTemplateSplitService(deps TemplateSplitDependencies) (*TemplateSplitService, error) {
	if deps.GroupRepo == nil || deps.CategoryRepo == nil || deps.ScheduleRepo == nil || deps.EnrollmentRepo == nil ||
		deps.SupervisorRepo == nil || noInstanceRows(deps.InstanceRepo) || deps.TimeframeRepo == nil ||
		deps.Materialization == nil || deps.Deviations == nil || deps.RecurrenceLock == nil ||
		deps.CareOfferings.ValidateSeries == nil || deps.CareOfferings.ValidateOfferingSource == nil || deps.Transactions == nil {
		return nil, errors.New("timetable template split: required dependency is nil")
	}
	if err := validateSchoolClassRules(deps.SchoolClasses); err != nil {
		return nil, err
	}
	if deps.Today == nil {
		deps.Today = calendar.TodayDate
	}
	return &TemplateSplitService{
		deps:                 deps,
		lockTenantRecurrence: deps.RecurrenceLock.LockRecurrenceWrites,
		runInTx: func(ctx context.Context, fn func(context.Context) error) error {
			return inTenant(ctx, deps.Transactions, tenant.FromContext(ctx), func(txCtx context.Context) error {
				return fn(txCtx)
			})
		},
	}, nil
}

func (s *TemplateSplitService) getLogger() *slog.Logger {
	return OrDefaultLogger(s.deps.Logger)
}

// resolveSuccessorOfferingSource merges the presence-aware offering-source
// request fields with the old template (#2147 review round 14). Omitted
// fields inherit — the pre-#2137 split body must not silently cut the
// successor off its Betreuungsangebot feed — while submitted fields are
// authoritative, so an editor save that changes the Quelle or the
// Jahrgangsfilter and then picks the "following" scope actually lands on the
// successor instead of being dropped. The filter follows the source: an
// omitted filter stays with a kept source and falls with a cleared one, the
// same contract applyOfferingSourcePresence pins on the template PUT. The
// merged state then passes the same service validation as create/update
// (ErrOfferingSourceInvalid → 400), so a source next to explicit student_ids
// or per-weekday child lists is rejected rather than half-applied; per-weekday
// staff is carried onto the successor (#3165).
func resolveSuccessorOfferingSource(in *TemplateSplitInput, old *activityContract.Group, rules ports.SchoolClassRules) error {
	if in.TargetGroupType != activityContract.TargetGroupTypeAngebot {
		// The DB CHECK ties the source to 'angebot': a split away from the
		// type drops the rule regardless of what the request carried.
		in.SourceCareOfferingIDs = nil
		in.SourceGradeLevels = nil
		in.SourceSchoolClasses = nil
	} else {
		mergeSuccessorOfferingSource(in, old)
	}
	if err := validateOfferingSourceInput(
		in.SourceCareOfferingIDs, in.SourceGradeLevels, in.SourceSchoolClasses,
		in.TargetGroupType, in.StudentIDs, in.WeekdayAssignments, rules,
	); err != nil {
		return &ports.ScheduleError{Op: "split template: validate offering source", Err: err}
	}
	return nil
}

// mergeSuccessorOfferingSource fills the omitted source fields of an
// 'angebot' successor from the old template.
func mergeSuccessorOfferingSource(in *TemplateSplitInput, old *activityContract.Group) {
	if !in.SourceCareOfferingIDsProvided {
		in.SourceCareOfferingIDs = append([]int64(nil), old.SourceCareOfferingIDs...)
	}
	if len(in.SourceCareOfferingIDs) == 0 {
		in.SourceCareOfferingIDs = nil
		in.SourceGradeLevels = nil
		in.SourceSchoolClasses = nil
		return
	}
	if !in.SourceGradeLevelsProvided {
		in.SourceGradeLevels = append([]int(nil), old.SourceGradeLevels...)
	}
	if !in.SourceSchoolClassesProvided {
		in.SourceSchoolClasses = append([]string(nil), old.SourceSchoolClasses...)
	}
	// The two filters are mutually exclusive, so an explicitly provided one
	// replaces an inherited one instead of colliding with it (#2482):
	// switching a split successor from Jahrgang to Klasse must not fail
	// because the old filter came along.
	if in.SourceGradeLevelsProvided && len(in.SourceGradeLevels) > 0 && !in.SourceSchoolClassesProvided {
		in.SourceSchoolClasses = nil
	}
	if in.SourceSchoolClassesProvided && len(in.SourceSchoolClasses) > 0 && !in.SourceGradeLevelsProvided {
		in.SourceGradeLevels = nil
	}
}

// validateSuccessorOfferingSource rejects a split whose successor would carry
// its (merged) offering-source rule (#2137) into a calendar period the
// offering's phase does not fit, or point it at an unknown or newly selected
// inactive offering (a stored one that turned inactive is skipped, #5313).
// Create and update refuse those combinations (ErrOfferingSourceInvalid →
// 400); a "this and following" edit must not be the back door that persists
// them, leaving later approvals to skip the template with only a warning.
// Skipped entirely when the merge left the successor without a source.
func (s *TemplateSplitService) validateSuccessorOfferingSource(
	ctx context.Context,
	in TemplateSplitInput,
	storedOfferingIDs []int64,
) error {
	if len(in.SourceCareOfferingIDs) == 0 {
		return nil
	}
	// Nil only in package-local unit fixtures constructing the struct
	// directly; NewTemplateSplitService requires the callback.
	if s.deps.CareOfferings.ValidateOfferingSource == nil {
		return nil
	}
	if err := s.deps.CareOfferings.ValidateOfferingSource(ctx, in.SourceCareOfferingIDs, storedOfferingIDs, in.CalendarPeriodID); err != nil {
		// Client-correctable 400: keep TenantTxMiddleware from committing any
		// provisional split writes that preceded the check.
		tenant.MarkRollback(ctx)
		return &ports.ScheduleError{Op: "split template: validate offering source", Err: err}
	}
	return nil
}

// resyncSuccessorOfferingSource runs when the successor's offering feed differs
// from the old template's — the source was dropped (split away from
// 'angebot', explicit null), switched to another offering, added, or its
// Jahrgangsfilter changed (#2147 review rounds 8 and 14). createStudentRoster
// carries every provenance-tagged row regardless, so without the resync a
// changed rule would keep the old feed's children and never seed the new
// feed's. The resync reconciles the carried rows against the merged rule:
// stale rows are removed, missing children of the new source are seeded, and
// rows the legacy CareOffering.ActivityGroupID feed still plans on this
// series segment are protected by its lineage-wide legacy coverage. The
// successor has no materialized occurrences yet, so the resync's instance
// pass is a no-op; the split's own materialization runs afterwards and reads
// the reconciled roster. A successor that keeps a source is resynced even
// when the feed is unchanged: the carry-over copies each sourced row's period,
// but only Care Plan's resync decides which period a sourced row carries on
// the successor's schedules (#5176). Only a feed that was and stays empty
// skips it.
func (s *TemplateSplitService) resyncSuccessorOfferingSource(
	ctx context.Context,
	old, successor *activityContract.Group,
	in TemplateSplitInput,
) error {
	if len(successor.SourceCareOfferingIDs) == 0 && !offeringRosterFeedChanged(old, successor, s.deps.SchoolClasses.Normalize) {
		return nil
	}
	if s.deps.ResyncOfferingRoster == nil {
		return &ports.ScheduleError{Op: "split template: resync offering source", Err: errors.New("offering roster resync is not configured")}
	}
	if err := s.deps.ResyncOfferingRoster(ctx, timetable.OfferingRosterResyncInput{
		TemplateID:       successor.ID,
		OfferingIDs:      append([]int64(nil), successor.SourceCareOfferingIDs...),
		GradeLevels:      append([]int(nil), successor.SourceGradeLevels...),
		SchoolClasses:    append([]string(nil), successor.SourceSchoolClasses...),
		CalendarPeriodID: in.CalendarPeriodID,
		EffectiveFrom:    in.EffectiveDate,
		// The guard before the split checked the same ids against these.
		StoredOfferingIDs: append([]int64(nil), old.SourceCareOfferingIDs...),
	}); err != nil {
		return &ports.ScheduleError{Op: "split template: resync offering source", Err: err}
	}
	return nil
}

// resyncSplitPredecessorFromToday reconciles the capped predecessor from
// today up to a future cut when one of its stored sources turned inactive
// (#5313 review): such a source contributes nothing from today on, as on every
// template save, so its children leave the predecessor's rows and purely
// planned occurrences before the cut, while recorded and hand-decided
// attendance and history stay (Care Plan's resync). The successor's own
// resync starts at the cut. Predecessors whose stored sources still pass the
// guard as a fresh selection keep their rows untouched, as before. A
// predecessor whose remaining sources are refused too (every source inactive,
// another drift) keeps its rows as well.
func (s *TemplateSplitService) resyncSplitPredecessorFromToday(ctx context.Context, old *activityContract.Group, in TemplateSplitInput) error {
	today := s.deps.Today()
	if len(old.SourceCareOfferingIDs) == 0 || !in.EffectiveDate.After(today) || s.deps.CareOfferings.ValidateOfferingSource == nil {
		return nil
	}
	const op = "split template: resync predecessor offering source"
	if err := s.deps.CareOfferings.ValidateOfferingSource(ctx, old.SourceCareOfferingIDs, nil, old.CalendarPeriodID); err == nil {
		return nil
	} else if !errors.Is(err, timetable.ErrOfferingSourceInvalid) {
		return &ports.ScheduleError{Op: op, Err: err}
	}
	if s.deps.ResyncOfferingRoster == nil {
		return &ports.ScheduleError{Op: op, Err: errors.New("offering roster resync is not configured")}
	}
	err := s.deps.ResyncOfferingRoster(ctx, timetable.OfferingRosterResyncInput{
		TemplateID:        old.ID,
		OfferingIDs:       append([]int64(nil), old.SourceCareOfferingIDs...),
		GradeLevels:       append([]int(nil), old.SourceGradeLevels...),
		SchoolClasses:     append([]string(nil), old.SourceSchoolClasses...),
		CalendarPeriodID:  cloneOptionalInt64(old.CalendarPeriodID),
		EffectiveFrom:     today,
		StoredOfferingIDs: append([]int64(nil), old.SourceCareOfferingIDs...),
	})
	if err == nil {
		return nil
	}
	if !errors.Is(err, timetable.ErrOfferingSourceInvalid) {
		return &ports.ScheduleError{Op: op, Err: err}
	}
	s.getLogger().Warn("split template: predecessor keeps its rows, its stored sources are refused",
		slog.Int64("template_id", old.ID),
		slog.String("error", err.Error()),
	)
	return nil
}

// offeringRosterFeedChanged reports whether the successor's offering feed
// differs from what the roster carry-over copied: source added, removed,
// switched, or its Jahrgangsfilter changed. The filter comparison is
// order-insensitive — both sides are duplicate-free by validation.
func offeringRosterFeedChanged(old, successor *activityContract.Group, normalizeClass func(string) string) bool {
	// Order-SENSITIVE comparison on purpose: the id array's order is the
	// union's subtraction order, so a reordered set produces differently
	// shaped rows and must resync.
	if !slices.Equal(old.SourceCareOfferingIDs, successor.SourceCareOfferingIDs) {
		return true
	}
	if len(successor.SourceCareOfferingIDs) == 0 {
		return false
	}
	if !sameGradeLevelSet(old.SourceGradeLevels, successor.SourceGradeLevels) {
		return true
	}
	return !sameSchoolClassSet(old.SourceSchoolClasses, successor.SourceSchoolClasses, normalizeClass)
}

// sameSchoolClassSet compares two class filters order- and case-insensitively
// (#2482); both sides are duplicate-free by validation.
func sameSchoolClassSet(left, right []string, normalize func(string) string) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[string]struct{}, len(left))
	for _, class := range left {
		seen[normalize(class)] = struct{}{}
	}
	for _, class := range right {
		if _, ok := seen[normalize(class)]; !ok {
			return false
		}
	}
	return true
}

func sameGradeLevelSet(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[int]struct{}, len(left))
	for _, level := range left {
		seen[level] = struct{}{}
	}
	for _, level := range right {
		if _, ok := seen[level]; !ok {
			return false
		}
	}
	return true
}

func (s *TemplateSplitService) validateCareOfferingSeries(ctx context.Context, groupID int64) error {
	// NewTemplateSplitService requires this callback. The nil guard keeps
	// package-local unit fixtures that construct the service struct directly
	// focused on their unrelated behavior.
	if s.deps.CareOfferings.ValidateSeries == nil {
		return nil
	}
	return s.deps.CareOfferings.ValidateSeries(ctx, groupID)
}

// Split caps the old template at in.EffectiveDate (exclusive), creates a
// successor template from the updated fields, deletes the old template's
// planned future instances and optionally re-materializes the window.
// The service reuses an ambient request transaction or creates one when called
// directly, so the recurrence lock covers the complete operation.
func (s *TemplateSplitService) Split(ctx context.Context, in TemplateSplitInput) (*timetable.SplitTemplateResult, error) {
	if !in.targetsPresenceSet {
		in.targetsProvided = in.Targets != nil
		in.targetsPresenceSet = true
	}
	if err := validateSplitInput(&in, s.deps.Today(), s.deps.SchoolClasses); err != nil {
		return nil, err
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return nil, &ports.ScheduleError{Op: "split template", Err: errors.New("no tenant in context")}
	}
	if _, ok := tenant.TransactionFromContext(ctx); !ok && s.runInTx != nil {
		var result *timetable.SplitTemplateResult
		err := s.runInTx(ctx, func(txCtx context.Context) error {
			var err error
			result, err = s.Split(txCtx, in)
			return err
		})
		return result, err
	}
	return s.splitInTransaction(ctx, in, tenantID)
}

func (s *TemplateSplitService) splitInTransaction(
	ctx context.Context,
	in TemplateSplitInput,
	tenantID int64,
) (*timetable.SplitTemplateResult, error) {
	if err := s.lockSplitRecurrence(ctx, "split template"); err != nil {
		return nil, err
	}
	if err := validateAssignableCategory(ctx, s.deps.CategoryRepo, in.CategoryID, "split template: validate category"); err != nil {
		return nil, err
	}
	if err := s.lockSplitDays(ctx, in, tenantID); err != nil {
		return nil, err
	}
	old, newGroup, scheduleIDs, err := s.writeSplitSuccessor(ctx, &in, tenantID)
	if err != nil {
		return nil, err
	}
	preserved, err := s.preserveSplitDeviations(ctx, in, old.ID)
	if err != nil {
		return nil, err
	}

	deleted, err := s.deleteSplitSourceInstances(ctx, old.ID, in.EffectiveDate)
	if err != nil {
		return nil, err
	}

	mat, err := s.materializeWindow(ctx, in)
	if err != nil {
		return nil, err
	}
	snapshotted, reapplied, err := reapplySplitDeviations(ctx, preserved, newGroup.ID, in.ActorAccountID)
	if err != nil {
		return nil, err
	}

	// The delete side bypasses the CRUD broadcast paths; the materializer
	// broadcasts separately for the successor rows it created.
	if deleted > 0 {
		announceStaffingChanged(ctx, s.deps.Staffing, s.getLogger(), "template_split")
	}

	s.getLogger().Info("template split completed",
		slog.Int64("tenant_id", tenantID),
		slog.Int64("old_template_id", old.ID),
		slog.Int64("new_template_id", newGroup.ID),
		slog.String("effective_date", in.EffectiveDate.String()),
		slog.Int("schedule_count", len(scheduleIDs)),
		slog.Int64("deleted_instances", deleted),
		slog.Int("deviations_snapshotted", snapshotted),
		slog.Int("deviations_reapplied", reapplied),
	)

	return &timetable.SplitTemplateResult{
		OldTemplateID:    old.ID,
		NewTemplateID:    newGroup.ID,
		NewScheduleIDs:   scheduleIDs,
		DeletedInstances: int(deleted),
		Materialization:  mat,
	}, nil
}

// deleteSplitSourceInstances removes ALL of the old template's still-planned
// future instances from the effective date onward — deliberately open-ended
// (nil `to`), NOT tied to the materialization window: the split must
// guarantee that no planned non-spontaneous old-template instance survives
// on/after the effective date, however far materialization once reached.
// Started/completed/cancelled and spontaneous rows survive (same protection
// rule as ReplanWeek). preserveDeviations=false: split is the destructive
// series operation. A surviving deviated old-template row would dedupe apart
// from the new template's materialized successor and show as a duplicate
// block (#1840).
func (s *TemplateSplitService) deleteSplitSourceInstances(ctx context.Context, oldID int64, effectiveDate calendar.Date) (int64, error) {
	deleted, err := deletePlannedInstancesInWindow(ctx, s.deps.InstanceRepo.Plans, calendar.Date(effectiveDate), nil, &oldID, false)
	if err != nil {
		return 0, &ports.ScheduleError{Op: "split template: delete planned instances", Err: err}
	}
	return deleted, nil
}

func (s *TemplateSplitService) lockSplitRecurrence(ctx context.Context, op string) error {
	if s.lockTenantRecurrence == nil {
		return &ports.ScheduleError{Op: op + ": lock recurrence", Err: errors.New("template recurrence lock is not configured")}
	}
	if err := s.lockTenantRecurrence(ctx); err != nil {
		return &ports.ScheduleError{Op: op + ": lock recurrence", Err: err}
	}
	return nil
}

// writeSplitSuccessor caps the old template, creates and validates the
// successor with its schedules and roster, and resyncs a changed offering
// feed.
func (s *TemplateSplitService) writeSplitSuccessor(
	ctx context.Context,
	in *TemplateSplitInput,
	tenantID int64,
) (*activityContract.Group, *activityContract.Group, []int64, error) {
	old, sourceValidUntil, activeEnrollments, activeSupervisors, err := s.prepareSplitSource(ctx, in)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := s.validateSuccessorOfferingSource(ctx, *in, old.SourceCareOfferingIDs); err != nil {
		return nil, nil, nil, err
	}
	if in.PlanningTrackIDProvided && !samePlanningTrackID(in.PlanningTrackID, old.PlanningTrackID) {
		if err := validateAssignablePlanningTrack(ctx, s.deps.PlanningTracks, in.PlanningTrackID, old.PlanningTrackID); err != nil {
			return nil, nil, nil, err
		}
	}
	newGroup, scheduleIDs, err := s.createSplitSuccessor(
		ctx, old, *in, tenantID, sourceValidUntil, activeEnrollments, activeSupervisors,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := s.resyncSuccessorOfferingSource(ctx, old, newGroup, *in); err != nil {
		return nil, nil, nil, err
	}
	if err := s.resyncSplitPredecessorFromToday(ctx, old, *in); err != nil {
		return nil, nil, nil, err
	}
	if err := s.validateCareOfferingSeries(ctx, newGroup.ID); err != nil {
		return nil, nil, nil, careOfferingValidationError(
			ctx, s.deps.CareOfferings,
			"split template: validate linked care offerings",
			"successor is incompatible with an existing care offering",
			err,
		)
	}
	return old, newGroup, scheduleIDs, nil
}

// preserveSplitDeviations snapshots the old template's deviations under the
// days lockSplitDays acquired before any writes. Without a materialization
// window there is nothing to preserve.
func (s *TemplateSplitService) preserveSplitDeviations(ctx context.Context, in TemplateSplitInput, oldID int64) (ports.PreservedDeviations, error) {
	deviationFrom, deviationTo, preserve := splitMaterializationWindow(in)
	if !preserve {
		return nil, nil
	}
	if s.deps.Deviations == nil {
		return nil, &ports.ScheduleError{Op: "split template: preserve deviations", Err: errors.New("deviation preservation is not configured")}
	}
	preserved, err := s.deps.Deviations.SnapshotDeviations(ctx, deviationFrom, deviationTo, oldID)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "split template: snapshot deviations", Err: err}
	}
	return preserved, nil
}

// reapplySplitDeviations writes the snapshot onto the successor's
// rematerialized occurrences. It reports the snapshot size and the number
// reapplied.
func reapplySplitDeviations(ctx context.Context, preserved ports.PreservedDeviations, newID int64, actorAccountID *int64) (int, int, error) {
	if preserved == nil {
		return 0, 0, nil
	}
	reapplied, err := preserved.Reapply(ctx, newID, actorAccountID)
	if err != nil {
		return 0, 0, &ports.ScheduleError{Op: "split template: reapply deviations", Err: err}
	}
	return preserved.Count(), reapplied, nil
}

func (s *TemplateSplitService) prepareSplitSource(
	ctx context.Context,
	in *TemplateSplitInput,
) (*activityContract.Group, *calendar.Date, []*activityContract.StudentEnrollment, []*activityContract.SupervisorPlanned, error) {
	old, err := s.loadTemplate(ctx, in.TemplateID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if err := s.resolveSplitTargets(ctx, in, old); err != nil {
		return nil, nil, nil, nil, err
	}
	// Runs before any write: a rejected merge surfaces as a plain 400 with
	// nothing to roll back.
	if err := resolveSuccessorOfferingSource(in, old, s.deps.SchoolClasses); err != nil {
		return nil, nil, nil, nil, err
	}
	effectiveDate, sourceValidUntil, err := s.normalizeEffectiveDateInSegment(ctx, old.ID, in.EffectiveDate, "split template", false)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if sourceValidUntil != nil {
		return nil, nil, nil, nil, fmt.Errorf(
			"%w: bounded template segments cannot be split again",
			timetable.ErrSplitInvalidInput,
		)
	}
	in.EffectiveDate = effectiveDate

	// Load before capping: the active-row predicates no longer match after it.
	enrollments, supervisors, err := s.loadSegmentRosterCandidates(ctx, old.ID, in.EffectiveDate, sourceValidUntil)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if err := s.capSplitSource(ctx, old.ID, in.EffectiveDate, enrollments, supervisors, sourceValidUntil); err != nil {
		return nil, nil, nil, nil, err
	}
	return old, sourceValidUntil, enrollments, supervisors, nil
}

// resolveSplitTargets keeps the old template's dynamic targets when the split
// sent none, checks them against the grade cap and mirrors the first target
// onto the successor fields.
func (s *TemplateSplitService) resolveSplitTargets(ctx context.Context, in *TemplateSplitInput, old *activityContract.Group) error {
	existingTargets, err := loadExistingDynamicTargets(ctx, s.deps.GroupRepo, old.ID)
	if err != nil {
		return fmt.Errorf("load existing dynamic targets: %w", err)
	}
	if !in.targetsProvided && len(existingTargets) > 0 {
		in.Targets = existingTargets
		in.TargetGroupType = existingTargets[0].TargetGroupType
		in.TargetGradeLevel = existingTargets[0].TargetGradeLevel
		in.TargetSchoolClass = existingTargets[0].TargetSchoolClass
		if in.TargetGroupType == activityContract.TargetGroupTypeGruppe {
			in.EducationGroupID = existingTargets[0].EducationGroupID
		}
	}
	if err := validateTemplateTargetsGradeLimit(s.deps.SchoolClasses, in.GradeLevelMax, old, existingTargets, in.Targets); err != nil {
		return fmt.Errorf("%w: %w", timetable.ErrSplitInvalidInput, err)
	}
	mirrorSplitTargets(in)
	return nil
}

// mirrorSplitTargets copies the first dynamic target onto the successor's
// group mirror fields.
func mirrorSplitTargets(in *TemplateSplitInput) {
	in.TargetGradeLevel = nil
	in.TargetSchoolClass = nil
	if len(in.Targets) == 0 {
		return
	}
	in.TargetGradeLevel = in.Targets[0].TargetGradeLevel
	in.TargetSchoolClass = in.Targets[0].TargetSchoolClass
	if in.TargetGroupType == activityContract.TargetGroupTypeGruppe {
		in.EducationGroupID = in.Targets[0].EducationGroupID
	}
}

func (s *TemplateSplitService) capSplitSource(
	ctx context.Context,
	groupID int64,
	effectiveDate calendar.Date,
	enrollments []*activityContract.StudentEnrollment,
	supervisors []*activityContract.SupervisorPlanned,
	sourceValidUntil *calendar.Date,
) error {
	if _, err := capLegacySchedules(ctx, s.deps.ScheduleRepo, groupID, effectiveDate.String()); err != nil {
		return &ports.ScheduleError{Op: "split template: cap schedules", Err: err}
	}
	if _, err := capLegacyStudentEnrollments(ctx, s.deps.EnrollmentRepo, groupID, activityContract.Date(effectiveDate)); err != nil {
		return &ports.ScheduleError{Op: "split template: cap enrollments", Err: err}
	}
	if _, err := capLegacyPlannedSupervisors(ctx, s.deps.SupervisorRepo, groupID, activityContract.Date(effectiveDate)); err != nil {
		return &ports.ScheduleError{Op: "split template: cap supervisors", Err: err}
	}
	if _, err := s.capBoundedSegmentEnrollments(ctx, enrollments, sourceValidUntil, effectiveDate); err != nil {
		return err
	}
	if _, err := s.capBoundedSegmentSupervisors(ctx, supervisors, sourceValidUntil, effectiveDate); err != nil {
		return err
	}
	return nil
}

func (s *TemplateSplitService) createSplitSuccessor(
	ctx context.Context,
	old *activityContract.Group,
	in TemplateSplitInput,
	tenantID int64,
	sourceValidUntil *calendar.Date,
	enrollments []*activityContract.StudentEnrollment,
	supervisors []*activityContract.SupervisorPlanned,
) (*activityContract.Group, []int64, error) {
	timeframeID, err := findOrCreateTimeframe(ctx, s.deps.TimeframeRepo, in.StartTime, in.EndTime, in.Name)
	if err != nil {
		return nil, nil, &ports.ScheduleError{Op: "split template: resolve timeframe", Err: err}
	}
	newGroup, err := s.createSuccessorGroup(ctx, old, in, tenantID)
	if err != nil {
		return nil, nil, err
	}
	scheduleIDs, err := s.createSuccessorSchedules(ctx, newGroup.ID, timeframeID, in, tenantID, sourceValidUntil)
	if err != nil {
		return nil, nil, err
	}
	if err := s.createStudentRoster(ctx, newGroup.ID, in, enrollments, tenantID, sourceValidUntil); err != nil {
		return nil, nil, err
	}
	if err := s.createStaffRoster(ctx, newGroup.ID, in, supervisors, tenantID, sourceValidUntil); err != nil {
		return nil, nil, err
	}
	return newGroup, scheduleIDs, nil
}

// materializeWindow re-materializes the requested window, clamped so it never
// starts before the effective date. Requires both bounds; an inverted clamped
// window is a silent no-op (nothing to materialize before the split point).
func (s *TemplateSplitService) materializeWindow(ctx context.Context, in TemplateSplitInput) (*timetable.MaterializationResult, error) {
	from, to, ok := splitMaterializationWindow(in)
	if !ok {
		return nil, nil
	}
	mat, err := s.deps.Materialization.MaterializeForTenant(ctx, from, to, timetable.MaterializationSourceManual)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "split template: materialize", Err: err}
	}
	return mat, nil
}

// splitMaterializationWindow returns the exact inclusive window shared by
// deviation preservation and materialization. It clamps the lower bound to the
// split date because the predecessor remains authoritative before that date.
func splitMaterializationWindow(in TemplateSplitInput) (calendar.Date, calendar.Date, bool) {
	if in.MaterializeFrom == nil || in.MaterializeTo == nil {
		return calendar.Date(""), calendar.Date(""), false
	}
	from := *in.MaterializeFrom
	if from.Before(in.EffectiveDate) {
		from = in.EffectiveDate
	}
	to := *in.MaterializeTo
	if from.After(to) {
		return calendar.Date(""), calendar.Date(""), false
	}
	return from, to, true
}
