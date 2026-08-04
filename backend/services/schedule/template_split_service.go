// Package schedule — template split service (WP-B3, "Dieser und alle folgenden").
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
package schedule

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModel "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Sentinel errors callers branch on via errors.Is.
var (
	// ErrSplitTemplateNotFound is returned when the template id does not
	// resolve to a non-archived template in the current tenant. → 404.
	ErrSplitTemplateNotFound = errors.New("template not found")

	// ErrSplitInvalidInput is returned for semantically invalid split input
	// (past effective date, bad weekdays, …). → 400.
	ErrSplitInvalidInput = errors.New("invalid template split input")

	// ErrTemplateCareOfferingConflict identifies a recurrence mutation that
	// would invalidate an existing care-offering link. All template mutation
	// endpoints expose this as the same stable HTTP 400 contract.
	ErrTemplateCareOfferingConflict = errors.New("template recurrence conflicts with an existing care offering")

	// ErrTemplateRosterRebaseConflict identifies a period change that would
	// collapse two protected active roster rows onto the same period-scoped
	// unique key. The mutation must be rejected rather than dropping provenance.
	ErrTemplateRosterRebaseConflict = errors.New("protected template roster cannot be rebased without a collision")
)

func templateCareOfferingValidationError(ctx context.Context, op, action string, err error) error {
	// TenantTxMiddleware commits ordinary 4xx responses. Mark the ambient
	// transaction explicitly so a client-correctable conflict cannot persist
	// the provisional schedule/group/roster mutations that revealed it.
	tenant.MarkRollback(ctx)
	if errors.Is(err, enrollmentModel.ErrCareOfferingInvalid) {
		return &ScheduleError{
			Op: op,
			Err: fmt.Errorf(
				"%w: %w: %s: %w",
				ErrSplitInvalidInput,
				ErrTemplateCareOfferingConflict,
				action,
				err,
			),
		}
	}
	// Lock/repository failures are operational errors. Do not disguise them
	// as administrator input or expose their details through a 400 response.
	return &ScheduleError{Op: op, Err: fmt.Errorf("validate linked care offerings: %w", err)}
}

// TemplateSplitInput mirrors the update-template request plus the split
// controls. StartTime/EndTime are wall-clock values (parsed from HH:MM).
// StudentIDs/StaffIDs follow tri-state semantics: nil = carry over the
// previously-active roster of the old template; non-nil (including empty) =
// use exactly the provided ids. PrimaryStaffID applies to an explicitly
// provided StaffIDs roster; carried-over supervisors keep their own
// is_primary flag.
type TemplateSplitInput struct {
	TemplateID      int64
	EffectiveDate   timezone.Date
	Name            string
	Type            string // care | activity | external
	Weekdays        []int  // ISO 8601, Mo=1 … Su=7
	StartTime       time.Time
	EndTime         time.Time
	RoomID          int64
	CategoryID      int64
	MaxParticipants *int
	// RequiredStaff is the manual Personalbedarf override (#1839) for the
	// successor Group, already normalized (nil = clear/derive). It is only
	// applied when RequiredStaffProvided is true; otherwise the successor
	// inherits the source template's override. This omitted-vs-null split is
	// what lets a "this and following" edit actually CLEAR an override.
	RequiredStaff         *int
	RequiredStaffProvided bool
	WeekPattern           *int // 0=every week, 1=A, 2=B; nil = 0
	CalendarPeriodID      *int64
	EducationGroupID      *int64
	// Zielgruppe (target-group) fields, carried onto the successor Group
	// (see createSuccessorGroup). "gruppe" reuses EducationGroupID above.
	TargetGroupType    string
	TargetGradeLevel   *int16
	TargetSchoolClass  *string
	Targets            []*activitiesModel.GroupTarget
	targetsProvided    bool
	targetsPresenceSet bool
	// Notes is the durable Wochennotiz (#1837 follow-up) for the successor
	// Group, already normalized (nil = clear). Only applied when NotesProvided
	// is true; otherwise the successor inherits the source template's note. Same
	// omitted-vs-null split as RequiredStaff so a "this and following" edit can
	// actually CLEAR the series note.
	Notes         *string
	NotesProvided bool
	// ListKind classifies the successor for printable daily lists (#1565),
	// already normalized (nil = clear). Same omitted-vs-null split as
	// RequiredStaff/Notes: only applied when ListKindProvided is true,
	// otherwise the successor inherits the source template's list kind —
	// without this a plain "this and following" edit would silently drop the
	// series from its automatic Randstunden/Lernzeit/AG/Mensa list.
	ListKind         *string
	ListKindProvided bool
	StudentIDs       []int64
	StaffIDs         []int64
	PrimaryStaffID   *int64
	// WeekdayAssignments carries the per-weekday roster deviations (#2129)
	// onto the successor. It only applies to the explicit-roster path: a
	// carried-over roster keeps each row's own weekday scope.
	WeekdayAssignments []WeekdayRosterAssignment
	MaterializeFrom    *timezone.Date
	MaterializeTo      *timezone.Date
	// GradeLevelMax is the caller's validated snapshot of
	// enrollment.grade_level_max. Missing or out-of-range values are rejected.
	GradeLevelMax int
	// ActorAccountID stamps Änderungsprotokoll entries for deviations the
	// split drops (#1886); nil records actor-less events.
	ActorAccountID *int64
}

// TemplateSplitResult summarises one Split call.
type TemplateSplitResult struct {
	OldTemplateID    int64
	NewTemplateID    int64
	NewScheduleIDs   []int64
	DeletedInstances int
	Materialization  *MaterializationResult
}

// TemplateEndInput describes the destructive "delete this and following"
// recurrence action. EffectiveDate is inclusive for planned instance deletes
// and exclusive for schedule/roster valid_until caps.
type TemplateEndInput struct {
	TemplateID    int64
	EffectiveDate timezone.Date
}

// TemplateEndResult summarises one EndFromDate call.
type TemplateEndResult struct {
	TemplateID        int64
	EffectiveDate     timezone.Date
	DeletedInstances  int
	CappedSchedules   int64
	CappedEnrollments int64
	CappedSupervisors int64
}

// TemplateSplitDependencies aggregates wiring. All fields are required;
// Logger may be nil (falls back to slog.Default).
type TemplateSplitDependencies struct {
	GroupRepo       activitiesModel.GroupRepository
	CategoryRepo    activitiesModel.CategoryRepository
	ScheduleRepo    activitiesModel.ScheduleRepository
	EnrollmentRepo  activitiesModel.StudentEnrollmentRepository
	SupervisorRepo  activitiesModel.SupervisorPlannedRepository
	InstanceRepo    scheduleModel.ActivityInstanceRepository
	TimeframeRepo   scheduleModel.TimeframeRepository
	Materialization MaterializationService
	// InstanceService supplies the existing deviation snapshot/reapply machinery.
	// The constructor narrows it to the package-private capability used by Split.
	InstanceService InstanceService
	// ValidateCareOfferingSeries runs after the successor and its schedules are
	// visible inside the split transaction. It rejects a split that would make
	// an existing care-offering link impossible to materialize later.
	ValidateCareOfferingSeries func(context.Context, int64) error
	// Broadcaster is optional (nil → no SSE). Split/end delete planned
	// instances outside the CRUD broadcast paths, so they must invalidate the
	// staffing caches themselves (#1844).
	Broadcaster realtime.Broadcaster
	Logger      *slog.Logger
	DB          *bun.DB
}

// TemplateSplitService performs recurring-template scope operations.
type TemplateSplitService struct {
	deps                 TemplateSplitDependencies
	deviations           splitDeviationPreserver
	lockTenantRecurrence func(context.Context) error
	runInTx              func(context.Context, func(context.Context) error) error
}

