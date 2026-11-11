package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type catalogTimetableBatch interface {
	CatalogSnapshot(context.Context) (careplan.CatalogTimetableSnapshot, error)
}
type catalogCalendarBatch interface {
	CatalogPeriods(context.Context) ([]careplan.LinkedPeriod, error)
}
type catalogPhaseBatch interface {
	CatalogPhases(context.Context, []int64) ([]careplan.OfferingPhase, error)
}

// validateTraegerBatch uses the existing save validators against an immutable
// final catalog and bulk-loaded foreign inputs. No validator issues row reads.
func (c *CareOfferingCatalog) validateTraegerBatch(ctx context.Context, batch traegerOfferingBatch) error {
	if len(batch.writes) == 0 {
		return nil
	}
	validator := *c
	validator.deps.Records = catalogBatchRecords{CatalogRecords: c.deps.Records, rows: batch.final}
	validator.deps.LockTemplateRecurrence = nil // the caller already holds the gate
	grade, err := c.deps.Settings.GradeLevelMax(ctx)
	if err != nil {
		return err
	}
	validator.deps.Settings = catalogBatchSettings{grade: grade}
	if validator.deps, err = loadTraegerBatchLinks(ctx, validator.deps, batch.writes, c.todayDate()); err != nil {
		return err
	}
	phases := map[int64][]careplan.CareOffering{}
	for _, row := range batch.final {
		phases[row.PhaseID] = append(phases[row.PhaseID], row)
	}
	touched := map[int64]bool{}
	for index, write := range batch.writes {
		row := write.Offering
		if err = validator.prepareWrite(ctx, &row); err != nil {
			return err
		}
		batch.writes[index].Offering = row
		touched[row.PhaseID] = true
	}
	for phase := range touched {
		if err = validateCatalogGroupRuleConsistency(phases[phase]); err != nil {
			return err
		}
	}
	return nil
}

func loadTraegerBatchLinks(ctx context.Context, deps CareOfferingCatalogDependencies, writes []careplan.TraegerOfferingWrite, today calendar.Date) (CareOfferingCatalogDependencies, error) {
	phases := []int64{}
	inactive := []int64{}
	seen := map[int64]bool{}
	for _, write := range writes {
		row := write.Offering
		if row.ActivityGroupID == nil {
			continue
		}
		if !seen[row.PhaseID] {
			phases = append(phases, row.PhaseID)
			seen[row.PhaseID] = true
		}
		if !row.IsActive && row.ID > 0 {
			inactive = append(inactive, row.ID)
		}
	}
	if len(phases) == 0 {
		return deps, nil
	}
	timetable, ok := deps.Timetable.(catalogTimetableBatch)
	if !ok {
		return deps, errors.New("catalog batch: timetable snapshot is required")
	}
	snapshot, err := timetable.CatalogSnapshot(ctx)
	if err != nil {
		return deps, err
	}
	deps.Timetable = catalogBatchTimetable{snapshot: snapshot}
	periods, ok := deps.Calendar.(catalogCalendarBatch)
	if !ok {
		return deps, errors.New("catalog batch: calendar snapshot is required")
	}
	periodRows, err := periods.CatalogPeriods(ctx)
	if err != nil {
		return deps, err
	}
	deps.Calendar = catalogBatchCalendar{CatalogCalendar: deps.Calendar, rows: periodRows}
	phaseQuery, ok := deps.Phases.(catalogPhaseBatch)
	if !ok {
		return deps, errors.New("catalog batch: phase snapshot is required")
	}
	phaseRows, err := phaseQuery.CatalogPhases(ctx, phases)
	if err != nil {
		return deps, err
	}
	deps.Phases = catalogBatchPhases{rows: phaseRows}
	return loadTraegerBatchCounts(ctx, deps, inactive, today)
}

func loadTraegerBatchCounts(ctx context.Context, deps CareOfferingCatalogDependencies, inactive []int64, today calendar.Date) (CareOfferingCatalogDependencies, error) {
	if len(inactive) > 0 {
		reads, ok := deps.Bookings.(traegerBatchBookings)
		if !ok {
			return deps, errors.New("catalog batch: materializable counts are required")
		}
		counts, err := reads.CatalogMaterializable(ctx, inactive, today)
		if err != nil {
			return deps, err
		}
		deps.Bookings = catalogBatchBookings{CatalogBookings: deps.Bookings, counts: counts}
	}
	return deps, nil
}

