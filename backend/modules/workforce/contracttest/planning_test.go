package contracttest_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	models "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/shiftplanning"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// Unused embedded methods deliberately panic: a contract test must specify
// every dependency operation the capability is allowed to perform.
type planningShifts struct {
	shiftplanning.StaffShiftService
	create func(context.Context, *models.StaffShift) (*models.StaffShift, error)
	update func(context.Context, *models.StaffShift, shiftplanning.StaffShiftUpdateOptions) (*models.StaffShift, error)
	move   func(context.Context, shiftplanning.MoveShiftInput) (*models.StaffShift, error)
	cancel func(context.Context, shiftplanning.CancelShiftInput) (*shiftplanning.CancelShiftResult, error)
	list   func(context.Context, int64, timezone.Date, timezone.Date) ([]*models.StaffShift, error)
	remove func(context.Context, int64) error
}

func (s planningShifts) CreateShift(c context.Context, v *models.StaffShift) (*models.StaffShift, error) {
	return s.create(c, v)
}
func (s planningShifts) UpdateShiftWithOptions(c context.Context, v *models.StaffShift, o shiftplanning.StaffShiftUpdateOptions) (*models.StaffShift, error) {
	return s.update(c, v, o)
}
func (s planningShifts) MoveShift(c context.Context, v shiftplanning.MoveShiftInput) (*models.StaffShift, error) {
	return s.move(c, v)
}
func (s planningShifts) ApplyCancellation(c context.Context, v shiftplanning.CancelShiftInput) (*shiftplanning.CancelShiftResult, error) {
	return s.cancel(c, v)
}
func (s planningShifts) ListShifts(c context.Context, a, b timezone.Date) ([]*models.StaffShift, error) {
	return s.list(c, 0, a, b)
}
func (s planningShifts) ListShiftsForStaff(c context.Context, id int64, a, b timezone.Date) ([]*models.StaffShift, error) {
	return s.list(c, id, a, b)
}
func (s planningShifts) DeleteShift(c context.Context, id int64) error { return s.remove(c, id) }

type planningSeries struct {
	shiftplanning.StaffShiftSeriesService
	create func(context.Context, *models.StaffShiftSeries) (*shiftplanning.SeriesResult, error)
	split  func(context.Context, shiftplanning.SplitSeriesInput) (*shiftplanning.SeriesResult, error)
	get    func(context.Context, int64) (*models.StaffShiftSeries, error)
	end    func(context.Context, int64, timezone.Date) (*shiftplanning.SeriesResult, error)
}

func (s planningSeries) CreateSeries(c context.Context, v *models.StaffShiftSeries) (*shiftplanning.SeriesResult, error) {
	return s.create(c, v)
}
func (s planningSeries) SplitSeries(c context.Context, v shiftplanning.SplitSeriesInput) (*shiftplanning.SeriesResult, error) {
	return s.split(c, v)
}
func (s planningSeries) GetSeries(c context.Context, id int64) (*models.StaffShiftSeries, error) {
	return s.get(c, id)
}
func (s planningSeries) EndSeries(c context.Context, id int64, d timezone.Date) (*shiftplanning.SeriesResult, error) {
	return s.end(c, id, d)
}

type planningOverview func(context.Context, timezone.Date, timezone.Date) (*shiftplanning.StaffScheduleOverview, error)

func (f planningOverview) GetOverview(c context.Context, a, b timezone.Date) (*shiftplanning.StaffScheduleOverview, error) {
	return f(c, a, b)
}

type planningExport struct {
	planexport.Service
	export func(context.Context, planexport.Params) (listexport.File, error)
}

func (f planningExport) ExportDienstplan(c context.Context, p planexport.Params) (listexport.File, error) {
	return f.export(c, p)
}
func planning(deps shiftplanning.PlanningDependencies) workforce.StaffShiftPlanning {
	if deps.Shifts == nil {
		deps.Shifts = planningShifts{}
	}
	if deps.Series == nil {
		deps.Series = planningSeries{}
	}
	if deps.Overview == nil {
		deps.Overview = planningOverview(nil)
	}
	if deps.PlanExport == nil {
		deps.PlanExport = planningExport{}
	}
	return shiftplanning.NewStaffShiftPlanning(deps)
}