// splitDeviationPreserver is the existing instance-service machinery the split
// needs around its delete/materialize cycle. Keeping this capability private
// avoids adding snapshot types or split-only methods to the public service API.
type splitDeviationPreserver interface {
	acquireSubstituteDayLocks(context.Context, int64, timezone.Date, timezone.Date) error
	snapshotDeviations(context.Context, timezone.Date, timezone.Date, *int64) ([]deviationSnapshot, map[groupDay]int, error)
	reapplyDeviations(context.Context, []deviationSnapshot, map[groupDay]int, *int64, *int64) (int, error)
}

// NewTemplateSplitService constructs a TemplateSplitService. Panics if a
// required dependency is nil — the split has no sensible degraded mode, so
// the factory must wire it completely at startup.
func NewTemplateSplitService(deps TemplateSplitDependencies) *TemplateSplitService {
	if deps.GroupRepo == nil || deps.CategoryRepo == nil || deps.ScheduleRepo == nil || deps.EnrollmentRepo == nil ||
		deps.SupervisorRepo == nil || deps.InstanceRepo == nil || deps.TimeframeRepo == nil ||
		deps.Materialization == nil || deps.InstanceService == nil ||
		deps.ValidateCareOfferingSeries == nil || deps.DB == nil {
		panic("schedule.NewTemplateSplitService: required dependency is nil")
	}
	deviations, ok := deps.InstanceService.(splitDeviationPreserver)
	if !ok {
		panic("schedule.NewTemplateSplitService: instance service does not support deviation preservation")
	}
	return &TemplateSplitService{
		deps:       deps,
		deviations: deviations,
		lockTenantRecurrence: func(ctx context.Context) error {
			return lockTenantRecurrenceWrites(ctx, deps.DB)
		},
		runInTx: func(ctx context.Context, fn func(context.Context) error) error {
			return tenant.WithTenantTx(ctx, deps.DB, tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
				return fn(txCtx)
			})
		},
	}
}

func (s *TemplateSplitService) getLogger() *slog.Logger {
	return cmp.Or(s.deps.Logger, slog.Default())
}

func (s *TemplateSplitService) validateCareOfferingSeries(ctx context.Context, groupID int64) error {
	// NewTemplateSplitService requires this callback in production. The nil
	// guard keeps legacy package-local unit fixtures that construct the service
	// struct directly focused on their unrelated behavior.
	if s.deps.ValidateCareOfferingSeries == nil {
		return nil
	}
	return s.deps.ValidateCareOfferingSeries(ctx, groupID)
}

