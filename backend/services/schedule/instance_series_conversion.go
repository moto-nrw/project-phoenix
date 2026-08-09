package schedule

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

var ErrInstanceAlreadyInSeries = errors.New("activity instance already belongs to a series")

// InstanceSeriesConversionDependencies are the three existing modules the
// conversion coordinates. The converter adds transactionality and ordering;
// it does not duplicate their persistence rules.
type InstanceSeriesConversionDependencies struct {
	DB              *bun.DB
	InstanceRepo    scheduleModel.ActivityInstanceRepository
	InstanceService InstanceService
	TimetableData   *TimetableDataService
}

type instanceSeriesConversionService struct {
	deps InstanceSeriesConversionDependencies
}

func NewInstanceSeriesConversionService(deps InstanceSeriesConversionDependencies) InstanceSeriesConverter {
	if deps.DB == nil || deps.InstanceRepo == nil || deps.InstanceService == nil || deps.TimetableData == nil {
		panic("schedule.NewInstanceSeriesConversionService: required dependency is nil")
	}
	return &instanceSeriesConversionService{deps: deps}
}

// ConvertInstanceToSeries creates the template and links the pre-existing
// occurrence in one tenant transaction. The recurrence lock serializes this
// path with template creates, updates, splits, and materialization. A second
// concurrent conversion observes the committed activity_group_id and cannot
// create another template.
func (s *instanceSeriesConversionService) ConvertInstanceToSeries(
	ctx context.Context,
	in ConvertInstanceToSeriesInput,
) (*ConvertInstanceToSeriesResult, error) {
	if in.InstanceID <= 0 {
		return nil, fmt.Errorf("convert instance to series: instance id must be positive")
	}
	if in.Template.ScheduleValidFrom == nil || in.Template.ScheduleValidFrom.IsZero() {
		return nil, fmt.Errorf("convert instance to series: start date is required")
	}
	if in.Template.CalendarPeriodID == nil || *in.Template.CalendarPeriodID <= 0 {
		return nil, fmt.Errorf("convert instance to series: calendar period is required")
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return nil, fmt.Errorf("convert instance to series: no tenant in context")
	}

	var result ConvertInstanceToSeriesResult
	err := tenant.WithTenantTx(ctx, s.deps.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		if err := lockTenantRecurrenceWrites(txCtx, s.deps.DB); err != nil {
			return &ScheduleError{Op: "convert instance to series: lock recurrence", Err: err}
		}

		instance, err := s.deps.InstanceRepo.FindByID(txCtx, in.InstanceID)
		if errors.Is(err, sql.ErrNoRows) || instance == nil {
			return ErrInstanceNotFound
		}
		if err != nil {
			return &ScheduleError{Op: "convert instance to series: load instance", Err: err}
		}
		if instance.Status != scheduleModel.InstanceStatusPlanned {
			return fmt.Errorf("%w: cannot convert instance in status %q", ErrInvalidInstanceTransition, instance.Status)
		}
		if instance.ActivityGroupID != nil {
			return ErrInstanceAlreadyInSeries
		}

		created, err := s.deps.TimetableData.CreateTemplate(txCtx, in.Template)
		if err != nil {
			return err
		}
		date := *in.Template.ScheduleValidFrom
		studentIDs, staffIDs, err := s.deps.TimetableData.templateAssignmentsOn(
			txCtx, created.TemplateID, date, *in.Template.CalendarPeriodID,
		)
		if err != nil {
			return err
		}

		templateID := created.TemplateID
		_, err = s.deps.InstanceService.UpdatePlanned(txCtx, in.InstanceID, UpdateInstanceInput{
			Date:            date,
			StartTime:       in.Template.StartTime,
			EndTime:         in.Template.EndTime,
			Title:           in.Template.Name,
			Description:     instance.Description,
			Notes:           in.InstanceNotes,
			RoomID:          in.Template.RoomID,
			ActivityGroupID: &templateID,
			ListKind:        in.Template.ListKind,
			StaffIDs:        staffIDs,
			StudentIDs:      studentIDs,
			RequiredStaff:   nil,
		}, in.ActorAccountID)
		if err != nil {
			return err
		}

		// UpdatePlanned preserves occurrence-specific staff metadata. Align the
		// still-planned rows with the template once more so a newly selected
		// Hauptbetreuung receives is_primary on the converted seed as well.
		if err := s.deps.TimetableData.reconcilePredecessorInstanceStaff(
			txCtx, templateID, staffIDs, date, nil,
		); err != nil {
			return err
		}

		result = ConvertInstanceToSeriesResult{
			TemplateID:       created.TemplateID,
			TimeframeID:      created.TimeframeID,
			ScheduleIDs:      created.ScheduleIDs,
			LinkedInstanceID: in.InstanceID,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// templateAssignmentsOn derives the same concrete people a fresh
// materialized occurrence would receive for one date. It intentionally reads
// the persisted template roster after CreateTemplate has completed, so
// offering-service start dates and selected weekdays are already authoritative.
func (s *TimetableDataService) templateAssignmentsOn(
	ctx context.Context,
	templateID int64,
	date timezone.Date,
	periodID int64,
) ([]int64, []int64, error) {
	if s.deps.StudentEnrollmentRepo == nil || s.deps.ActivitySupervisorRepo == nil || s.deps.ActivityGroupRepo == nil {
		return nil, nil, &ScheduleError{
			Op:  "derive template assignments: validate dependencies",
			Err: errors.New("required roster repository is nil"),
		}
	}

	enrollments, err := s.deps.StudentEnrollmentRepo.FindByGroupID(ctx, templateID)
	if err != nil {
		return nil, nil, &ScheduleError{Op: "derive template assignments: load enrollments", Err: err}
	}
	studentIDs := make([]int64, 0, len(enrollments))
	seenStudents := make(map[int64]struct{}, len(enrollments))
	for _, enrollment := range enrollments {
		if !isEnrollmentValidOn(enrollment, date, periodID) || enrollmentStudentIsAlumnus(enrollment) {
			continue
		}
		if _, exists := seenStudents[enrollment.StudentID]; exists {
			continue
		}
		seenStudents[enrollment.StudentID] = struct{}{}
		studentIDs = append(studentIDs, enrollment.StudentID)
	}

	if targetRepo, ok := s.deps.ActivityGroupRepo.(activitiesModel.GroupTargetRepository); ok {
		targetStudentIDs, err := targetRepo.FindTargetStudentIDs(ctx, templateID)
		if err != nil {
			return nil, nil, &ScheduleError{Op: "derive template assignments: load target students", Err: err}
		}
		for _, studentID := range targetStudentIDs {
			if studentID <= 0 {
				continue
			}
			if _, exists := seenStudents[studentID]; exists {
				continue
			}
			seenStudents[studentID] = struct{}{}
			studentIDs = append(studentIDs, studentID)
		}
	}

	supervisors, err := s.deps.ActivitySupervisorRepo.FindByGroupID(ctx, templateID)
	if err != nil {
		return nil, nil, &ScheduleError{Op: "derive template assignments: load supervisors", Err: err}
	}
	staffIDs := make([]int64, 0, len(supervisors))
	seenStaff := make(map[int64]struct{}, len(supervisors))
	for _, supervisor := range supervisors {
		if !isSupervisorValidOn(supervisor, date, periodID) {
			continue
		}
		if _, exists := seenStaff[supervisor.StaffID]; exists {
			continue
		}
		seenStaff[supervisor.StaffID] = struct{}{}
		staffIDs = append(staffIDs, supervisor.StaffID)
	}

	return studentIDs, staffIDs, nil
}
