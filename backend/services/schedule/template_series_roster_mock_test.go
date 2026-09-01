// Mock-driven tests for the #2187 split-series roster reconciliation: the
// repository-failure branches and the row classifications the DB-backed suite
// cannot set up cheaply (a row of another calendar period, a row outside the
// reconciliation window, a sibling segment with a broken schedule envelope).
//
// Everything here is stack-allocated: the fakes embed the repository
// interfaces and implement only the handful of methods this code path calls,
// the same shape template_end_service_unit_test.go uses.
package schedule

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	seriesMockLivingID = int64(9001)
	seriesMockOldID    = int64(9002)
	seriesMockTenantID = int64(9003)
	seriesMockStudent  = int64(9101)
	seriesMockStaff    = int64(9201)
)

var errSeriesMock = errors.New("repository exploded")

func seriesMockAnchor() timezone.Date { return timezone.NewDate(2026, 8, 24).AddDays(7) }

func seriesMockUntil() timezone.Date { return timezone.NewDate(2026, 8, 24).AddDays(21) }

// ---------------------------------------------------------------- fakes ----

type seriesMockGroupRepo struct {
	activitiesModel.GroupRepository
	groups []*activitiesModel.Group
	err    error
}

func (r *seriesMockGroupRepo) FindTemplateSeries(context.Context, int64) ([]*activitiesModel.Group, error) {
	return r.groups, r.err
}

type seriesMockScheduleRepo struct {
	activitiesModel.ScheduleRepository
	byGroup map[int64][]*activitiesModel.Schedule
	err     error
}

func (r *seriesMockScheduleRepo) FindByGroupID(_ context.Context, groupID int64) ([]*activitiesModel.Schedule, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.byGroup[groupID], nil
}

type seriesMockEnrollmentRepo struct {
	activitiesModel.StudentEnrollmentRepository
	rows      []*activitiesModel.StudentEnrollment
	findErr   error
	createErr error
	closeErr  error
	deleteErr error
	created   []*activitiesModel.StudentEnrollment
	closed    []int64
	deleted   []int64
}

func (r *seriesMockEnrollmentRepo) FindByGroupID(context.Context, int64) ([]*activitiesModel.StudentEnrollment, error) {
	return r.rows, r.findErr
}

func (r *seriesMockEnrollmentRepo) Create(_ context.Context, row *activitiesModel.StudentEnrollment) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.created = append(r.created, row)
	return nil
}

func (r *seriesMockEnrollmentRepo) SetValidUntilByID(_ context.Context, id int64, _ timezone.Date) error {
	if r.closeErr != nil {
		return r.closeErr
	}
	r.closed = append(r.closed, id)
	return nil
}

func (r *seriesMockEnrollmentRepo) Delete(_ context.Context, id any) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	if numeric, ok := id.(int64); ok {
		r.deleted = append(r.deleted, numeric)
	}
	return nil
}

type seriesMockSupervisorRepo struct {
	activitiesModel.SupervisorPlannedRepository
	rows      []*activitiesModel.SupervisorPlanned
	findErr   error
	createErr error
	closeErr  error
	deleteErr error
	created   []*activitiesModel.SupervisorPlanned
	closed    []int64
	deleted   []int64
}

func (r *seriesMockSupervisorRepo) FindByGroupID(context.Context, int64) ([]*activitiesModel.SupervisorPlanned, error) {
	return r.rows, r.findErr
}

func (r *seriesMockSupervisorRepo) Create(_ context.Context, row *activitiesModel.SupervisorPlanned) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.created = append(r.created, row)
	return nil
}

func (r *seriesMockSupervisorRepo) SetValidUntilByID(_ context.Context, id int64, _ timezone.Date) error {
	if r.closeErr != nil {
		return r.closeErr
	}
	r.closed = append(r.closed, id)
	return nil
}

func (r *seriesMockSupervisorRepo) Delete(_ context.Context, id any) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	if numeric, ok := id.(int64); ok {
		r.deleted = append(r.deleted, numeric)
	}
	return nil
}

// -------------------------------------------------------------- fixtures ----

func seriesMockGroup(id int64) *activitiesModel.Group {
	group := &activitiesModel.Group{Name: "Series mock", IsTemplate: true}
	group.ID = id
	return group
}