// Split caps the old template at in.EffectiveDate (exclusive), creates a
// successor template from the updated fields, deletes the old template's
// planned future instances and optionally re-materializes the window.
// The service reuses an ambient request transaction or creates one when called
// directly, so the recurrence lock covers the complete operation.
func (s *TemplateSplitService) Split(ctx context.Context, in TemplateSplitInput) (*TemplateSplitResult, error) {
	if !in.targetsPresenceSet {
		in.targetsProvided = in.Targets != nil
		in.targetsPresenceSet = true
	}
	if err := validateSplitInput(&in); err != nil {
		return nil, err
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return nil, &ScheduleError{Op: "split template", Err: errors.New("no tenant in context")}
	}
	if _, ok := modelBase.TxFromContext(ctx); !ok && s.runInTx != nil {
		var result *TemplateSplitResult
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
) (*TemplateSplitResult, error) {
	if s.lockTenantRecurrence == nil {
		return nil, &ScheduleError{Op: "split template: lock recurrence", Err: errors.New("template recurrence lock is not configured")}
	}
	if err := s.lockTenantRecurrence(ctx); err != nil {
		return nil, &ScheduleError{Op: "split template: lock recurrence", Err: err}
	}
	if err := validateAssignableCategory(ctx, s.deps.CategoryRepo, in.CategoryID, "split template: validate category"); err != nil {
		return nil, err
	}

	old, sourceValidUntil, activeEnrollments, activeSupervisors, err := s.prepareSplitSource(ctx, &in)
	if err != nil {
		return nil, err
	}
	newGroup, scheduleIDs, err := s.createSplitSuccessor(
		ctx, old, in, tenantID, sourceValidUntil, activeEnrollments, activeSupervisors,
	)
	if err != nil {
		return nil, err
	}
	if err := s.validateCareOfferingSeries(ctx, newGroup.ID); err != nil {
		return nil, templateCareOfferingValidationError(
			ctx,
			"split template: validate linked care offerings",
			"successor is incompatible with an existing care offering",
			err,
		)
	}

	var snapshots []deviationSnapshot
	var occurrences map[groupDay]int
	deviationFrom, deviationTo, preserveDeviations := splitMaterializationWindow(in)
	if preserveDeviations {
		if s.deviations == nil {
			return nil, &ScheduleError{Op: "split template: preserve deviations", Err: errors.New("deviation preservation is not configured")}
		}
		if err := s.deviations.acquireSubstituteDayLocks(ctx, tenantID, deviationFrom, deviationTo); err != nil {
			return nil, &ScheduleError{Op: "split template: lock deviation days", Err: err}
		}
		oldID := old.ID
		snapshots, occurrences, err = s.deviations.snapshotDeviations(ctx, deviationFrom, deviationTo, &oldID)
		if err != nil {
			return nil, &ScheduleError{Op: "split template: snapshot deviations", Err: err}
		}
	}

	// Remove ALL of the old template's still-planned future instances from
	// the effective date onward — deliberately open-ended (nil `to`), NOT
	// tied to the materialization window: the split must guarantee that no
	// planned non-spontaneous old-template instance survives on/after the
	// effective date, however far materialization once reached.
	// Started/completed/cancelled and spontaneous rows survive (same
	// protection rule as ReplanWeek).
	oldID := old.ID
	// preserveDeviations=false: split is the destructive series operation. A
	// surviving deviated old-template row would dedupe apart from the new
	// template's materialized successor and show as a duplicate block (#1840).
	deleted, err := s.deps.InstanceRepo.DeletePlannedNonSpontaneousInWindow(ctx, in.EffectiveDate, nil, &oldID, false)
	if err != nil {
		return nil, &ScheduleError{Op: "split template: delete planned instances", Err: err}
	}

	mat, err := s.materializeWindow(ctx, in)
	if err != nil {
		return nil, err
	}

	reapplied := 0
	if preserveDeviations {
		newID := newGroup.ID
		reapplied, err = s.deviations.reapplyDeviations(ctx, snapshots, occurrences, &newID, in.ActorAccountID)
		if err != nil {
			return nil, &ScheduleError{Op: "split template: reapply deviations", Err: err}
		}
	}

	// The delete side bypasses the CRUD broadcast paths; the materializer
	// broadcasts separately for the successor rows it created.
	if deleted > 0 {
		broadcastStaffingChanged(ctx, s.deps.Broadcaster, s.getLogger(), "template_split")
	}

	s.getLogger().Info("template split completed",
		slog.Int64("tenant_id", tenantID),
		slog.Int64("old_template_id", old.ID),
		slog.Int64("new_template_id", newGroup.ID),
		slog.String("effective_date", in.EffectiveDate.String()),
		slog.Int("schedule_count", len(scheduleIDs)),
		slog.Int64("deleted_instances", deleted),
		slog.Int("deviations_snapshotted", len(snapshots)),
		slog.Int("deviations_reapplied", reapplied),
	)

	return &TemplateSplitResult{
		OldTemplateID:    old.ID,
		NewTemplateID:    newGroup.ID,
		NewScheduleIDs:   scheduleIDs,
		DeletedInstances: int(deleted),
		Materialization:  mat,
	}, nil
}

func (s *TemplateSplitService) prepareSplitSource(
	ctx context.Context,
	in *TemplateSplitInput,
) (*activitiesModel.Group, *timezone.Date, []*activitiesModel.StudentEnrollment, []*activitiesModel.SupervisorPlanned, error) {
	old, err := s.loadTemplate(ctx, in.TemplateID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	existingTargets, err := loadExistingDynamicTargets(ctx, s.deps.GroupRepo, old.ID)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("load existing dynamic targets: %w", err)
	}
	if !in.targetsProvided && len(existingTargets) > 0 {
		in.Targets = existingTargets
		in.TargetGroupType = existingTargets[0].TargetGroupType
		in.TargetGradeLevel = existingTargets[0].TargetGradeLevel
		in.TargetSchoolClass = existingTargets[0].TargetSchoolClass
		if in.TargetGroupType == activitiesModel.TargetGroupTypeGruppe {
			in.EducationGroupID = existingTargets[0].EducationGroupID
		}
	}
	if err := validateSplitDynamicTargets(in.GradeLevelMax, old, existingTargets, in.Targets); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("%w: %w", ErrSplitInvalidInput, err)
	}
	in.TargetGradeLevel = nil
	in.TargetSchoolClass = nil
	if len(in.Targets) > 0 {
		in.TargetGradeLevel = in.Targets[0].TargetGradeLevel
		in.TargetSchoolClass = in.Targets[0].TargetSchoolClass
		if in.TargetGroupType == activitiesModel.TargetGroupTypeGruppe {
			in.EducationGroupID = in.Targets[0].EducationGroupID
		}
	}
	effectiveDate, sourceValidUntil, err := s.normalizeEffectiveDateInSegment(ctx, old.ID, in.EffectiveDate, "split template", false)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if sourceValidUntil != nil {
		return nil, nil, nil, nil, fmt.Errorf(
			"%w: bounded template segments cannot be split again",
			ErrSplitInvalidInput,
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

func validateSplitDynamicTargets(gradeLevelMax int, existing *activitiesModel.Group, existingTargets, targets []*activitiesModel.GroupTarget) error {
	return ValidateTemplateTargetsGradeLimit(gradeLevelMax, existing, existingTargets, targets)
}

func (s *TemplateSplitService) capSplitSource(
	ctx context.Context,
	groupID int64,
	effectiveDate timezone.Date,
	enrollments []*activitiesModel.StudentEnrollment,
	supervisors []*activitiesModel.SupervisorPlanned,
	sourceValidUntil *timezone.Date,
) error {
	if _, err := s.deps.ScheduleRepo.CapValidUntil(ctx, groupID, effectiveDate); err != nil {
		return &ScheduleError{Op: "split template: cap schedules", Err: err}
	}
	if _, err := s.deps.EnrollmentRepo.CapActiveByGroup(ctx, groupID, effectiveDate); err != nil {
		return &ScheduleError{Op: "split template: cap enrollments", Err: err}
	}
	if _, err := s.deps.SupervisorRepo.CapActiveByGroup(ctx, groupID, effectiveDate); err != nil {
		return &ScheduleError{Op: "split template: cap supervisors", Err: err}
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
	old *activitiesModel.Group,
	in TemplateSplitInput,
	tenantID int64,
	sourceValidUntil *timezone.Date,
	enrollments []*activitiesModel.StudentEnrollment,
	supervisors []*activitiesModel.SupervisorPlanned,
) (*activitiesModel.Group, []int64, error) {
	timeframeID, err := FindOrCreateTimeframe(ctx, s.deps.TimeframeRepo, in.StartTime, in.EndTime, in.Name)
	if err != nil {
		return nil, nil, &ScheduleError{Op: "split template: resolve timeframe", Err: err}
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

// EndFromDate caps a template from the effective date onward without creating
// a successor ("Dieser und alle folgenden löschen") — intentionally the
// destructive half of Split. Planned non-spontaneous future instances are
// deleted; active/completed/cancelled/spontaneous rows remain as history.
func (s *TemplateSplitService) EndFromDate(ctx context.Context, in TemplateEndInput) (*TemplateEndResult, error) {
	if err := validateTemplateEndInput(in); err != nil {
		return nil, err
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return nil, &ScheduleError{Op: "end template", Err: errors.New("no tenant in context")}
	}
	if _, ok := modelBase.TxFromContext(ctx); !ok && s.runInTx != nil {
		return s.endFromDateWithTransaction(ctx, in, tenantID)
	}
	return s.endFromDateInTransaction(ctx, in, tenantID)
}

func (s *TemplateSplitService) endFromDateWithTransaction(
	ctx context.Context,
	in TemplateEndInput,
	tenantID int64,
) (*TemplateEndResult, error) {
	var result *TemplateEndResult
	err := s.runInTx(ctx, func(txCtx context.Context) error {
		var innerErr error
		result, innerErr = s.endFromDateInTransaction(txCtx, in, tenantID)
		return innerErr
	})
	return result, err
}

func (s *TemplateSplitService) endFromDateInTransaction(
	ctx context.Context,
	in TemplateEndInput,
	tenantID int64,
) (*TemplateEndResult, error) {
	if s.lockTenantRecurrence == nil {
		return nil, &ScheduleError{Op: "end template: lock recurrence", Err: errors.New("template recurrence lock is not configured")}
	}
	if err := s.lockTenantRecurrence(ctx); err != nil {
		return nil, &ScheduleError{Op: "end template: lock recurrence", Err: err}
	}

	old, err := s.loadTemplate(ctx, in.TemplateID)
	if err != nil {
		return nil, err
	}
	effectiveDate, sourceValidUntil, err := s.normalizeEffectiveDateInSegment(ctx, old.ID, in.EffectiveDate, "end template", true)
	if err != nil {
		return nil, err
	}
	in.EffectiveDate = effectiveDate
	segmentEnrollments, segmentSupervisors, err := s.loadSegmentRosterCandidates(ctx, old.ID, in.EffectiveDate, sourceValidUntil)
	if err != nil {
		return nil, err
	}

	cappedSchedules, err := s.deps.ScheduleRepo.CapValidUntil(ctx, old.ID, in.EffectiveDate)
	if err != nil {
		return nil, &ScheduleError{Op: "end template: cap schedules", Err: err}
	}
	cappedEnrollments, err := s.deps.EnrollmentRepo.CapActiveByGroup(ctx, old.ID, in.EffectiveDate)
	if err != nil {
		return nil, &ScheduleError{Op: "end template: cap enrollments", Err: err}
	}
	cappedSupervisors, err := s.deps.SupervisorRepo.CapActiveByGroup(ctx, old.ID, in.EffectiveDate)
	if err != nil {
		return nil, &ScheduleError{Op: "end template: cap supervisors", Err: err}
	}
	cappedBoundedEnrollments, err := s.capBoundedSegmentEnrollments(ctx, segmentEnrollments, sourceValidUntil, in.EffectiveDate)
	if err != nil {
		return nil, err
	}
	cappedEnrollments += cappedBoundedEnrollments
	cappedBoundedSupervisors, err := s.capBoundedSegmentSupervisors(ctx, segmentSupervisors, sourceValidUntil, in.EffectiveDate)
	if err != nil {
		return nil, err
	}
	cappedSupervisors += cappedBoundedSupervisors
	if err := s.validateCareOfferingSeries(ctx, old.ID); err != nil {
		return nil, templateCareOfferingValidationError(
			ctx,
			"end template: validate linked care offerings",
			"ending the recurrence is incompatible with an existing care offering",
			err,
		)
	}

	templateID := old.ID
	// preserveDeviations=false: "end this and all following" must actually
	// remove the whole planned series — keeping a deviated row would leave the
	// ended series partly alive (#1840).
	deleted, err := s.deps.InstanceRepo.DeletePlannedNonSpontaneousInWindow(ctx, in.EffectiveDate, nil, &templateID, false)
	if err != nil {
		return nil, &ScheduleError{Op: "end template: delete planned instances", Err: err}
	}

	// Ending a series deletes planned instances outside the CRUD broadcast
	// paths — invalidate the staffing caches (#1844).
	if deleted > 0 {
		broadcastStaffingChanged(ctx, s.deps.Broadcaster, s.getLogger(), "template_end")
	}

	s.getLogger().Info("template ended from date",
		slog.Int64("tenant_id", tenantID),
		slog.Int64("template_id", old.ID),
		slog.String("effective_date", in.EffectiveDate.String()),
		slog.Int64("deleted_instances", deleted),
		slog.Int64("capped_schedules", cappedSchedules),
		slog.Int64("capped_enrollments", cappedEnrollments),
		slog.Int64("capped_supervisors", cappedSupervisors),
	)

	return &TemplateEndResult{
		TemplateID:        old.ID,
		EffectiveDate:     in.EffectiveDate,
		DeletedInstances:  int(deleted),
		CappedSchedules:   cappedSchedules,
		CappedEnrollments: cappedEnrollments,
		CappedSupervisors: cappedSupervisors,
	}, nil
}

// validateSplitInput checks the semantic rules the handler's Bind cannot:
// dates, weekday range, week pattern, time order, activity type.
func validateSplitInput(in *TemplateSplitInput) error {
	if err := validateSplitTemplateFields(*in); err != nil {
		return err
	}
	if err := validateSplitRecurrence(*in); err != nil {
		return err
	}
	if err := validateSplitWeekdayAssignments(*in); err != nil {
		return err
	}
	if err := validateSplitTargetGroup(in); err != nil {
		return err
	}
	return validateSplitMaterializationWindow(*in)
}

func validateSplitWeekdayAssignments(in TemplateSplitInput) error {
	if len(in.WeekdayAssignments) == 0 {
		return nil
	}
	if in.StudentIDs == nil || in.StaffIDs == nil {
		return fmt.Errorf("%w: weekday assignments require explicit student and staff rosters", ErrSplitInvalidInput)
	}
	if _, err := indexWeekdayAssignments(in.Weekdays, in.WeekdayAssignments); err != nil {
		return fmt.Errorf("%w: %s", ErrSplitInvalidInput, err.Error())
	}
	return nil
}

func validateSplitTemplateFields(in TemplateSplitInput) error {
	if in.TemplateID <= 0 {
		return fmt.Errorf("%w: template id is required", ErrSplitInvalidInput)
	}
	if in.Name == "" {
		return fmt.Errorf("%w: name is required", ErrSplitInvalidInput)
	}
	switch in.Type {
	case activitiesModel.GroupTypeCare, activitiesModel.GroupTypeActivity, activitiesModel.GroupTypeExternal:
	default:
		return fmt.Errorf("%w: invalid type %q (must be care, activity, or external)", ErrSplitInvalidInput, in.Type)
	}
	if in.RoomID <= 0 {
		return fmt.Errorf("%w: room_id is required", ErrSplitInvalidInput)
	}
	if in.CategoryID <= 0 {
		return fmt.Errorf("%w: category_id is required", ErrSplitInvalidInput)
	}
	if err := validateTemplateGradeLevelMax(in.GradeLevelMax); err != nil {
		return fmt.Errorf("%w: %s", ErrSplitInvalidInput, err.Error())
	}
	return nil
}

func validateSplitRecurrence(in TemplateSplitInput) error {
	if len(in.Weekdays) == 0 {
		return fmt.Errorf("%w: at least one weekday is required", ErrSplitInvalidInput)
	}
	for _, w := range in.Weekdays {
		if !activitiesModel.IsValidWeekday(w) {
			return fmt.Errorf("%w: invalid weekday %d (must be 1=Mon … 7=Sun)", ErrSplitInvalidInput, w)
		}
		if w > activitiesModel.WeekdayFriday {
			return fmt.Errorf("%w: timetable templates can only be scheduled from Monday to Friday", ErrSplitInvalidInput)
		}
	}
	if !in.EndTime.After(in.StartTime) {
		return fmt.Errorf("%w: end_time must be after start_time", ErrSplitInvalidInput)
	}
	if wp := in.WeekPattern; wp != nil && (*wp < 0 || *wp > 2) {
		return fmt.Errorf("%w: week_pattern must be 0 (every), 1 (A), or 2 (B)", ErrSplitInvalidInput)
	}
	if in.EffectiveDate.IsZero() {
		return fmt.Errorf("%w: effective_date is required", ErrSplitInvalidInput)
	}
	if in.EffectiveDate.Before(timezone.TodayDate()) {
		return fmt.Errorf("%w: effective_date must not be in the past", ErrSplitInvalidInput)
	}
	return nil
}

func validateSplitTargetGroup(in *TemplateSplitInput) error {
	targets, err := normalizeDynamicTargets(
		in.TargetGroupType,
		in.TargetGradeLevel,
		in.TargetSchoolClass,
		in.EducationGroupID,
		in.Targets,
	)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSplitInvalidInput, err)
	}
	in.Targets = targets
	in.TargetGradeLevel = nil
	in.TargetSchoolClass = nil
	if len(targets) > 0 {
		in.TargetGradeLevel = targets[0].TargetGradeLevel
		in.TargetSchoolClass = targets[0].TargetSchoolClass
		if in.TargetGroupType == activitiesModel.TargetGroupTypeGruppe {
			in.EducationGroupID = targets[0].EducationGroupID
		}
	}
	target := &activitiesModel.Group{
		TargetGroupType:   in.TargetGroupType,
		TargetGradeLevel:  in.TargetGradeLevel,
		TargetSchoolClass: in.TargetSchoolClass,
		EducationGroupID:  in.EducationGroupID,
	}
	if err := target.ValidateTargetGroup(); err != nil {
		return fmt.Errorf("%w: %s", ErrSplitInvalidInput, err.Error())
	}
	in.TargetGroupType = target.TargetGroupType
	in.TargetSchoolClass = target.TargetSchoolClass
	return nil
}

func validateTemplateEndInput(in TemplateEndInput) error {
	if in.TemplateID <= 0 {
		return fmt.Errorf("%w: template id is required", ErrSplitInvalidInput)
	}
	if in.EffectiveDate.IsZero() {
		return fmt.Errorf("%w: effective_date is required", ErrSplitInvalidInput)
	}
	if in.EffectiveDate.Before(timezone.TodayDate()) {
		return fmt.Errorf("%w: effective_date must not be in the past", ErrSplitInvalidInput)
	}
	return nil
}

// validateSplitMaterializationWindow rejects self-inconsistent materialization
// windows up front so they surface as 400 instead of bubbling out of the
// materialization service as 500s:
//
//   - materialize_to before effective_date (window entirely before the split)
//   - materialize_from after materialize_to (inverted window)
//   - clamped span over MaxMaterializationWindowDays (would trip the
//     materializer's own "window exceeds" guard mid-split)
//
// The span check uses the same clamp materializeWindow applies (from never
// before the effective date), so a wide materialize_from that gets clamped
// into range still passes.
func validateSplitMaterializationWindow(in TemplateSplitInput) error {
	if in.MaterializeTo == nil {
		return nil
	}
	if in.MaterializeTo.Before(in.EffectiveDate) {
		return fmt.Errorf("%w: materialize_to must not be before effective_date", ErrSplitInvalidInput)
	}
	if in.MaterializeFrom == nil {
		return nil
	}
	if in.MaterializeFrom.After(*in.MaterializeTo) {
		return fmt.Errorf("%w: materialize_from must not be after materialize_to", ErrSplitInvalidInput)
	}
	clampedFrom := *in.MaterializeFrom
	if clampedFrom.Before(in.EffectiveDate) {
		clampedFrom = in.EffectiveDate
	}
	if clampedFrom.DaysUntil(*in.MaterializeTo)+1 > MaxMaterializationWindowDays {
		return fmt.Errorf("%w: materialization window exceeds %d days", ErrSplitInvalidInput, MaxMaterializationWindowDays)
	}
	return nil
}

// loadTemplate resolves the old template and enforces the split preconditions:
// it must exist in the current tenant, be a template, and not be archived.
func (s *TemplateSplitService) loadTemplate(ctx context.Context, id int64) (*activitiesModel.Group, error) {
	group, err := s.deps.GroupRepo.FindByID(ctx, id)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, ErrSplitTemplateNotFound
		}
		return nil, &ScheduleError{Op: "split template: load template", Err: err}
	}
	if group == nil || !group.IsTemplate || group.ArchivedAt != nil {
		return nil, ErrSplitTemplateNotFound
	}
	return group, nil
}

// normalizeEffectiveDateInSegment enforces the editable segment interval
// [valid_from, valid_until). The check runs after acquiring the tenant
// recurrence gate, so a concurrent PUT/split/end cannot change the envelope
// between this read and the following cap. Split rejects a date before the
// segment start. End clamps it to valid_from, allowing the UI's default
// "today" to delete an entire future successor without inverting its rows.
// Both operations reject dates at or after the exclusive end because those
// dates do not belong to this segment and could create a second successor at
// an existing split boundary.
func (s *TemplateSplitService) normalizeEffectiveDateInSegment(
	ctx context.Context,
	templateID int64,
	effectiveDate timezone.Date,
	op string,
	clampBeforeStart bool,
) (timezone.Date, *timezone.Date, error) {
	schedules, err := s.deps.ScheduleRepo.FindByGroupID(ctx, templateID)
	if err != nil {
		return timezone.Date{}, nil, &ScheduleError{Op: op + ": load schedule envelope", Err: err}
	}
	validFrom, validUntil, err := commonScheduleValidityEnvelope(schedules)
	if err != nil {
		return timezone.Date{}, nil, &ScheduleError{Op: op + ": inspect schedule envelope", Err: err}
	}
	if validFrom != nil && effectiveDate.Before(*validFrom) {
		if clampBeforeStart {
			effectiveDate = *validFrom
		} else {
			return timezone.Date{}, nil, fmt.Errorf(
				"%w: effective_date %s is before segment valid_from %s",
				ErrSplitInvalidInput,
				effectiveDate.String(),
				validFrom.String(),
			)
		}
	}
	if validUntil != nil && !effectiveDate.Before(*validUntil) {
		return timezone.Date{}, nil, fmt.Errorf(
			"%w: effective_date %s must be before segment valid_until %s",
			ErrSplitInvalidInput,
			effectiveDate.String(),
			validUntil.String(),
		)
	}
	return effectiveDate, cloneOptionalDate(validUntil), nil
}

// loadSegmentRosterCandidates returns editor-owned enrollment and supervisor
// rows whose validity overlaps the segment tail. This includes future-starting
// open rows (their start is preserved) and plain bounded rows whose end equals
// the source segment end when a predecessor is split a second time. Sourced,
// weekday-specific, and unrelated bounded windows stay with their owning
// workflow. Since migration 1.15.52 the partial
// unique indexes are period-scoped — (tenant, person, group,
// COALESCE(calendar_period_id, 0)) WHERE valid_until IS NULL — so a person
// can have SEVERAL active rows on the same group, one per calendar period.
// Carry-over must therefore dedupe per person before stamping the successor's
// calendar_period_id, or the insert violates the index (see
// createStudentRoster / createStaffRoster).
func (s *TemplateSplitService) loadSegmentRosterCandidates(
	ctx context.Context,
	groupID int64,
	effectiveDate timezone.Date,
	segmentValidUntil *timezone.Date,
) ([]*activitiesModel.StudentEnrollment, []*activitiesModel.SupervisorPlanned, error) {
	enrollments, err := s.deps.EnrollmentRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return nil, nil, &ScheduleError{Op: "split template: load enrollments", Err: err}
	}
	supervisors, err := s.deps.SupervisorRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return nil, nil, &ScheduleError{Op: "split template: load supervisors", Err: err}
	}

	activeEnrollments := make([]*activitiesModel.StudentEnrollment, 0, len(enrollments))
	for _, e := range enrollments {
		if e != nil && validityWindowsOverlap(e.ValidFrom, e.ValidUntil, effectiveDate, segmentValidUntil) &&
			(enrollmentIsProtected(e) || enrollmentBelongsToSegmentRoster(e, segmentValidUntil)) {
			activeEnrollments = append(activeEnrollments, e)
		}
	}
	activeSupervisors := make([]*activitiesModel.SupervisorPlanned, 0, len(supervisors))
	for _, sp := range supervisors {
		if sp != nil && validityWindowsOverlap(sp.ValidFrom, sp.ValidUntil, effectiveDate, segmentValidUntil) &&
			supervisorBelongsToSegmentRoster(sp, segmentValidUntil) {
			activeSupervisors = append(activeSupervisors, sp)
		}
	}
	return activeEnrollments, activeSupervisors, nil
}

