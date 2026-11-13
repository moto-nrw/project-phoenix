package application

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	scheduleModel "github.com/moto-ogs/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-ogs/project-phoenix/modules/timetable/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

const replanTemplateOp = "update template: re-plan occurrences"

// templateReplan is the re-plan a template update runs in its own
// transaction (#6838): the materializer only inserts, keyed by group, date
// and start time, so a changed series must delete its planned occurrences
// and materialize them again in the same unit. A re-plan left to the client
// after the commit kept stale and duplicate occurrences when it failed.
type templateReplan struct {
	from, to calendar.Date
	// instances are the template's planned occurrences in [from, to] before
	// the update; their days and people are locked with the update's.
	instances []*scheduleModel.ActivityInstance
}

// planTemplateReplan resolves the window [today, horizon]. The horizon is
// the later of the planning period's end and the template's last planned
// occurrence, so no stale occurrence outlives the edit, also after a period
// move. Nil means nothing to re-plan.
func (s *TemplateService) planTemplateReplan(ctx context.Context, in TemplateUpdateInput, previousPeriodID *int64) (*templateReplan, error) {
	if !in.ReplanFuture {
		return nil, nil
	}
	if s.deps.Materialization == nil || s.deps.Deviations == nil || s.deps.Periods == nil {
		return nil, &ports.ScheduleError{Op: replanTemplateOp, Err: errors.New("re-plan is not configured")}
	}
	today := s.deps.Today()
	from, spontaneous := today.String(), false
	instances, err := listLegacyInstances(ctx, s.deps.ActivityInstanceRepo.Reads, ports.LegacyInstanceFilter{
		ActivityGroupID: &in.TemplateID, FromDate: &from, Status: scheduleModel.InstanceStatusPlanned, IsSpontaneous: &spontaneous,
	}, replanTemplateOp+": load occurrences")
	if err != nil {
		return nil, err
	}
	horizon, err := s.replanPeriodEnd(ctx, in.CalendarPeriodID, previousPeriodID)
	if err != nil {
		return nil, err
	}
	for _, inst := range instances {
		if day := calendar.Date(inst.Date); day.After(horizon) {
			horizon = day
		}
	}
	if horizon.IsZero() || horizon.Before(today) {
		return nil, nil
	}
	return &templateReplan{from: today, to: horizon, instances: instances}, nil
}

// replanPeriodEnd is the end of the pinned planning period, else the latest
// end of the active periods, which the materializer then chooses from.
func (s *TemplateService) replanPeriodEnd(ctx context.Context, periodID, previousPeriodID *int64) (calendar.Date, error) {
	if periodID == nil {
		periodID = previousPeriodID
	}
	if periodID != nil {
		period, err := s.deps.Periods.FindByID(ctx, *periodID)
		if err != nil {
			return "", &ports.ScheduleError{Op: replanTemplateOp + ": load period", Err: err}
		}
		if period == nil {
			return "", nil
		}
		return period.EndDate, nil
	}
	periods, err := s.deps.Periods.FindActiveByTenantID(ctx)
	if err != nil {
		return "", &ports.ScheduleError{Op: replanTemplateOp + ": load active periods", Err: err}
	}
	var end calendar.Date
	for _, period := range periods {
		if period != nil && (end.IsZero() || period.EndDate.After(end)) {
			end = period.EndDate
		}
	}
	return end, nil
}

// days lists every day of the window: the re-plan deletes and materializes
// occurrences on any of them, so it holds each day like ReplanWeek does.
func (r *templateReplan) days() []calendar.Date {
	if r == nil {
		return nil
	}
	days := make([]calendar.Date, 0, r.from.DaysUntil(r.to)+1)
	for day := r.from; !day.After(r.to); day = day.AddDays(1) {
		days = append(days, day)
	}
	return days
}

// replanChunk is one materialization run of the re-plan.
type replanChunk struct{ from, to calendar.Date }