func seriesMockSchedule(groupID int64, from timezone.Date, until *timezone.Date) *activitiesModel.Schedule {
	start := from
	return &activitiesModel.Schedule{
		Weekday:         activitiesModel.WeekdayMonday,
		ActivityGroupID: groupID,
		ValidFrom:       &start,
		ValidUntil:      until,
	}
}

// seriesMockChain wires a two-segment lineage: a bounded predecessor
// [today, anchor+14) and the living segment that starts where it ends.
func seriesMockChain(
	enrollments *seriesMockEnrollmentRepo,
	supervisors *seriesMockSupervisorRepo,
) *TimetableDataService {
	until := seriesMockUntil()
	return NewTimetableDataService(TimetableDataDependencies{
		ActivityGroupRepo: &seriesMockGroupRepo{groups: []*activitiesModel.Group{
			seriesMockGroup(seriesMockOldID), seriesMockGroup(seriesMockLivingID),
		}},
		ActivityScheduleRepo: &seriesMockScheduleRepo{byGroup: map[int64][]*activitiesModel.Schedule{
			seriesMockOldID:    {seriesMockSchedule(seriesMockOldID, timezone.NewDate(2026, 8, 24), &until)},
			seriesMockLivingID: {seriesMockSchedule(seriesMockLivingID, until, nil)},
		}},
		StudentEnrollmentRepo:  enrollments,
		ActivitySupervisorRepo: supervisors,
		Today:                  seriesMockAnchor,
	})
}

func seriesMockInput() TemplateUpdateInput {
	anchor := seriesMockAnchor()
	return TemplateUpdateInput{
		TemplateID:                  seriesMockLivingID,
		Weekdays:                    []int{activitiesModel.WeekdayMonday},
		StudentIDs:                  []int64{seriesMockStudent},
		StaffIDs:                    []int64{seriesMockStaff},
		SeriesRosterFrom:            &anchor,
		SeriesRosterScopeStudentIDs: []int64{seriesMockStudent},
		SeriesRosterScopeStaffIDs:   []int64{seriesMockStaff},
	}
}

// ----------------------------------------------------------------- tests ----

func TestReconcileSeriesPredecessorRoster_NoScopeIsANoop(t *testing.T) {
	t.Parallel()

	enrollments := &seriesMockEnrollmentRepo{}
	svc := seriesMockChain(enrollments, &seriesMockSupervisorRepo{})

	in := seriesMockInput()
	in.SeriesRosterScopeStudentIDs = nil
	in.SeriesRosterScopeStaffIDs = nil

	require.NoError(t, svc.reconcileSeriesPredecessorRoster(t.Context(), in, seriesMockTenantID, nil))
	assert.Empty(t, enrollments.created,
		"an anchor without a scope must not write anything to a predecessor")
}

func TestReconcileSeriesPredecessorRoster_CreatesBoundedRows(t *testing.T) {
	t.Parallel()

	enrollments := &seriesMockEnrollmentRepo{}
	supervisors := &seriesMockSupervisorRepo{}
	svc := seriesMockChain(enrollments, supervisors)

	require.NoError(t, svc.reconcileSeriesPredecessorRoster(
		t.Context(), seriesMockInput(), seriesMockTenantID, nil))

	require.Len(t, enrollments.created, 1)
	assert.Equal(t, seriesMockAnchor(), enrollments.created[0].ValidFrom)
	require.NotNil(t, enrollments.created[0].ValidUntil)
	assert.Equal(t, seriesMockUntil(), *enrollments.created[0].ValidUntil)
	require.NotNil(t, enrollments.created[0].Weekday)
	assert.Equal(t, activitiesModel.WeekdayMonday, *enrollments.created[0].Weekday)
	require.Len(t, supervisors.created, 1)
	assert.Equal(t, seriesMockStaff, supervisors.created[0].StaffID)
}