func enrollmentBelongsToSegmentRoster(row *activitiesModel.StudentEnrollment, segmentValidUntil *timezone.Date) bool {
	if enrollmentIsProtected(row) {
		return false
	}
	if row.ValidUntil == nil {
		return true
	}
	return optionalDatesEqual(row.ValidUntil, segmentValidUntil)
}

func enrollmentIsProtected(row *activitiesModel.StudentEnrollment) bool {
	return row.EnrollmentRequestChildID != nil || len(row.SelectedWeekdays) > 0
}

func supervisorBelongsToSegmentRoster(row *activitiesModel.SupervisorPlanned, segmentValidUntil *timezone.Date) bool {
	return row.ValidUntil == nil || optionalDatesEqual(row.ValidUntil, segmentValidUntil)
}

func (s *TemplateSplitService) capBoundedSegmentEnrollments(
	ctx context.Context,
	rows []*activitiesModel.StudentEnrollment,
	segmentValidUntil *timezone.Date,
	capAt timezone.Date,
) (int64, error) {
	if segmentValidUntil == nil {
		return 0, nil
	}
	var changed int64
	for _, row := range rows {
		if enrollmentIsProtected(row) || row.ValidUntil == nil || !optionalDatesEqual(row.ValidUntil, segmentValidUntil) {
			continue
		}
		if !row.ValidFrom.Before(capAt) {
			if err := s.deps.EnrollmentRepo.Delete(ctx, row.ID); err != nil {
				return changed, &ScheduleError{Op: "cap segment: delete future enrollment", Err: err}
			}
		} else if err := s.deps.EnrollmentRepo.SetValidUntilByID(ctx, row.ID, capAt); err != nil {
			return changed, &ScheduleError{Op: "cap segment: close enrollment", Err: err}
		}
		changed++
	}
	return changed, nil
}