func TestPlanningPreservesShiftEditsAndCancellation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	staff := testpkg.CreateTestStaff(t, db, "Planning", "Contract")
	ctx := testpkg.Ctx(t)
	input := workforce.StaffShiftInput{StaffID: staff.ID, ActorStaffID: staff.ID, Date: "2026-09-21", StartTime: "08:30:00", EndTime: "14:45:00", BreakMinutes: 30}
	var saved *models.StaffShift
	calls := 0
	p := planning(shiftplanning.PlanningDependencies{Shifts: planningShifts{
		create: func(c context.Context, v *models.StaffShift) (*models.StaffShift, error) {
			require.Equal(t, ctx, c)
			require.Equal(t, staff.ID, v.CreatedBy)
			require.Equal(t, staff.ID, v.StaffID)
			require.Equal(t, "2026-09-21", v.Date.String())
			require.Equal(t, "08:30", v.StartTime.Format("15:04"))
			require.Equal(t, 30, v.BreakMinutes)
			calls++
			saved = v
			return v, nil
		},
		update: func(_ context.Context, v *models.StaffShift, o shiftplanning.StaffShiftUpdateOptions) (*models.StaffShift, error) {
			require.Equal(t, staff.ID, *v.UpdatedBy)
			require.Equal(t, shiftplanning.StaffShiftUpdateOptions{PreserveExistingNotes: true, PreserveExistingShiftType: true, PreserveExistingChangeReason: true, PreserveExistingCancelled: true}, o)
			calls++
			return v, nil
		},
		move: func(_ context.Context, v shiftplanning.MoveShiftInput) (*models.StaffShift, error) {
			require.Equal(t, staff.ID, v.TargetStaffID)
			require.Equal(t, "2026-09-22", v.Date.String())
			require.Equal(t, "09:00", v.StartTime.Format("15:04"))
			calls++
			return saved, nil
		},
		cancel: func(_ context.Context, v shiftplanning.CancelShiftInput) (*shiftplanning.CancelShiftResult, error) {
			require.True(t, v.Cancelled)
			require.Equal(t, staff.ID, v.ActorStaffID)
			require.Equal(t, "10:00", v.StartTime.Format("15:04"))
			require.Len(t, v.Replacements, 1)
			require.Equal(t, staff.ID, v.Replacements[0].StaffID)
			require.Equal(t, "11:00", v.Replacements[0].StartTime.Format("15:04"))
			calls++
			return &shiftplanning.CancelShiftResult{Shift: saved, Replacements: []*models.StaffShift{saved, nil}}, nil
		},
		list: func(_ context.Context, id int64, a, b timezone.Date) ([]*models.StaffShift, error) {
			require.Contains(t, []int64{0, staff.ID}, id)
			require.Equal(t, "2026-09-21", a.String())
			require.Equal(t, "2026-09-25", b.String())
			calls++
			return []*models.StaffShift{nil, saved}, nil
		},
		remove: func(_ context.Context, id int64) error { require.Equal(t, staff.ID, id); calls++; return nil },
	}})
	created, err := p.CreateShift(ctx, workforce.CreateStaffShift{StaffShiftInput: input})
	require.NoError(t, err)
	require.Equal(t, "14:45:00", created.EndTime)
	_, err = p.UpdateShift(ctx, workforce.UpdateStaffShift{StaffShiftInput: input, PreserveExistingNotes: true, PreserveExistingShiftType: true, PreserveExistingChangeReason: true})
	require.NoError(t, err)
	_, err = p.MoveShift(ctx, workforce.MoveStaffShift{TargetStaffID: staff.ID, Date: "2026-09-22", StartTime: "09:00:00", EndTime: "15:00:00"})
	require.NoError(t, err)
	cancelled, err := p.ApplyCancellation(ctx, workforce.CancelStaffShift{Cancelled: true, ActorStaffID: staff.ID, ApplyOriginEdits: true, StartTime: "10:00:00", EndTime: "15:00:00", Replacements: []workforce.ShiftReplacement{{StaffID: staff.ID, StartTime: "11:00:00", EndTime: "15:00:00"}}})
	require.NoError(t, err)
	require.Len(t, cancelled.Replacements, 1)
	for _, id := range []int64{0, staff.ID} {
		rows, e := p.ListShifts(ctx, workforce.ShiftRange{From: "2026-09-21", To: "2026-09-25", StaffID: id})
		require.NoError(t, e)
		require.Len(t, rows, 1)
	}
	require.NoError(t, p.DeleteShift(ctx, staff.ID))
	require.Equal(t, 7, calls)
}

