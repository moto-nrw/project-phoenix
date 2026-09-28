package compose

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// TemplateSeriesSeeding is the slice of the template writes the series
// conversion seeds a series with.
type TemplateSeriesSeeding interface {
	CreateTemplate(ctx context.Context, cmd timetable.CreateTemplateCommand) (*timetable.CreateTemplateResult, error)
	TemplateAssignmentsOn(ctx context.Context, templateID int64, date timezone.Date, calendarPeriodID int64) (timetable.TemplateAssignments, error)
	AlignPlannedInstanceStaff(ctx context.Context, templateID int64, staffIDs []int64, from timezone.Date) error
}

// InstanceSeriesConversionDependencies are the capabilities the conversion
// coordinates; it adds transactionality and ordering, not persistence rules.
type InstanceSeriesConversionDependencies struct {
	DB             *bun.DB
	InstanceRepo   scheduleModel.ActivityInstanceRepository
	Lifecycle      timetable.InstancePlanning
	Templates      TemplateSeriesSeeding
	RecurrenceLock timetable.RecurrenceWriteLock
}

type instanceSeriesConversion struct {
	deps InstanceSeriesConversionDependencies
}

// NewInstanceSeriesConversion composes the conversion of a one-off
// occurrence into a series.
func NewInstanceSeriesConversion(deps InstanceSeriesConversionDependencies) (timetable.InstanceSeriesConversion, error) {
	if deps.DB == nil || deps.InstanceRepo == nil || deps.Lifecycle == nil || deps.Templates == nil || deps.RecurrenceLock == nil {
		return nil, errors.New("timetable instance series conversion: required dependency is nil")
	}
	return &instanceSeriesConversion{deps: deps}, nil
}

// ConvertInstanceToSeries creates the template and links the pre-existing
// occurrence in one tenant transaction. The recurrence lock serializes it
// with template writes and materialization; a second concurrent conversion
// observes the committed link and cannot create another template.
func (s *instanceSeriesConversion) ConvertInstanceToSeries(ctx context.Context, in timetable.ConvertInstanceToSeriesInput) (*timetable.ConvertInstanceToSeriesResult, error) {
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
	var result timetable.ConvertInstanceToSeriesResult
	err := tenant.WithTenantTx(ctx, s.deps.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		result, err = s.convertLocked(txCtx, in)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *instanceSeriesConversion) convertLocked(ctx context.Context, in timetable.ConvertInstanceToSeriesInput) (timetable.ConvertInstanceToSeriesResult, error) {
	var none timetable.ConvertInstanceToSeriesResult
	if err := s.deps.RecurrenceLock.LockRecurrenceWrites(ctx); err != nil {
		return none, &ScheduleError{Op: "convert instance to series: lock recurrence", Err: err}
	}
	instance, err := s.convertibleInstance(ctx, in.InstanceID)
	if err != nil {
		return none, err
	}
	created, err := s.deps.Templates.CreateTemplate(ctx, in.Template)
	if err != nil {
		return none, err
	}
	date := *in.Template.ScheduleValidFrom
	assignments, err := s.deps.Templates.TemplateAssignmentsOn(ctx, created.TemplateID, date, *in.Template.CalendarPeriodID)
	if err != nil {
		return none, err
	}
	templateID := created.TemplateID
	// The calendar period stamps the materializer marker, so the seed is
	// visible to the template-backed reads (reconcile, offering resync,
	// primary-staff alignment) and the period-scoped roster predicates.
	if _, err := s.deps.Lifecycle.UpdatePlanned(ctx, in.InstanceID, timetable.UpdateInstanceInput{
		Date:             date,
		StartTime:        in.Template.StartTime,
		EndTime:          in.Template.EndTime,
		Title:            in.Template.Name,
		Description:      instance.Description,
		Notes:            in.InstanceNotes,
		RoomID:           in.Template.RoomID,
		ActivityGroupID:  &templateID,
		CalendarPeriodID: in.Template.CalendarPeriodID,
		ListKind:         in.Template.ListKind,
		StaffIDs:         assignments.StaffIDs,
		StudentIDs:       assignments.StudentIDs,
		RequiredStaff:    nil,
	}, in.ActorAccountID); err != nil {
		return none, err
	}
	// The edit keeps occurrence-specific staff metadata; align the planned
	// rows with the template once more so a newly chosen Hauptbetreuung is
	// primary on the seed as well.
	if err := s.deps.Templates.AlignPlannedInstanceStaff(ctx, templateID, assignments.StaffIDs, date); err != nil {
		return none, err
	}
	return timetable.ConvertInstanceToSeriesResult{
		TemplateID:       created.TemplateID,
		TimeframeID:      created.TimeframeID,
		ScheduleIDs:      created.ScheduleIDs,
		LinkedInstanceID: in.InstanceID,
	}, nil
}

func (s *instanceSeriesConversion) convertibleInstance(ctx context.Context, instanceID int64) (*scheduleModel.ActivityInstance, error) {
	instance, err := s.deps.InstanceRepo.FindByID(ctx, instanceID)
	if errors.Is(err, sql.ErrNoRows) || instance == nil {
		return nil, timetable.ErrInstanceNotFound
	}
	if err != nil {
		return nil, &ScheduleError{Op: "convert instance to series: load instance", Err: err}
	}
	if instance.Status != scheduleModel.InstanceStatusPlanned {
		return nil, fmt.Errorf("%w: cannot convert instance in status %q", timetable.ErrInvalidInstanceTransition, instance.Status)
	}
	if instance.ActivityGroupID != nil {
		return nil, timetable.ErrInstanceAlreadyInSeries
	}
	return instance, nil
}