func (s *TemplateSplitService) capBoundedSegmentSupervisors(
	ctx context.Context,
	rows []*activitiesModel.SupervisorPlanned,
	segmentValidUntil *timezone.Date,
	capAt timezone.Date,
) (int64, error) {
	if segmentValidUntil == nil {
		return 0, nil
	}
	var changed int64
	for _, row := range rows {
		if row.ValidUntil == nil || !optionalDatesEqual(row.ValidUntil, segmentValidUntil) {
			continue
		}
		if !row.ValidFrom.Before(capAt) {
			if err := s.deps.SupervisorRepo.Delete(ctx, row.ID); err != nil {
				return changed, &ScheduleError{Op: "cap segment: delete future supervisor", Err: err}
			}
		} else if err := s.deps.SupervisorRepo.SetValidUntilByID(ctx, row.ID, capAt); err != nil {
			return changed, &ScheduleError{Op: "cap segment: close supervisor", Err: err}
		}
		changed++
	}
	return changed, nil
}

// createSuccessorGroup copies the old template and applies the updated fields.
// MaxParticipants falls back to the old template's value when not provided.
func (s *TemplateSplitService) createSuccessorGroup(ctx context.Context, old *activitiesModel.Group, in TemplateSplitInput, tenantID int64) (*activitiesModel.Group, error) {
	maxParticipants := old.MaxParticipants
	if in.MaxParticipants != nil && *in.MaxParticipants > 0 {
		maxParticipants = *in.MaxParticipants
	}
	// Personalbedarf override (#1839): only touch the successor's override when
	// the request actually carried the field. When provided, the (already
	// normalized) value is authoritative — a null CLEARS it (derive). When
	// omitted, inherit the source template's override.
	requiredStaff := old.RequiredStaff
	if in.RequiredStaffProvided {
		requiredStaff = in.RequiredStaff
	}
	// Wochennotiz (#1837 follow-up): same omitted-vs-null carry-over as the
	// Personalbedarf override — inherit the source note unless the request
	// explicitly carried one (a null then clears it).
	notes := old.Notes
	if in.NotesProvided {
		notes = in.Notes
	}
	// Listenart (#1565): same omitted-vs-null carry-over — inherit the source
	// template's list kind unless the request explicitly carried the field (a
	// null then clears it).
	listKind := old.ListKind
	if in.ListKindProvided {
		listKind = in.ListKind
	}
	seriesRootID := old.ID
	if old.SeriesRootID != nil {
		seriesRootID = *old.SeriesRootID
	}
	roomID := in.RoomID
	group := &activitiesModel.Group{
		Name:              in.Name,
		MaxParticipants:   maxParticipants,
		RequiredStaff:     requiredStaff,
		IsOpen:            old.IsOpen,
		CategoryID:        in.CategoryID,
		PlannedRoomID:     &roomID,
		CreatedBy:         old.CreatedBy,
		Type:              in.Type,
		EducationGroupID:  in.EducationGroupID,
		IsTemplate:        true,
		SeriesRootID:      &seriesRootID,
		CalendarPeriodID:  in.CalendarPeriodID,
		TargetGroupType:   in.TargetGroupType,
		TargetGradeLevel:  in.TargetGradeLevel,
		TargetSchoolClass: in.TargetSchoolClass,
		ListKind:          listKind,
		Notes:             notes,
	}
	group.SetTenantID(tenantID)
	if err := s.deps.GroupRepo.Create(ctx, group); err != nil {
		return nil, &ScheduleError{Op: "split template: create successor template", Err: err}
	}
	targetRepo, ok := s.deps.GroupRepo.(activitiesModel.GroupTargetRepository)
	if !ok && len(in.Targets) > 0 {
		return nil, &ScheduleError{Op: "split template: create successor targets", Err: errors.New("target repository is not configured")}
	}
	if ok {
		if err := targetRepo.ReplaceTargets(ctx, group.ID, in.Targets); err != nil {
			return nil, &ScheduleError{Op: "split template: create successor targets", Err: err}
		}
	}
	return group, nil
}

