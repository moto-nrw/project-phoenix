package timetableplanning

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	auditModel "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// validateAssignableCategory rejects missing, cross-tenant, and archived
// categories before a timetable write creates a new reference. The repository
// read is tenant-scoped, so all three cases share the same client-facing error.
func validateAssignableCategory(
	ctx context.Context,
	repo activitiesModel.CategoryRepository,
	categoryID int64,
	op string,
) error {
	category, err := repo.FindByIDForShare(ctx, categoryID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return &ScheduleError{Op: op, Err: timetableModule.ErrCategoryNotAssignable}
		}
		return &ScheduleError{Op: op, Err: err}
	}
	if category == nil || category.IsArchived() {
		return &ScheduleError{Op: op, Err: timetableModule.ErrCategoryNotAssignable}
	}
	return nil
}

// TemplateEducationGroupError marks an education_group_id precheck failure so
// handlers can surface it as a 400 while preserving the precise message. It is
// shared by the create, update, and split template flows.
type TemplateEducationGroupError struct{ Err error }

func (e *TemplateEducationGroupError) Error() string { return e.Err.Error() }

func (e *TemplateEducationGroupError) Unwrap() error { return e.Err }

// ValidateTemplateEducationGroup confirms an optional education_group_id refers
// to a positive id that belongs to the caller's tenant. A nil pointer means no
// group is linked and validates trivially. All failures are wrapped in
// TemplateEducationGroupError so callers render them uniformly as 400.
func (s *TemplateService) ValidateTemplateEducationGroup(ctx context.Context, groupID *int64) error {
	if groupID == nil {
		return nil
	}
	if *groupID <= 0 {
		return &TemplateEducationGroupError{Err: errors.New("education_group_id must be positive when set")}
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return &TemplateEducationGroupError{Err: errors.New("no tenant in context")}
	}
	exists, err := s.deps.EducationGroupRepo.Exists(ctx, *groupID)
	if err != nil {
		return &TemplateEducationGroupError{Err: fmt.Errorf("validate education_group_id: %w", err)}
	}
	if !exists {
		return &TemplateEducationGroupError{Err: errors.New("education_group_id does not reference a group in this tenant")}
	}
	return nil
}