func TestPlanningRejectsMalformedInputsBeforeCallingDependencies(t *testing.T) {
	t.Parallel()
	p := planning(shiftplanning.PlanningDependencies{})
	ctx := context.Background()
	for _, input := range []workforce.StaffShiftInput{{Date: "bad"}, {Date: "2026-09-21", StartTime: "bad"}, {Date: "2026-09-21", StartTime: "09:00:00", EndTime: "bad"}} {
		_, err := p.CreateShift(ctx, workforce.CreateStaffShift{StaffShiftInput: input})
		require.ErrorIs(t, err, workforce.ErrInvalidStaffShift)
		_, err = p.UpdateShift(ctx, workforce.UpdateStaffShift{StaffShiftInput: input})
		require.ErrorIs(t, err, workforce.ErrInvalidStaffShift)
		_, err = p.MoveShift(ctx, workforce.MoveStaffShift{Date: input.Date, StartTime: input.StartTime, EndTime: input.EndTime})
		require.ErrorIs(t, err, workforce.ErrInvalidStaffShift)
	}
	for _, dates := range [][2]string{{"bad", "2026-09-25"}, {"2026-09-21", "bad"}} {
		_, err := p.ListShifts(ctx, workforce.ShiftRange{From: dates[0], To: dates[1]})
		require.ErrorIs(t, err, workforce.ErrInvalidStaffShift)
		_, err = p.Overview(ctx, dates[0], dates[1])
		require.ErrorIs(t, err, workforce.ErrInvalidStaffShift)
	}
	_, err := p.ApplyCancellation(ctx, workforce.CancelStaffShift{ApplyOriginEdits: true, StartTime: "bad"})
	require.ErrorIs(t, err, workforce.ErrInvalidStaffShift)
	_, err = p.ApplyCancellation(ctx, workforce.CancelStaffShift{Replacements: []workforce.ShiftReplacement{{StartTime: "bad"}}})
	require.ErrorIs(t, err, workforce.ErrInvalidStaffShift)
	_, err = p.EndSeries(ctx, 0, "bad")
	require.ErrorIs(t, err, workforce.ErrInvalidStaffShift)
}

