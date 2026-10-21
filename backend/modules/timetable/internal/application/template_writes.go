package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModel "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The template writes of the Timetable owner (#3424 slice S2) behind
// timetable.TemplateAdministration: create, update, archive, the split-series
// roster and, in template_split.go, split and end. They run over the retained
// repository rows. Collaborators the owner may not name arrive as
// consumer-owned ports the composition root binds: Care Plan's care-offering
// checks, Enrollment's roster resync, School Structure's education groups and
// class
// rules, and the realtime announcement of staffing changes.

// announceStaffingChanged queues the announcement until the surrounding
// tenant transaction commits, so subscribers that refetch see the new rows.
// A nil announcer announces nothing.
func announceStaffingChanged(ctx context.Context, announcer ports.StaffingAnnouncer, logger *slog.Logger, source string) {
	if announcer == nil {
		return
	}
	tenantID := tenant.FromContext(ctx)
	tenant.RegisterAfterCommit(ctx, func() {
		if err := announcer.AnnounceStaffingChanged(tenantID, source); err != nil {
			logger.Warn("SSE planned instance broadcast failed",
				slog.String("source", source),
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
		}
	})
}

func validateSchoolClassRules(r ports.SchoolClassRules) error {
	if r.Normalize == nil || r.MinGradeLevel <= 0 || r.MaxGradeLevel < r.MinGradeLevel {
		return errors.New("school class rules are not configured")
	}
	return nil
}

// careOfferingValidationError classifies a failed ValidateSeries. A
// conflict is a client-correctable 400; the ambient transaction is marked for
// rollback because TenantTxMiddleware commits ordinary 4xx responses. Lock or
// repository failures stay operational errors.
func careOfferingValidationError(ctx context.Context, c ports.CareOfferingChecks, op, action string, err error) error {
	tenant.MarkRollback(ctx)
	if c.IsConflict != nil && c.IsConflict(err) {
		return &ports.ScheduleError{
			Op: op,
			Err: fmt.Errorf(
				"%w: %w: %s: %w",
				timetable.ErrSplitInvalidInput,
				timetable.ErrTemplateCareOfferingConflict,
				action,
				err,
			),
		}
	}
	return &ports.ScheduleError{Op: op, Err: fmt.Errorf("validate linked care offerings: %w", err)}
}

// validateAssignableCategory rejects missing, cross-tenant, and archived
// categories before a timetable write creates a new reference. The repository
// read is tenant-scoped, so all three cases share the same client-facing error.
func validateAssignableCategory(
	ctx context.Context,
	categories timetable.CategoryQuery,
	categoryID int64,
	op string,
) error {
	category, err := categories.FindCategoryForShare(ctx, categoryID)
	if err != nil {
		if errors.Is(err, timetable.ErrCategoryNotFound) || errors.Is(err, timetable.ErrInvalidCategory) {
			return &ports.ScheduleError{Op: op, Err: timetable.ErrCategoryNotAssignable}
		}
		return &ports.ScheduleError{Op: op, Err: legacyDatabaseError("find category by id for share", err)}
	}
	if category.IsArchived() {
		return &ports.ScheduleError{Op: op, Err: timetable.ErrCategoryNotAssignable}
	}
	return nil
}

// ValidateTemplateEducationGroup confirms an optional education_group_id refers
// to a positive id that belongs to the caller's tenant. A nil pointer means no
// group is linked and validates trivially. All failures are wrapped in
// TemplateEducationGroupError so callers render them uniformly as 400.
func (s *TemplateService) ValidateTemplateEducationGroup(ctx context.Context, groupID *int64) error {
	if groupID == nil {
		return nil
	}
	if *groupID <= 0 {
		return &timetable.TemplateEducationGroupError{Err: errors.New("education_group_id must be positive when set")}
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return &timetable.TemplateEducationGroupError{Err: errors.New("no tenant in context")}
	}
	exists, err := s.deps.EducationGroups.Exists(ctx, *groupID)
	if err != nil {
		return &timetable.TemplateEducationGroupError{Err: fmt.Errorf("validate education_group_id: %w", err)}
	}
	if !exists {
		return &timetable.TemplateEducationGroupError{Err: errors.New("education_group_id does not reference a group in this tenant")}
	}
	return nil
}

// FindOrCreateTimeframe returns the id of an existing schedule.timeframes row
// matching [start, end] or inserts a fresh one. Shared by the template create
// and update paths so the find-or-create rule lives in exactly one place.
func (s *TemplateService) FindOrCreateTimeframe(ctx context.Context, start, end time.Time, descHint string) (int64, error) {
	return findOrCreateTimeframe(ctx, s.deps.TimeframeRepo, start, end, descHint)
}

// findOrCreateTimeframe returns the id of an existing schedule.timeframes row
// matching the [start, end] clock window or inserts a fresh one. Description
// is set to descHint on first creation as a debug hint, but is informational
// only — lookups go by time window. Reusing existing timeframes keeps the
// schedule.timeframes table from growing one row per template — common slots
// (12:00–12:50) end up shared across templates.
func findOrCreateTimeframe(ctx context.Context, timeframes timetable.TimeframeCapability, start, end time.Time, descHint string) (int64, error) {
	from, until := PublicClock(start), PublicClock(end)
	listed, err := timeframes.ListTimeframes(ctx, timetable.TimeframeFilter{OverlapsStart: &from, OverlapsEnd: &until})
	var existing []*scheduleModel.Timeframe
	if err == nil {
		existing, err = legacyTimeframes(listed)
	}
	if err == nil {
		for _, tf := range existing {
			// Match exact clock times; the overlap read returns every
			// overlapping window, so be precise. Do not use time.Time.Equal
			// here: schedule.timeframes stores SQL TIME, and drivers may
			// decode TIME with a different date anchor than the caller's
			// HH:MM parser uses.
			if calendar.SameClockTime(tf.StartTime, start) && tf.EndTime != nil && calendar.SameClockTime(*tf.EndTime, end) {
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
	if err := tf.Validate(); err != nil {
		return 0, fmt.Errorf("create timeframe: %w", err)
	}
	created, err := timeframes.CreateTimeframe(ctx, publicTimeframeInput(tf))
	if err != nil {
		return 0, fmt.Errorf("create timeframe: %w", WrapDatabaseError("create", err))
	}
	return created.ID, nil
}

// validateAssignablePlanningTrack rejects a missing or archived track unless
// it is the one the template already carries. A composition without the
// administration can assign no track at all.
func validateAssignablePlanningTrack(ctx context.Context, tracks ports.PlanningTrackAssignments, id, allowedArchivedID *int64) error {
	if id == nil {
		return nil
	}
	if tracks == nil {
		return timetable.ErrPlanningTrackNotFound
	}
	return tracks.ValidatePlanningTrackAssignment(ctx, id, allowedArchivedID)
}

func samePlanningTrackID(left, right *int64) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

// TemplateServiceDependencies wires the template create, update and archive
// writes and the split-series roster. The repositories, RecurrenceLock and
// DB are required for writes; read-only test facades may leave the rest
// nil. ResyncOfferingRoster reconciles an offering-sourced template's roster
// with the offering's approved enrollments (#2137) and is Enrollment's,
// bound late at the root.
type TemplateServiceDependencies struct {
	InstanceStudentRepo    ports.InstanceStudentRecords
	ActivityInstanceRepo   ports.ActivityInstanceRecords
	ActivityScheduleRepo   timetable.ScheduleCapability
	InstanceStaffRepo      ports.InstanceStaffRecords
	ActivityCategoryRepo   timetable.CategoryQuery
	PlanningTracks         ports.PlanningTrackAssignments
	ActivityGroupRepo      ports.GroupRows
	ActivitySupervisorRepo ports.PlannedSupervisorRecords
	StudentEnrollmentRepo  ports.ActivityEnrollmentRecords
	TimeframeRepo          timetable.TimeframeCapability
	EducationGroups        ports.EducationGroupDirectory
	CareOfferings          ports.CareOfferingChecks
	ResyncOfferingRoster   func(context.Context, timetable.OfferingRosterResyncInput) error
	RecurrenceLock         timetable.RecurrenceWriteLock
	SchoolClasses          ports.SchoolClassRules
	Staffing               ports.StaffingAnnouncer
	Logger                 *slog.Logger
	Transactions           ports.TenantTransactions
	Today                  func() calendar.Date
}

// TemplateService holds the template writes (create, update, archive, the
// series roster).
type TemplateService struct {
	deps TemplateServiceDependencies
}

// NewTemplateService creates the template writes.
func NewTemplateService(deps TemplateServiceDependencies) *TemplateService {
	if deps.Today == nil {
		deps.Today = calendar.TodayDate
	}
	return &TemplateService{deps: deps}
}

func (s *TemplateService) getLogger() *slog.Logger {
	return OrDefaultLogger(s.deps.Logger)
}

// lockRecurrence takes the tenant recurrence gate for a template write.
func (s *TemplateService) lockRecurrence(ctx context.Context, op string) error {
	if s.deps.RecurrenceLock == nil {
		return &ports.ScheduleError{Op: op + ": lock recurrence", Err: errors.New("template recurrence lock is not configured")}
	}
	if err := s.deps.RecurrenceLock.LockRecurrenceWrites(ctx); err != nil {
		return &ports.ScheduleError{Op: op + ": lock recurrence", Err: err}
	}
	return nil
}

// newRosterReconciler builds the roster maintenance the template writes use
// to align already-materialized occurrences.
func (s *TemplateService) newRosterReconciler() *RosterReconciler {
	return NewRosterReconciler(s.deps.ActivityInstanceRepo, s.deps.InstanceStudentRepo, s.deps.StudentEnrollmentRepo, s.deps.Logger)
}

// OrDefaultLogger falls back to slog.Default for services a test builds
// without a logger.
func OrDefaultLogger(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return slog.Default()
	}
	return logger
}