// chunks splits the window into materialization runs of at most
// MaxMaterializationWindowDays days.
func (r *templateReplan) chunks() []replanChunk {
	var chunks []replanChunk
	for from := r.from; !from.After(r.to); {
		to := from.AddDays(timetable.MaxMaterializationWindowDays - 1)
		if to.After(r.to) {
			to = r.to
		}
		chunks = append(chunks, replanChunk{from: from, to: to})
		from = to.AddDays(1)
	}
	return chunks
}

// replanStaff lists the people the re-plan writes: the staff of the deleted
// occurrences and the live supervisors the materialization plans across all
// templates. The update locks them with its roster in one ascending batch
// before its first write (#6583, #6702).
func (s *TemplateService) replanStaff(ctx context.Context, replan *templateReplan) ([]int64, error) {
	if replan == nil {
		return nil, nil
	}
	rows, err := staffOfInstances(ctx, s.deps.InstanceStaffRepo, instanceRowIDs(replan.instances))
	if err != nil {
		return nil, err
	}
	staff := make([]int64, 0, len(rows))
	for _, row := range rows {
		staff = append(staff, row.StaffID)
	}
	// Without the roster the materialization would lock its people after
	// the update's writes, against the lock order; refuse like ReplanWeek.
	roster, ok := s.deps.Materialization.(materializationStaffRoster)
	if !ok {
		return nil, &ports.ScheduleError{Op: replanTemplateOp, Err: errors.New("materialization provides no staff roster")}
	}
	var candidates []int64
	for _, chunk := range replan.chunks() {
		ids, err := roster.staffForMaterialization(ctx, chunk.from, chunk.to)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, ids...)
	}
	if len(candidates) == 0 {
		return staff, nil
	}
	live, err := s.deps.Directories.StaffPersons(ctx, candidates, false)
	if err != nil {
		return nil, &ports.ScheduleError{Op: replanTemplateOp + ": load materialization staff", Err: err}
	}
	for id := range live {
		staff = append(staff, id)
	}
	return staff, nil
}

// replanTemplateOccurrences snapshots the template's deviations in the
// window, deletes its planned occurrences, materializes the window again and
// reapplies the deviations, the same steps as a template-scoped ReplanWeek.
func (s *TemplateService) replanTemplateOccurrences(ctx context.Context, in TemplateUpdateInput, replan *templateReplan) error {
	if replan == nil {
		return nil
	}
	preserved, err := s.deps.Deviations.SnapshotDeviations(ctx, replan.from, replan.to, in.TemplateID)
	if err != nil {
		return &ports.ScheduleError{Op: replanTemplateOp + ": snapshot deviations", Err: err}
	}
	deleted, err := deletePlannedInstancesInWindow(ctx, s.deps.ActivityInstanceRepo.Plans, replan.from, &replan.to, &in.TemplateID, false)
	if err != nil {
		return &ports.ScheduleError{Op: replanTemplateOp + ": delete planned", Err: err}
	}
	created := 0
	for _, chunk := range replan.chunks() {
		mat, err := s.deps.Materialization.MaterializeForTenant(ctx, chunk.from, chunk.to, timetable.MaterializationSourceManual)
		if err != nil {
			return &ports.ScheduleError{Op: replanTemplateOp + ": materialize", Err: err}
		}
		if mat != nil {
			created += mat.InstancesCreated
		}
	}
	reapplied, err := preserved.Reapply(ctx, in.TemplateID, in.ActorAccountID)
	if err != nil {
		return &ports.ScheduleError{Op: replanTemplateOp + ": reapply deviations", Err: err}
	}
	s.getLogger().Info("template update re-planned occurrences",
		slog.Int64("template_id", in.TemplateID),
		slog.String("from", replan.from.String()),
		slog.String("to", replan.to.String()),
		slog.Int64("deleted_instances", deleted),
		slog.Int("instances_created", created),
		slog.Int("deviations_snapshotted", preserved.Count()),
		slog.Int("deviations_reapplied", reapplied),
	)
	return nil
}