type catalogBatchRecords struct {
	ports.CatalogRecords
	rows []careplan.CareOffering
}

func (r catalogBatchRecords) ListCareOfferings(_ context.Context, filter careplan.CareOfferingFilter) ([]careplan.CareOffering, error) {
	result := []careplan.CareOffering{}
	for _, row := range r.rows {
		include := filter.PhaseIDs == nil
		for _, phase := range filter.PhaseIDs {
			if row.PhaseID == phase {
				include = true
			}
		}
		if include {
			result = append(result, row)
		}
	}
	return result, nil
}

type catalogBatchSettings struct{ grade int }

func (s catalogBatchSettings) GradeLevelMax(context.Context) (int, error) { return s.grade, nil }

type catalogBatchPhases struct{ rows []careplan.OfferingPhase }

func (p catalogBatchPhases) Phase(_ context.Context, id int64) (careplan.OfferingPhase, error) {
	for _, row := range p.rows {
		if row.ID == id {
			return row, nil
		}
	}
	return careplan.OfferingPhase{}, ports.ErrCatalogRowNotFound
}

type catalogBatchCalendar struct {
	ports.CatalogCalendar
	rows []careplan.LinkedPeriod
}

func (c catalogBatchCalendar) FindPeriod(_ context.Context, id int64) (careplan.LinkedPeriod, error) {
	for _, row := range c.rows {
		if row.ID == id {
			return row, nil
		}
	}
	return careplan.LinkedPeriod{}, ports.ErrCatalogRowNotFound
}

type catalogBatchBookings struct {
	ports.CatalogBookings
	counts map[int64]int
}

func (b catalogBatchBookings) MaterializableOfferingCount(_ context.Context, id int64, _ calendar.Date) (int, error) {
	return b.counts[id], nil
}

type catalogBatchTimetable struct {
	snapshot careplan.CatalogTimetableSnapshot
}

func (t catalogBatchTimetable) FindGroup(_ context.Context, id int64) (careplan.LinkedGroup, error) {
	for _, row := range t.snapshot.Groups {
		if row.ID == id {
			return row, nil
		}
	}
	return careplan.LinkedGroup{}, ports.ErrCatalogRowNotFound
}
func (t catalogBatchTimetable) TemplateSeries(ctx context.Context, id int64) ([]careplan.LinkedGroup, error) {
	group, err := t.FindGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	root := group.ID
	if group.SeriesRootID != nil {
		root = *group.SeriesRootID
	}
	result := []careplan.LinkedGroup{}
	for _, row := range t.snapshot.Groups {
		series := row.ID
		if row.SeriesRootID != nil {
			series = *row.SeriesRootID
		}
		if series == root && row.IsTemplate && !row.Archived {
			result = append(result, row)
		}
	}
	return result, nil
}
func (t catalogBatchTimetable) GroupSchedules(_ context.Context, ids []int64) ([]careplan.LinkedSchedule, error) {
	selected := map[int64]bool{}
	for _, id := range ids {
		selected[id] = true
	}
	result := []careplan.LinkedSchedule{}
	for _, row := range t.snapshot.Schedules {
		if selected[row.GroupID] {
			result = append(result, row)
		}
	}
	return result, nil
}
func (t catalogBatchTimetable) Timeframes(context.Context) ([]careplan.LinkedTimeframe, error) {
	return t.snapshot.Timeframes, nil
}
func (t catalogBatchTimetable) ExceptionsBetween(_ context.Context, from, until calendar.Date) ([]careplan.LinkedException, error) {
	result := []careplan.LinkedException{}
	for _, row := range t.snapshot.Exceptions {
		if !row.Date.Before(from) && !row.Date.After(until) {
			result = append(result, row)
		}
	}
	return result, nil
}
func (t catalogBatchTimetable) GroupExceptions(_ context.Context, id int64) ([]careplan.LinkedException, error) {
	result := []careplan.LinkedException{}
	for _, row := range t.snapshot.Exceptions {
		if row.GroupID == id {
			result = append(result, row)
		}
	}
	return result, nil
}