func TestPlanningSeriesPreservesRecurrenceAndExplicitClears(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	staff := testpkg.CreateTestStaff(t, db, "Series", "Contract")
	ctx := testpkg.Ctx(t)
	var saved *models.StaffShiftSeries
	calls := 0
	p := planning(shiftplanning.PlanningDependencies{Series: planningSeries{
		create: func(_ context.Context, v *models.StaffShiftSeries) (*shiftplanning.SeriesResult, error) {
			require.Equal(t, []int16{1, 3, 5}, v.Weekdays)
			require.Equal(t, "2026-09-21", v.ValidFrom.String())
			require.Equal(t, "2026-10-30", v.ValidUntil.String())
			require.Equal(t, staff.ID, v.CreatedBy)
			require.Equal(t, "09:00", v.StartTime.Format("15:04"))
			saved = v
			calls++
			return &shiftplanning.SeriesResult{Series: v, Created: 4, SkippedDates: []timezone.Date{timezone.NewDate(2026, 9, 23)}}, nil
		},
		split: func(_ context.Context, v shiftplanning.SplitSeriesInput) (*shiftplanning.SeriesResult, error) {
			require.Equal(t, "2026-09-28", v.EffectiveDate.String())
			require.True(t, v.ValidUntilSet)
			require.True(t, v.ShiftTypeIDSet)
			require.Nil(t, v.ShiftTypeID)
			require.Equal(t, []int16{2, 4}, v.Weekdays)
			require.Equal(t, "2026-11-30", v.ValidUntil.String())
			calls++
			return &shiftplanning.SeriesResult{Series: saved, Created: 2, Deleted: 3}, nil
		},
		get: func(context.Context, int64) (*models.StaffShiftSeries, error) { calls++; return saved, nil },
		end: func(_ context.Context, _ int64, d timezone.Date) (*shiftplanning.SeriesResult, error) {
			require.Equal(t, "2026-10-01", d.String())
			calls++
			return &shiftplanning.SeriesResult{Deleted: 5}, nil
		},
	}})
	result, err := p.CreateSeries(ctx, workforce.CreateStaffShiftSeries{StaffShiftSeriesInput: workforce.StaffShiftSeriesInput{StaffID: staff.ID, ActorStaffID: staff.ID, Weekdays: []int{1, 3, 5}, StartTime: "09:00:00", EndTime: "15:00:00", ValidFrom: "2026-09-21", ValidUntil: "2026-10-30"}})
	require.NoError(t, err)
	require.Equal(t, 4, result.Created)
	require.Equal(t, []string{"2026-09-23"}, result.SkippedDates)
	result, err = p.SplitSeries(ctx, workforce.SplitStaffShiftSeries{EffectiveDate: "2026-09-28", StartTime: "10:00:00", EndTime: "16:00:00", Weekdays: []int{2, 4}, ValidUntil: "2026-11-30", ValidUntilSet: true, ShiftTypeIDSet: true})
	require.NoError(t, err)
	require.EqualValues(t, 3, result.Deleted)
	series, err := p.GetSeries(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, []int{1, 3, 5}, series.Weekdays)
	require.Equal(t, "2026-10-30", series.ValidUntil)
	require.Equal(t, "15:00:00", series.EndTime)
	result, err = p.EndSeries(ctx, 0, "2026-10-01")
	require.NoError(t, err)
	require.EqualValues(t, 5, result.Deleted)
	require.NotNil(t, result.SkippedDates)
	require.Equal(t, 4, calls)
}

func TestPlanningSeriesRejectsMalformedRecurrence(t *testing.T) {
	t.Parallel()
	p := planning(shiftplanning.PlanningDependencies{})
	ctx := context.Background()
	for _, change := range []func(*workforce.StaffShiftSeriesInput){
		func(v *workforce.StaffShiftSeriesInput) { v.ValidFrom = "bad" }, func(v *workforce.StaffShiftSeriesInput) { v.ValidUntil = "bad" }, func(v *workforce.StaffShiftSeriesInput) { v.StartTime = "bad" }, func(v *workforce.StaffShiftSeriesInput) { v.EndTime = "bad" }, func(v *workforce.StaffShiftSeriesInput) { v.Weekdays = []int{8} },
	} {
		v := workforce.StaffShiftSeriesInput{ValidFrom: "2026-09-21", StartTime: "09:00:00", EndTime: "15:00:00"}
		change(&v)
		_, err := p.CreateSeries(ctx, workforce.CreateStaffShiftSeries{StaffShiftSeriesInput: v})
		require.Error(t, err)
		_, err = p.SplitSeries(ctx, workforce.SplitStaffShiftSeries{EffectiveDate: v.ValidFrom, ValidUntil: v.ValidUntil, StartTime: v.StartTime, EndTime: v.EndTime, Weekdays: v.Weekdays})
		require.Error(t, err)
	}
}

