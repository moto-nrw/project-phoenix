package timetableplanning

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

var ErrInstanceAlreadyInSeries = errors.New("activity instance already belongs to a series")

// ConvertInstanceToSeriesInput turns one existing planned occurrence into the
// seed of a new recurring template. Template contains the complete series
// definition; InstanceNotes is the per-occurrence note that stays on the seed.
type ConvertInstanceToSeriesInput struct {
	InstanceID     int64
	Template       timetable.CreateTemplateCommand
	InstanceNotes  *string
	ActorAccountID *int64
}

// ConvertInstanceToSeriesResult identifies both sides of a successful atomic
// conversion. LinkedInstanceID is always the pre-existing occurrence ID.
type ConvertInstanceToSeriesResult struct {
	TemplateID       int64
	TimeframeID      int64
	ScheduleIDs      []int64
	LinkedInstanceID int64
}

// InstanceSeriesConverter is the single write seam used by the timetable API
// when a planner changes a one-off occurrence into a recurring series.
type InstanceSeriesConverter interface {
	ConvertInstanceToSeries(context.Context, ConvertInstanceToSeriesInput) (*ConvertInstanceToSeriesResult, error)
}

// TemplateSeriesSeeding is the slice of the Timetable owner's template
// writes (timetable.TemplateAdministration) the conversion seeds a series
// with.
type TemplateSeriesSeeding interface {
	CreateTemplate(ctx context.Context, cmd timetable.CreateTemplateCommand) (*timetable.CreateTemplateResult, error)
	TemplateAssignmentsOn(ctx context.Context, templateID int64, date timezone.Date, calendarPeriodID int64) (timetable.TemplateAssignments, error)
	AlignPlannedInstanceStaff(ctx context.Context, templateID int64, staffIDs []int64, from timezone.Date) error
}

// InstanceSeriesConversionDependencies are the existing modules the
// conversion coordinates. The converter adds transactionality and ordering;
// it does not duplicate their persistence rules.
type InstanceSeriesConversionDependencies struct {
	DB              *bun.DB
	InstanceRepo    scheduleModel.ActivityInstanceRepository
	InstanceService InstanceService
	Templates       TemplateSeriesSeeding
	RecurrenceLock  timetable.RecurrenceWriteLock
}

type instanceSeriesConversionService struct {
	deps InstanceSeriesConversionDependencies
}

func NewInstanceSeriesConversionService(deps InstanceSeriesConversionDependencies) InstanceSeriesConverter {
	if deps.DB == nil || deps.InstanceRepo == nil || deps.InstanceService == nil || deps.Templates == nil || deps.RecurrenceLock == nil {
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
		if err := s.deps.RecurrenceLock.LockRecurrenceWrites(txCtx); err != nil {
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

		created, err := s.deps.Templates.CreateTemplate(txCtx, in.Template)
		if err != nil {
			return err
		}
		date := *in.Template.ScheduleValidFrom
		assignments, err := s.deps.Templates.TemplateAssignmentsOn(
			txCtx, created.TemplateID, date, *in.Template.CalendarPeriodID,
		)
		if err != nil {
			return err
		}

		templateID := created.TemplateID
		// Stamp calendar_period_id with the same materializer marker so the
		// seed is visible to FindPlannedTemplateBackedFrom (reconcile, offering
		// resync, primary-staff alignment) and period-scoped roster predicates.
		_, err = s.deps.InstanceService.UpdatePlanned(txCtx, in.InstanceID, UpdateInstanceInput{
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
		}, in.ActorAccountID)
		if err != nil {
			return err
		}

		// UpdatePlanned preserves occurrence-specific staff metadata. Align the
		// still-planned rows with the template once more so a newly selected
		// Hauptbetreuung receives is_primary on the converted seed as well.
		if err := s.deps.Templates.AlignPlannedInstanceStaff(txCtx, templateID, assignments.StaffIDs, date); err != nil {
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

// seriesDeviationMachinery is the instance service's deviation snapshot and
// reapply machinery (#1840) a template split preserves Vertretungsplan
// overrides with.
type seriesDeviationMachinery interface {
	acquireSubstituteDayLocks(context.Context, int64, timezone.Date, timezone.Date) error
	snapshotDeviations(context.Context, timezone.Date, timezone.Date, *int64) ([]deviationSnapshot, map[groupDay]int, error)
	reapplyDeviations(context.Context, []deviationSnapshot, map[groupDay]int, *int64, *int64) (int, error)
}

// SeriesDeviationPreserver exposes that machinery to the Timetable owner's
// template split (#3424 slice S2), which the composition root binds to it.
type SeriesDeviationPreserver struct {
	machinery seriesDeviationMachinery
}

// NewSeriesDeviationPreserver narrows the instance service to its deviation
// machinery.
func NewSeriesDeviationPreserver(svc InstanceService) (*SeriesDeviationPreserver, error) {
	machinery, ok := svc.(seriesDeviationMachinery)
	if !ok {
		return nil, errors.New("instance service does not support deviation preservation")
	}
	return &SeriesDeviationPreserver{machinery: machinery}, nil
}

// LockDeviationDays takes the day-wide substitute/deviation lock of every
// date in [from, to], in ascending order.
func (p *SeriesDeviationPreserver) LockDeviationDays(ctx context.Context, tenantID int64, from, to timezone.Date) error {
	return p.machinery.acquireSubstituteDayLocks(ctx, tenantID, from, to)
}

// SnapshotDeviations snapshots the template's deviations in [from, to].
func (p *SeriesDeviationPreserver) SnapshotDeviations(ctx context.Context, from, to timezone.Date, templateID int64) (*SeriesDeviationSnapshot, error) {
	id := templateID
	snapshots, occurrences, err := p.machinery.snapshotDeviations(ctx, from, to, &id)
	if err != nil {
		return nil, err
	}
	return &SeriesDeviationSnapshot{machinery: p.machinery, snapshots: snapshots, occurrences: occurrences}, nil
}

// SeriesDeviationSnapshot is one snapshot of SnapshotDeviations.
type SeriesDeviationSnapshot struct {
	machinery   seriesDeviationMachinery
	snapshots   []deviationSnapshot
	occurrences map[groupDay]int
}

// Count is the number of snapshotted deviations.
func (s *SeriesDeviationSnapshot) Count() int { return len(s.snapshots) }

// Reapply writes the snapshot onto the given template's occurrences.
func (s *SeriesDeviationSnapshot) Reapply(ctx context.Context, templateID int64, actorAccountID *int64) (int, error) {
	id := templateID
	return s.machinery.reapplyDeviations(ctx, s.snapshots, s.occurrences, &id, actorAccountID)
}