// FindOrCreateTimeframe returns the id of an existing schedule.timeframes row
// matching [start, end] or inserts a fresh one. Description is set to the
// template name on first creation as a debug hint, but is informational only —
// lookups go by time window. Shared by the template create and update paths so
// the find-or-create rule lives in exactly one place.
func (s *TemplateService) FindOrCreateTimeframe(ctx context.Context, start, end time.Time, descHint string) (int64, error) {
	existing, err := s.deps.TimeframeRepo.FindByTimeRange(ctx, start, end)
	if err == nil {
		for _, tf := range existing {
			if tf == nil {
				continue
			}
			// Match exact clock times; FindByTimeRange may return overlapping
			// windows depending on impl, so be precise. Do not use
			// time.Time.Equal here: schedule.timeframes stores SQL TIME, and
			// drivers may decode TIME with a different date anchor than the
			// handler's parseClockTime uses.
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
	if err := s.deps.TimeframeRepo.Create(ctx, tf); err != nil {
		return 0, fmt.Errorf("create timeframe: %w", err)
	}
	return tf.ID, nil
}

// PlanningTrackAssignments is the slice of the Timetable owner's
// planning-track administration (timetable.PlanningTrackAdministration) the
// template writes check a track assignment against.
type PlanningTrackAssignments interface {
	ValidatePlanningTrackAssignment(ctx context.Context, id, allowedArchivedID *int64) error
}

// validateAssignablePlanningTrack rejects a missing or archived track unless
// it is the one the template already carries. A composition without the
// administration can assign no track at all.
func validateAssignablePlanningTrack(ctx context.Context, tracks PlanningTrackAssignments, id, allowedArchivedID *int64) error {
	if id == nil {
		return nil
	}
	if tracks == nil {
		return timetableModule.ErrPlanningTrackNotFound
	}
	return tracks.ValidatePlanningTrackAssignment(ctx, id, allowedArchivedID)
}

func samePlanningTrackID(left, right *int64) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

// TemplateServiceDependencies wires the template writes and the attendance
// correction of the retained planning services (#3424 slices S2 and S3).
type TemplateServiceDependencies struct {
	InstanceStudentRepo    scheduleModel.InstanceStudentRepository
	ActivityInstanceRepo   scheduleModel.ActivityInstanceRepository
	ActivityScheduleRepo   activitiesModel.ScheduleRepository
	InstanceStaffRepo      scheduleModel.InstanceStaffRepository
	ActivityCategoryRepo   activitiesModel.CategoryRepository
	PlanningTracks         PlanningTrackAssignments
	ActivityGroupRepo      activitiesModel.GroupRepository
	ActivitySupervisorRepo activitiesModel.SupervisorPlannedRepository
	StudentEnrollmentRepo  activitiesModel.StudentEnrollmentRepository
	TimeframeRepo          scheduleModel.TimeframeRepository
	EducationGroupRepo     educationModel.GroupRepository
	// ValidateCareOfferingSeries rejects archival when a care offering still
	// depends on the template being live. Production always wires it; partial
	// read-only test facades may leave it nil.
	ValidateCareOfferingSeries func(context.Context, int64) error
	// ResyncOfferingRoster reconciles an offering-sourced template's roster
	// with the offering's approved enrollments (#2137). Implemented by the
	// enrollment decision service (injected to avoid the enrollment→schedule
	// import cycle); production always wires it, tests that never save a
	// sourced template may leave it nil.
	ResyncOfferingRoster func(context.Context, OfferingRosterResyncInput) error
	// ValidateOfferingSource resolves the offering-source references before
	// the template row carrying them is written (#2147 review round 18):
	// existence, active flag, phase/period compatibility, and — with several
	// sources — that all offerings share one enrollment phase. storedOfferingIDs
	// are the template's persisted ids (nil on create). Production always
	// wires it (the care-offering service); test facades may leave it nil.
	ValidateOfferingSource func(ctx context.Context, offeringIDs, storedOfferingIDs []int64, calendarPeriodID *int64) error
	// AttendanceCorrectionRepo records corrections to a child's attendance in
	// an instance (#2898). Optional — nil disables the trail, which is only
	// acceptable in read-only test facades; production always wires it.
	AttendanceCorrectionRepo auditModel.AttendanceCorrectionRepository
	// PersonRepo snapshots the acting person's name onto a correction so the
	// trail survives a later account deletion. Optional.
	PersonRepo usersModel.PersonRepository
	// RecoveryRepo serializes attendance writes with instance completion.
	// Production always wires it; unit-test facades may leave it nil.
	RecoveryRepo scheduleModel.ActivityRecoveryRepository
	// ConflictDetection and TimetableData are the Timetable owner's conflict
	// detection (#3550) and planner reads (#3551), carried only for the
	// composition root: services.Factory may not grow a field for them. The
	// handles go with this package.
	ConflictDetection timetableModule.ConflictDetectionCapability
	TimetableData     timetableModule.TimetableDataCapability
	// Broadcaster invalidates planner and "Heute geplant" caches after
	// template-side changes that bypass the instance CRUD flows.
	Broadcaster realtime.Broadcaster
	Logger      *slog.Logger
	DB          *bun.DB
	Today       func() timezone.Date
}

// TemplateService holds the template writes (create, update, archive, the
// series roster) and the attendance correction until #3424 slices S2 and S3
// move them to the Timetable owner.
type TemplateService struct {
	deps TemplateServiceDependencies
}

// NewTemplateService creates the retained template writes.
func NewTemplateService(deps TemplateServiceDependencies) *TemplateService {
	if deps.Today == nil {
		deps.Today = timezone.TodayDate
	}
	return &TemplateService{deps: deps}
}

func (s *TemplateService) getLogger() *slog.Logger {
	return cmp.Or(s.deps.Logger, slog.Default())
}

// ConflictDetection hands the composition root the Timetable owner's
// conflict detection this service was built with (#3550).
func (s *TemplateService) ConflictDetection() timetableModule.ConflictDetectionCapability {
	return s.deps.ConflictDetection
}

// TimetableData hands the composition root the Timetable owner's planner
// reads this service was built with (#3551).
func (s *TemplateService) TimetableData() timetableModule.TimetableDataCapability {
	return s.deps.TimetableData
}

// lockInstanceAttendance takes FOR UPDATE on every participant row so a
// concurrent Complete cannot flip the instance between the status check and
// the attendance write. No-op without the recovery repository.
func (s *TemplateService) lockInstanceAttendance(ctx context.Context, instanceID int64) error {
	if s.deps.RecoveryRepo == nil {
		return nil
	}
	return s.deps.RecoveryRepo.LockAttendance(ctx, instanceID)
}