// createSuccessorSchedules creates one schedule row per weekday, starting at
// the effective date (valid_from inclusive) with an open end (valid_until
// NULL).
func (s *TemplateSplitService) createSuccessorSchedules(
	ctx context.Context,
	groupID, timeframeID int64,
	in TemplateSplitInput,
	tenantID int64,
	segmentValidUntil *timezone.Date,
) ([]int64, error) {
	weekPattern := 0
	if in.WeekPattern != nil {
		weekPattern = *in.WeekPattern
	}
	scheduleIDs := make([]int64, 0, len(in.Weekdays))
	for _, weekday := range in.Weekdays {
		tfID := timeframeID
		validFrom := in.EffectiveDate
		sched := &activitiesModel.Schedule{
			Weekday:          weekday,
			TimeframeID:      &tfID,
			ActivityGroupID:  groupID,
			WeekPattern:      weekPattern,
			CalendarPeriodID: in.CalendarPeriodID,
			ValidFrom:        &validFrom, // never materialize before the split point
			ValidUntil:       cloneOptionalDate(segmentValidUntil),
		}
		sched.SetTenantID(tenantID)
		if err := s.deps.ScheduleRepo.Create(ctx, sched); err != nil {
			return nil, &ScheduleError{Op: "split template: create successor schedule", Err: err}
		}
		scheduleIDs = append(scheduleIDs, sched.ID)
	}
	return scheduleIDs, nil
}

// createStudentRoster writes the successor's enrollments. Explicit StudentIDs
// replace the editor-owned manual roster; sourced or weekday-specific rows are
// carried independently because the editor cannot represent their provenance
// or per-weekday selection. Nil carries the manual roster too. Explicit rows
// start at the effective date; carried rows retain a later valid_from. Every
// row is clipped to the source segment's upper bound.
//
// The carry path dedupes per student: loadActiveRoster can return several
// active rows per student (one per calendar period since migration 1.15.52),
// and stamping them all with in.CalendarPeriodID would collide on
// idx_student_enrollments_active. The row matching in.CalendarPeriodID is
// preferred; selected_weekdays are unioned across the student's rows (an
// empty list means "all weekdays" and wins the union).
func (s *TemplateSplitService) createStudentRoster(
	ctx context.Context,
	groupID int64,
	in TemplateSplitInput,
	carried []*activitiesModel.StudentEnrollment,
	tenantID int64,
	segmentValidUntil *timezone.Date,
) error {
	manual, protected := partitionCarriedEnrollments(carried)
	if err := validateProtectedEnrollmentRebase(protected, in.CalendarPeriodID); err != nil {
		return &ScheduleError{Op: "split template: rebase protected enrollments", Err: err}
	}
	protectedCoverage := buildProtectedStudentCoverage(
		protected,
		in.Weekdays,
		func(row *activitiesModel.StudentEnrollment) bool {
			return rosterPeriodApplies(row.CalendarPeriodID, in.CalendarPeriodID) ||
				(in.CalendarPeriodID != nil && row.CalendarPeriodID != nil)
		},
	)
	rows := buildSuccessorStudentRows(in, manual, protectedCoverage, segmentValidUntil)
	if err := s.persistSuccessorStudentRows(ctx, groupID, in, rows, tenantID, segmentValidUntil); err != nil {
		return err
	}
	return s.persistProtectedStudentRows(ctx, groupID, in, protected, tenantID, segmentValidUntil)
}

func partitionCarriedEnrollments(
	carried []*activitiesModel.StudentEnrollment,
) (manual, protected []*activitiesModel.StudentEnrollment) {
	manual = make([]*activitiesModel.StudentEnrollment, 0, len(carried))
	protected = make([]*activitiesModel.StudentEnrollment, 0, len(carried))
	for _, row := range carried {
		if enrollmentIsProtected(row) {
			protected = append(protected, row)
		} else {
			manual = append(manual, row)
		}
	}
	return manual, protected
}