func TestReconcileSeriesPredecessorRoster_RepositoryFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		build  func() *TimetableDataService
		mutate func(*TemplateUpdateInput)
	}{
		{
			name: "lineage lookup fails",
			build: func() *TimetableDataService {
				return NewTimetableDataService(TimetableDataDependencies{
					ActivityGroupRepo:    &seriesMockGroupRepo{err: errSeriesMock},
					ActivityScheduleRepo: &seriesMockScheduleRepo{},
				})
			},
		},
		{
			name: "schedule lookup fails",
			build: func() *TimetableDataService {
				return NewTimetableDataService(TimetableDataDependencies{
					ActivityGroupRepo: &seriesMockGroupRepo{groups: []*activitiesModel.Group{
						seriesMockGroup(seriesMockLivingID),
					}},
					ActivityScheduleRepo: &seriesMockScheduleRepo{err: errSeriesMock},
				})
			},
		},
		{
			name: "predecessor enrollments cannot be read",
			build: func() *TimetableDataService {
				return seriesMockChain(&seriesMockEnrollmentRepo{findErr: errSeriesMock}, &seriesMockSupervisorRepo{})
			},
		},
		{
			name: "predecessor supervisors cannot be read",
			build: func() *TimetableDataService {
				return seriesMockChain(&seriesMockEnrollmentRepo{}, &seriesMockSupervisorRepo{findErr: errSeriesMock})
			},
		},
		{
			name: "creating the predecessor enrollment fails",
			build: func() *TimetableDataService {
				return seriesMockChain(&seriesMockEnrollmentRepo{createErr: errSeriesMock}, &seriesMockSupervisorRepo{})
			},
		},
		{
			name: "creating the predecessor supervision fails",
			build: func() *TimetableDataService {
				return seriesMockChain(&seriesMockEnrollmentRepo{}, &seriesMockSupervisorRepo{createErr: errSeriesMock})
			},
		},
		{
			name: "closing a retired enrollment fails",
			build: func() *TimetableDataService {
				until := seriesMockUntil()
				return seriesMockChain(&seriesMockEnrollmentRepo{
					rows:     []*activitiesModel.StudentEnrollment{seriesMockEnrollmentRow(1, timezone.NewDate(2026, 8, 24), &until, nil)},
					closeErr: errSeriesMock,
				}, &seriesMockSupervisorRepo{})
			},
			// The child is gone from the saved roster, so their predecessor row
			// has to be closed at the anchor.
			mutate: func(in *TemplateUpdateInput) { in.StudentIDs = nil },
		},
		{
			name: "closing a retired supervision fails",
			build: func() *TimetableDataService {
				until := seriesMockUntil()
				return seriesMockChain(&seriesMockEnrollmentRepo{}, &seriesMockSupervisorRepo{
					rows:     []*activitiesModel.SupervisorPlanned{seriesMockSupervisorRow(1, timezone.NewDate(2026, 8, 24), &until, nil)},
					closeErr: errSeriesMock,
				})
			},
			mutate: func(in *TemplateUpdateInput) { in.StaffIDs = nil },
		},
		{
			name: "deleting an enrollment that only started after the anchor fails",
			build: func() *TimetableDataService {
				until := seriesMockUntil()
				return seriesMockChain(&seriesMockEnrollmentRepo{
					rows: []*activitiesModel.StudentEnrollment{
						seriesMockEnrollmentRow(1, seriesMockAnchor().AddDays(1), &until, nil),
					},
					deleteErr: errSeriesMock,
				}, &seriesMockSupervisorRepo{})
			},
			// Nothing of the row survives the anchor, so it is deleted outright
			// instead of being capped.
			mutate: func(in *TemplateUpdateInput) { in.StudentIDs = nil },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := seriesMockInput()
			if tc.mutate != nil {
				tc.mutate(&in)
			}
			err := tc.build().reconcileSeriesPredecessorRoster(t.Context(), in, seriesMockTenantID, nil)
			require.Error(t, err)
			assert.ErrorIs(t, err, errSeriesMock)
		})
	}
}