func TestPlanningOverviewAndExportRetainPublicResults(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	from := timezone.NewDate(2026, 9, 21)
	to := timezone.NewDate(2026, 9, 25)
	p := planning(shiftplanning.PlanningDependencies{
		Overview: planningOverview(func(_ context.Context, a, b timezone.Date) (*shiftplanning.StaffScheduleOverview, error) {
			require.Equal(t, from, a)
			require.Equal(t, to, b)
			return &shiftplanning.StaffScheduleOverview{From: a, To: b, DienstplanInUse: true, UsedWeeks: []timezone.Date{a}, Assignments: []shiftplanning.StaffScheduleAssignment{{Date: a, ActivityTitle: "Betreuung", UncoveredIntervals: []schedule.ShiftCoverageInterval{{}}}}, WeeklySummaries: []shiftplanning.StaffWeeklySummary{{WeekStart: a, PlannedMinutes: 300}}}, nil
		}),
		PlanExport: planningExport{export: func(context.Context, planexport.Params) (listexport.File, error) {
			return listexport.File{Filename: "plan.pdf", ContentType: "application/pdf", Data: []byte("document")}, nil
		}},
	})
	overview, err := p.Overview(ctx, from.String(), to.String())
	require.NoError(t, err)
	require.True(t, overview.DienstplanInUse)
	require.Equal(t, []string{"2026-09-21"}, overview.UsedWeeks)
	require.Equal(t, "Betreuung", overview.Assignments[0].ActivityTitle)
	require.Len(t, overview.Assignments[0].UncoveredIntervals, 1)
	require.Equal(t, 300, overview.WeeklySummaries[0].PlannedMinutes)
	request := workforce.PlanExportRequest{From: from.String(), To: to.String(), Template: "persons", Variant: "aushang", Format: "pdf"}
	file, err := p.ExportPlan(ctx, request)
	require.NoError(t, err)
	require.Equal(t, "plan.pdf", file.Filename)
	require.Equal(t, []byte("document"), file.Data)
	request.Variant = "intern"
	_, err = p.ExportPlan(ctx, request)
	require.ErrorIs(t, err, workforce.ErrPlanExportForbidden)
	request.AllowInternal = true
	_, err = p.ExportPlan(ctx, request)
	require.NoError(t, err)
	request.From = "bad"
	_, err = p.ExportPlan(ctx, request)
	require.ErrorIs(t, err, workforce.ErrPlanExportInvalid)
}

func TestPlanningKeepsErrorIdentityAndCause(t *testing.T) {
	t.Parallel()
	for _, pair := range [][2]error{{shiftplanning.ErrShiftOverlap, workforce.ErrStaffShiftOverlap}, {shiftplanning.ErrShiftConflict, workforce.ErrStaffShiftConflict}, {shiftplanning.ErrShiftNotFound, workforce.ErrStaffShiftNotFound}, {shiftplanning.ErrShiftRangeTooLarge, workforce.ErrStaffShiftRangeTooLarge}, {shiftplanning.ErrShiftInvalid, workforce.ErrInvalidStaffShift}, {shiftplanning.ErrShiftTypeNotFound, workforce.ErrShiftTypeNotFound}, {shiftplanning.ErrShiftTypeInactive, workforce.ErrShiftTypeInactive}, {shiftplanning.ErrSeriesNotFound, workforce.ErrShiftSeriesNotFound}, {shiftplanning.ErrSeriesInvalid, workforce.ErrInvalidShiftSeries}} {
		cause := fmt.Errorf("repository: %w", pair[0])
		p := planning(shiftplanning.PlanningDependencies{Shifts: planningShifts{remove: func(context.Context, int64) error { return cause }}})
		err := p.DeleteShift(context.Background(), 0)
		require.ErrorIs(t, err, pair[1])
		require.ErrorIs(t, err, pair[0])
		require.Equal(t, cause.Error(), err.Error())
		require.Same(t, cause, errors.Unwrap(err))
	}
}