func validateProtectedEnrollmentRebase(
	rows []*activitiesModel.StudentEnrollment,
	targetPeriodID *int64,
) error {
	if targetPeriodID == nil {
		return nil
	}
	activeScopedByStudent := make(map[int64]int)
	for _, row := range rows {
		if row == nil || row.CalendarPeriodID == nil || row.ValidUntil != nil {
			continue
		}
		activeScopedByStudent[row.StudentID]++
		if activeScopedByStudent[row.StudentID] > 1 {
			return fmt.Errorf(
				"%w: student %d has multiple active protected period rows",
				ErrTemplateRosterRebaseConflict,
				row.StudentID,
			)
		}
	}
	return nil
}

func buildSuccessorStudentRows(
	in TemplateSplitInput,
	carried []*activitiesModel.StudentEnrollment,
	protectedCoverage map[int64]protectedStudentCoverage,
	segmentValidUntil *timezone.Date,
) []*activitiesModel.StudentEnrollment {
	if in.StudentIDs != nil {
		resolved, err := resolveTemplateRoster(in.Weekdays, in.StudentIDs, nil, nil, in.WeekdayAssignments)
		if err != nil {
			resolved = resolvedTemplateRoster{Students: sharedRosterRows(in.StudentIDs, nil)}
		}
		manualRows := excludeProtectedStudentWeekdays(resolved.Students, protectedCoverage)
		rows := make([]*activitiesModel.StudentEnrollment, 0, len(manualRows))
		for _, row := range manualRows {
			rows = append(rows, &activitiesModel.StudentEnrollment{
				StudentID: row.PersonID,
				Weekday:   weekdayScopePtr(row.Weekday),
			})
		}
		return rows
	}
	return carriedStudentRowsByPerson(carried, in.CalendarPeriodID, segmentValidUntil)
}

func carriedStudentRowsByPerson(
	carried []*activitiesModel.StudentEnrollment,
	periodID *int64,
	segmentValidUntil *timezone.Date,
) []*activitiesModel.StudentEnrollment {
	// Keyed by (student, weekday scope) — see carriedStaffRowsByPerson for why
	// the weekday must be part of the key (#2129).
	byStudent := make(map[rosterPersonScope][]*activitiesModel.StudentEnrollment, len(carried))
	order := make([]rosterPersonScope, 0, len(carried))
	for _, row := range carried {
		key := rosterPersonScope{PersonID: row.StudentID, Weekday: weekdayScopeKey(row.Weekday)}
		if _, seen := byStudent[key]; !seen {
			order = append(order, key)
		}
		byStudent[key] = append(byStudent[key], row)
	}
	rows := make([]*activitiesModel.StudentEnrollment, 0, len(order))
	for _, key := range order {
		group := byStudent[key]
		preferred := preferCarriedRow(group, periodID, func(row *activitiesModel.StudentEnrollment) *int64 { return row.CalendarPeriodID })
		rows = append(rows, &activitiesModel.StudentEnrollment{
			StudentID:        key.PersonID,
			ValidFrom:        preferred.ValidFrom,
			ValidUntil:       earliestOptionalDate(preferred.ValidUntil, segmentValidUntil),
			SelectedWeekdays: unionSelectedWeekdays(group, preferred),
			Weekday:          weekdayScopePtr(preferred.Weekday),
		})
	}
	return rows
}

func (s *TemplateSplitService) persistSuccessorStudentRows(
	ctx context.Context,
	groupID int64,
	in TemplateSplitInput,
	rows []*activitiesModel.StudentEnrollment,
	tenantID int64,
	segmentValidUntil *timezone.Date,
) error {
	for _, row := range rows {
		row.ActivityGroupID = groupID
		if row.ValidFrom.IsZero() || row.ValidFrom.Before(in.EffectiveDate) {
			row.ValidFrom = in.EffectiveDate
		}
		row.ValidUntil = earliestOptionalDate(row.ValidUntil, segmentValidUntil)
		row.CalendarPeriodID = in.CalendarPeriodID
		row.SetTenantID(tenantID)
		if err := s.deps.EnrollmentRepo.Create(ctx, row); err != nil {
			return &ScheduleError{Op: "split template: create enrollment", Err: err}
		}
	}
	return nil
}

func (s *TemplateSplitService) persistProtectedStudentRows(
	ctx context.Context,
	groupID int64,
	in TemplateSplitInput,
	rows []*activitiesModel.StudentEnrollment,
	tenantID int64,
	segmentValidUntil *timezone.Date,
) error {
	for _, source := range rows {
		validFrom := source.ValidFrom
		if validFrom.Before(in.EffectiveDate) {
			validFrom = in.EffectiveDate
		}
		row := &activitiesModel.StudentEnrollment{
			StudentID:                source.StudentID,
			ActivityGroupID:          groupID,
			ValidFrom:                validFrom,
			ValidUntil:               earliestOptionalDate(source.ValidUntil, segmentValidUntil),
			CalendarPeriodID:         protectedEnrollmentPeriodAfterRebase(source.CalendarPeriodID, in.CalendarPeriodID),
			EnrollmentRequestChildID: cloneOptionalInt64(source.EnrollmentRequestChildID),
			SelectedWeekdays:         append([]int(nil), source.SelectedWeekdays...),
			Weekday:                  weekdayScopePtr(source.Weekday),
		}
		row.SetTenantID(tenantID)
		if err := s.deps.EnrollmentRepo.Create(ctx, row); err != nil {
			return &ScheduleError{Op: "split template: carry protected enrollment", Err: err}
		}
	}
	return nil
}

func protectedEnrollmentPeriodAfterRebase(sourcePeriodID, targetPeriodID *int64) *int64 {
	// Unscoped protected rows already apply to every selected period. Scoped
	// rows must follow an explicit successor period so materialization does not
	// silently filter the child out after an A→B edit.
	if sourcePeriodID != nil && targetPeriodID != nil {
		return cloneOptionalInt64(targetPeriodID)
	}
	return cloneOptionalInt64(sourcePeriodID)
}

// createStaffRoster writes the successor's supervisors. Explicit StaffIDs win
// (primary derived from PrimaryStaffID); nil carries over the previously-
// active roster with each row's is_primary flag preserved.
//
// Like createStudentRoster, the carry path dedupes per staff member: several
// active rows per staff (one per calendar period, migration 1.15.52) collapse
// to one successor row, preferring the row matching in.CalendarPeriodID. The
// preferred row's is_primary flag is kept.
func (s *TemplateSplitService) createStaffRoster(
	ctx context.Context,
	groupID int64,
	in TemplateSplitInput,
	carried []*activitiesModel.SupervisorPlanned,
	tenantID int64,
	segmentValidUntil *timezone.Date,
) error {
	rows := buildSuccessorStaffRows(in, carried, segmentValidUntil)
	for _, row := range rows {
		row.GroupID = groupID
		if row.ValidFrom.IsZero() || row.ValidFrom.Before(in.EffectiveDate) {
			row.ValidFrom = in.EffectiveDate
		}
		row.ValidUntil = earliestOptionalDate(row.ValidUntil, segmentValidUntil)
		row.CalendarPeriodID = in.CalendarPeriodID
		row.SetTenantID(tenantID)
		if err := s.deps.SupervisorRepo.Create(ctx, row); err != nil {
			return &ScheduleError{Op: "split template: create supervisor", Err: err}
		}
	}
	return nil
}