// TestReconcileSeriesPredecessorRoster_SkipsUnrelatedRows pins the three
// classifications that make an existing row none of this pass's business.
func TestReconcileSeriesPredecessorRoster_SkipsUnrelatedRows(t *testing.T) {
	t.Parallel()

	otherPeriod := int64(4242)
	longPast := timezone.NewDate(2026, 8, 24).AddDays(-90)
	pastEnd := timezone.NewDate(2026, 8, 24).AddDays(-60)
	wednesday := activitiesModel.WeekdayWednesday
	until := seriesMockUntil()

	enrollments := &seriesMockEnrollmentRepo{rows: []*activitiesModel.StudentEnrollment{
		// Another calendar period — owned by a different planning window.
		seriesMockEnrollmentRow(11, timezone.NewDate(2026, 8, 24), &until, &otherPeriod),
		// A weekday the edit does not describe.
		func() *activitiesModel.StudentEnrollment {
			row := seriesMockEnrollmentRow(12, timezone.NewDate(2026, 8, 24), &until, nil)
			row.Weekday = &wednesday
			return row
		}(),
		// A window that ended long before the reconciliation window.
		seriesMockEnrollmentRow(13, longPast, &pastEnd, nil),
	}}
	supervisors := &seriesMockSupervisorRepo{rows: []*activitiesModel.SupervisorPlanned{
		seriesMockSupervisorRow(21, timezone.NewDate(2026, 8, 24), &until, &otherPeriod),
		func() *activitiesModel.SupervisorPlanned {
			row := seriesMockSupervisorRow(22, timezone.NewDate(2026, 8, 24), &until, nil)
			row.Weekday = &wednesday
			return row
		}(),
		seriesMockSupervisorRow(23, longPast, &pastEnd, nil),
	}}

	svc := seriesMockChain(enrollments, supervisors)
	require.NoError(t, svc.reconcileSeriesPredecessorRoster(
		t.Context(), seriesMockInput(), seriesMockTenantID, nil))

	assert.Empty(t, enrollments.closed, "unrelated enrollment rows are never rewritten")
	assert.Empty(t, enrollments.deleted)
	assert.Empty(t, supervisors.closed, "unrelated supervisor rows are never rewritten")
	assert.Empty(t, supervisors.deleted)
	assert.Len(t, enrollments.created, 1, "the wanted membership is still written")
	assert.Len(t, supervisors.created, 1)
}

// TestLoadTemplateSeriesSegments_SkipsBrokenSibling: one corrupt sibling must
// not block resolving the rest of the lineage, while the requested template's
// own corruption stays a hard error.
func TestLoadTemplateSeriesSegments_SkipsBrokenSibling(t *testing.T) {
	t.Parallel()

	until := seriesMockUntil()
	other := until.AddDays(7)
	broken := []*activitiesModel.Schedule{
		seriesMockSchedule(seriesMockOldID, timezone.NewDate(2026, 8, 24), &until),
		seriesMockSchedule(seriesMockOldID, timezone.NewDate(2026, 8, 24), &other),
	}
	groupRepo := &seriesMockGroupRepo{groups: []*activitiesModel.Group{
		seriesMockGroup(seriesMockOldID), seriesMockGroup(seriesMockLivingID),
	}}
	scheduleRepo := &seriesMockScheduleRepo{byGroup: map[int64][]*activitiesModel.Schedule{
		seriesMockOldID:    broken,
		seriesMockLivingID: {seriesMockSchedule(seriesMockLivingID, until, nil)},
	}}

	segments, err := loadTemplateSeriesSegments(t.Context(), groupRepo, scheduleRepo, nil, seriesMockLivingID)
	require.NoError(t, err)
	require.Len(t, segments, 1, "the broken sibling is skipped with a warning")
	assert.Equal(t, seriesMockLivingID, segments[0].Group.ID)

	_, err = loadTemplateSeriesSegments(t.Context(), groupRepo, scheduleRepo, nil, seriesMockOldID)
	require.Error(t, err, "the requested template's own envelope must fail loudly")
}

func TestResolveLivingTemplateSegment_PropagatesLineageErrors(t *testing.T) {
	t.Parallel()

	svc := NewTimetableDataService(TimetableDataDependencies{
		ActivityGroupRepo:    &seriesMockGroupRepo{err: errSeriesMock},
		ActivityScheduleRepo: &seriesMockScheduleRepo{},
	})

	_, _, err := svc.ResolveLivingTemplateSegment(t.Context(), seriesMockLivingID)
	require.Error(t, err)
	assert.ErrorIs(t, err, errSeriesMock)
}

func seriesMockEnrollmentRow(id int64, from timezone.Date, until *timezone.Date, periodID *int64) *activitiesModel.StudentEnrollment {
	row := &activitiesModel.StudentEnrollment{
		StudentID:        seriesMockStudent,
		ActivityGroupID:  seriesMockOldID,
		ValidFrom:        from,
		ValidUntil:       until,
		CalendarPeriodID: periodID,
	}
	row.ID = id
	return row
}

func seriesMockSupervisorRow(id int64, from timezone.Date, until *timezone.Date, periodID *int64) *activitiesModel.SupervisorPlanned {
	row := &activitiesModel.SupervisorPlanned{
		StaffID:          seriesMockStaff,
		GroupID:          seriesMockOldID,
		ValidFrom:        from,
		ValidUntil:       until,
		CalendarPeriodID: periodID,
	}
	row.ID = id
	return row
}