func buildSuccessorStaffRows(
	in TemplateSplitInput,
	carried []*activitiesModel.SupervisorPlanned,
	segmentValidUntil *timezone.Date,
) []*activitiesModel.SupervisorPlanned {
	if in.StaffIDs != nil {
		resolved, err := resolveTemplateRoster(in.Weekdays, nil, in.StaffIDs, in.PrimaryStaffID, in.WeekdayAssignments)
		if err != nil {
			// The handler validates the assignments before reaching the split,
			// so an error here means a caller bypassed that check; fall back to
			// the shared roster rather than dropping the staff entirely.
			resolved = resolvedTemplateRoster{Staff: sharedRosterRows(in.StaffIDs, in.PrimaryStaffID)}
		}
		rows := make([]*activitiesModel.SupervisorPlanned, 0, len(resolved.Staff))
		for _, row := range resolved.Staff {
			rows = append(rows, &activitiesModel.SupervisorPlanned{
				StaffID:   row.PersonID,
				IsPrimary: row.IsPrimary,
				Weekday:   weekdayScopePtr(row.Weekday),
			})
		}
		return rows
	}
	return carriedStaffRowsByPerson(carried, in.CalendarPeriodID, segmentValidUntil)
}

func carriedStaffRowsByPerson(
	carried []*activitiesModel.SupervisorPlanned,
	periodID *int64,
	segmentValidUntil *timezone.Date,
) []*activitiesModel.SupervisorPlanned {
	// Keyed by (staff, weekday scope), not by staff alone: collapsing a
	// per-weekday roster (#2129) to one row per person would turn "Anna
	// montags, Bea dienstags" into "Anna and Bea every day" on the successor.
	byStaff := make(map[rosterPersonScope][]*activitiesModel.SupervisorPlanned, len(carried))
	order := make([]rosterPersonScope, 0, len(carried))
	for _, row := range carried {
		key := rosterPersonScope{PersonID: row.StaffID, Weekday: weekdayScopeKey(row.Weekday)}
		if _, seen := byStaff[key]; !seen {
			order = append(order, key)
		}
		byStaff[key] = append(byStaff[key], row)
	}
	rows := make([]*activitiesModel.SupervisorPlanned, 0, len(order))
	for _, key := range order {
		preferred := preferCarriedRow(byStaff[key], periodID, func(row *activitiesModel.SupervisorPlanned) *int64 { return row.CalendarPeriodID })
		rows = append(rows, &activitiesModel.SupervisorPlanned{
			StaffID:    key.PersonID,
			IsPrimary:  preferred.IsPrimary,
			ValidFrom:  preferred.ValidFrom,
			ValidUntil: earliestOptionalDate(preferred.ValidUntil, segmentValidUntil),
			Weekday:    weekdayScopePtr(preferred.Weekday),
		})
	}
	return rows
}

// rosterPersonScope keys a carried roster row by person AND weekday scope.
type rosterPersonScope struct {
	PersonID int64
	// Weekday is 0 for the series-wide scope (a real ISO weekday is 1..7),
	// which makes the struct usable as a map key without a pointer.
	Weekday int
}

func weekdayScopeKey(weekday *int) int {
	if weekday == nil {
		return 0
	}
	return *weekday
}

// preferCarriedRow picks the carried roster row whose calendar_period_id
// matches the successor's target period (both nil counts as a match); when
// none matches it falls back to the first row. Generic over the two roster
// row types so the per-person selection rule lives in one place.
func preferCarriedRow[T any](rows []T, targetPeriodID *int64, periodOf func(T) *int64) T {
	for _, row := range rows {
		p := periodOf(row)
		if (p == nil && targetPeriodID == nil) ||
			(p != nil && targetPeriodID != nil && *p == *targetPeriodID) {
			return row
		}
	}
	return rows[0]
}

func earliestOptionalDate(left, right *timezone.Date) *timezone.Date {
	if left == nil {
		return cloneOptionalDate(right)
	}
	if right == nil || left.Before(*right) {
		return cloneOptionalDate(left)
	}
	return cloneOptionalDate(right)
}

func cloneOptionalInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

// unionSelectedWeekdays merges selected_weekdays across one student's carried
// rows. An empty/nil list means "all weekdays" and dominates the union; the
// result preserves ascending weekday order. With a single row this returns
// the preferred row's list unchanged.
func unionSelectedWeekdays(rows []*activitiesModel.StudentEnrollment, preferred *activitiesModel.StudentEnrollment) []int {
	if len(rows) == 1 {
		return preferred.SelectedWeekdays
	}
	seen := make(map[int]struct{})
	for _, e := range rows {
		if len(e.SelectedWeekdays) == 0 {
			return nil // "all weekdays" wins the union
		}
		for _, wd := range e.SelectedWeekdays {
			seen[wd] = struct{}{}
		}
	}
	out := make([]int, 0, len(seen))
	for wd := activitiesModel.WeekdayMonday; wd <= activitiesModel.WeekdaySunday; wd++ {
		if _, ok := seen[wd]; ok {
			out = append(out, wd)
		}
	}
	return out
}

// materializeWindow re-materializes the requested window, clamped so it never
// starts before the effective date. Requires both bounds; an inverted clamped
// window is a silent no-op (nothing to materialize before the split point).
func (s *TemplateSplitService) materializeWindow(ctx context.Context, in TemplateSplitInput) (*MaterializationResult, error) {
	from, to, ok := splitMaterializationWindow(in)
	if !ok {
		return nil, nil
	}
	mat, err := s.deps.Materialization.MaterializeForTenant(ctx, from, to, MaterializationSourceManual)
	if err != nil {
		return nil, &ScheduleError{Op: "split template: materialize", Err: err}
	}
	return mat, nil
}

// splitMaterializationWindow returns the exact inclusive window shared by
// deviation preservation and materialization. It clamps the lower bound to the
// split date because the predecessor remains authoritative before that date.
func splitMaterializationWindow(in TemplateSplitInput) (timezone.Date, timezone.Date, bool) {
	if in.MaterializeFrom == nil || in.MaterializeTo == nil {
		return timezone.Date{}, timezone.Date{}, false
	}
	from := *in.MaterializeFrom
	if from.Before(in.EffectiveDate) {
		from = in.EffectiveDate
	}
	to := *in.MaterializeTo
	if from.After(to) {
		return timezone.Date{}, timezone.Date{}, false
	}
	return from, to, true
}

// FindOrCreateTimeframe returns the id of an existing schedule.timeframes row
// matching the [start, end] clock window or inserts a fresh one. Description
// is set to descHint on first creation as a debug hint, but is informational
// only — lookups go by time window. Reusing existing timeframes keeps the
// schedule.timeframes table from growing one row per template — common slots
// (12:00–12:50) end up shared across templates.
//
// Shared by the POST /templates create handler, the PUT /templates/{id}
// update handler and the template split service so the find-or-create rule
// lives in exactly one place.
func FindOrCreateTimeframe(ctx context.Context, repo scheduleModel.TimeframeRepository, start, end time.Time, descHint string) (int64, error) {
	existing, err := repo.FindByTimeRange(ctx, start, end)
	if err == nil {
		for _, tf := range existing {
			if tf == nil {
				continue
			}
			// Match exact clock times; FindByTimeRange may return overlapping
			// windows depending on impl, so be precise. Do not use
			// time.Time.Equal here: schedule.timeframes stores SQL TIME, and
			// drivers may decode TIME with a different date anchor than the
			// caller's HH:MM parser uses.
			if timezone.SameClockTime(tf.StartTime, start) && tf.EndTime != nil && timezone.SameClockTime(*tf.EndTime, end) {
				return tf.ID, nil
			}
		}
	}

	endCopy := end
	tf := &scheduleModel.Timeframe{
		StartTime:   start,
		EndTime:     &endCopy,
		IsActive:    true,
		Description: fmt.Sprintf("auto: %s", descHint),
	}
	tf.SetTenantID(tenant.FromContext(ctx))
	if err := repo.Create(ctx, tf); err != nil {
		return 0, fmt.Errorf("create timeframe: %w", err)
	}
	return tf.ID, nil
}
