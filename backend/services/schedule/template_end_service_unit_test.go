package schedule

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateEndInputValidation(t *testing.T) {
	t.Parallel()

	today := timezone.NewDate(2026, 8, 24)
	future := today.AddDays(1)

	tests := []struct {
		name string
		in   TemplateEndInput
	}{
		{
			name: "missing template id",
			in:   TemplateEndInput{EffectiveDate: future},
		},
		{
			name: "missing effective date",
			in:   TemplateEndInput{TemplateID: 10},
		},
		{
			name: "past effective date",
			in:   TemplateEndInput{TemplateID: 10, EffectiveDate: today.AddDays(-1)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTemplateEndInput(tt.in, today)

			require.ErrorIs(t, err, ErrSplitInvalidInput)
		})
	}

	require.NoError(t, validateTemplateEndInput(TemplateEndInput{TemplateID: 10, EffectiveDate: future}, today))
}

func TestValidateProtectedEnrollmentRebaseRejectsActiveScopedCollision(t *testing.T) {
	t.Parallel()

	periodA, periodB, targetPeriod := int64(11), int64(12), int64(13)
	activeA := &activitiesModel.StudentEnrollment{
		StudentID:                42,
		CalendarPeriodID:         &periodA,
		EnrollmentRequestChildID: testInt64Ptr(101),
	}
	activeB := &activitiesModel.StudentEnrollment{
		StudentID:                42,
		CalendarPeriodID:         &periodB,
		EnrollmentRequestChildID: testInt64Ptr(101),
	}

	err := validateProtectedEnrollmentRebase(
		[]*activitiesModel.StudentEnrollment{activeA, activeB},
		&targetPeriod,
	)
	require.ErrorIs(t, err, ErrTemplateRosterRebaseConflict)

	boundedUntil := activitiesModel.Date(timezone.TodayDate().AddDays(30))
	activeB.ValidUntil = &boundedUntil
	require.NoError(t, validateProtectedEnrollmentRebase(
		[]*activitiesModel.StudentEnrollment{activeA, activeB},
		&targetPeriod,
	), "bounded rows do not collide with the active-row partial unique index")
	require.NoError(t, validateProtectedEnrollmentRebase(
		[]*activitiesModel.StudentEnrollment{activeA, activeB},
		nil,
	), "without an explicit target period no scoped rows are merged")
}

func testInt64Ptr(value int64) *int64 {
	return &value
}

func TestTemplateEndFromDate_RequiresTenantBeforeMutation(t *testing.T) {
	t.Parallel()

	future := timezone.TodayDate().AddDays(1)
	groupRepo := &templateEndUnitGroupRepo{group: templateEndUnitGroup(201)}
	svc := templateEndUnitService(
		groupRepo,
		&templateEndUnitScheduleRepo{},
		&templateEndUnitEnrollmentRepo{},
		&templateEndUnitSupervisorRepo{},
		&templateEndUnitInstanceRepo{},
	)

	_, err := svc.EndFromDate(context.Background(), TemplateEndInput{TemplateID: 201, EffectiveDate: future})

	require.Error(t, err)
	var scheduleErr *ScheduleError
	require.ErrorAs(t, err, &scheduleErr)
	assert.Equal(t, "end template", scheduleErr.Op)
	assert.Zero(t, groupRepo.findCalls)
}

func TestTemplateEndFromDate_ReturnsSummaryAndDeletesOpenEndedWindow(t *testing.T) {
	t.Parallel()

	ctx := tenant.WithTenantID(context.Background(), 7401)
	future := timezone.NewDate(2030, 8, 26).AddDays(1)
	group := templateEndUnitGroup(202)
	scheduleRepo := &templateEndUnitScheduleRepo{capped: 2}
	enrollmentRepo := &templateEndUnitEnrollmentRepo{capped: 3}
	supervisorRepo := &templateEndUnitSupervisorRepo{capped: 4}
	instanceRepo := &templateEndUnitInstanceRepo{deleted: 5}
	svc := templateEndUnitService(
		&templateEndUnitGroupRepo{group: group},
		scheduleRepo,
		enrollmentRepo,
		supervisorRepo,
		instanceRepo,
	)

	res, err := svc.EndFromDate(ctx, TemplateEndInput{TemplateID: group.ID, EffectiveDate: future})

	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, group.ID, res.TemplateID)
	assert.Equal(t, future, res.EffectiveDate)
	assert.Equal(t, 5, res.DeletedInstances)
	assert.EqualValues(t, 2, res.CappedSchedules)
	assert.EqualValues(t, 3, res.CappedEnrollments)
	assert.EqualValues(t, 4, res.CappedSupervisors)
	assert.Equal(t, group.ID, scheduleRepo.groupID)
	assert.Equal(t, future, scheduleRepo.validUntil)
	assert.Equal(t, group.ID, enrollmentRepo.groupID)
	assert.Equal(t, group.ID, supervisorRepo.groupID)
	require.NotNil(t, instanceRepo.activityGroupID)
	assert.Equal(t, group.ID, *instanceRepo.activityGroupID)
	assert.Equal(t, scheduleModel.Date(future), instanceRepo.from)
	assert.Nil(t, instanceRepo.to)
	// #1840: ending a series is destructive — it must hard-delete deviated
	// rows too, so preservation is OFF.
	assert.False(t, instanceRepo.preserveDeviations,
		"template end must NOT preserve deviations (destructive series delete)")
}

func TestTemplateEndFromDate_ErrorBranches(t *testing.T) {
	t.Parallel()

	ctx := tenant.WithTenantID(context.Background(), 7402)
	future := timezone.TodayDate().AddDays(1)
	group := templateEndUnitGroup(203)

	tests := []struct {
		name           string
		groupRepo      *templateEndUnitGroupRepo
		scheduleRepo   *templateEndUnitScheduleRepo
		enrollmentRepo *templateEndUnitEnrollmentRepo
		supervisorRepo *templateEndUnitSupervisorRepo
		instanceRepo   *templateEndUnitInstanceRepo
		wantErr        error
		wantOp         string
	}{
		{
			name:           "template not found",
			groupRepo:      &templateEndUnitGroupRepo{},
			scheduleRepo:   &templateEndUnitScheduleRepo{},
			enrollmentRepo: &templateEndUnitEnrollmentRepo{},
			supervisorRepo: &templateEndUnitSupervisorRepo{},
			instanceRepo:   &templateEndUnitInstanceRepo{},
			wantErr:        ErrSplitTemplateNotFound,
		},
		{
			name:           "cap schedules error",
			groupRepo:      &templateEndUnitGroupRepo{group: group},
			scheduleRepo:   &templateEndUnitScheduleRepo{err: errors.New("cap schedules failed")},
			enrollmentRepo: &templateEndUnitEnrollmentRepo{},
			supervisorRepo: &templateEndUnitSupervisorRepo{},
			instanceRepo:   &templateEndUnitInstanceRepo{},
			wantOp:         "end template: cap schedules",
		},
		{
			name:           "cap enrollments error",
			groupRepo:      &templateEndUnitGroupRepo{group: group},
			scheduleRepo:   &templateEndUnitScheduleRepo{},
			enrollmentRepo: &templateEndUnitEnrollmentRepo{err: errors.New("cap enrollments failed")},
			supervisorRepo: &templateEndUnitSupervisorRepo{},
			instanceRepo:   &templateEndUnitInstanceRepo{},
			wantOp:         "end template: cap enrollments",
		},
		{
			name:           "cap supervisors error",
			groupRepo:      &templateEndUnitGroupRepo{group: group},
			scheduleRepo:   &templateEndUnitScheduleRepo{},
			enrollmentRepo: &templateEndUnitEnrollmentRepo{},
			supervisorRepo: &templateEndUnitSupervisorRepo{err: errors.New("cap supervisors failed")},
			instanceRepo:   &templateEndUnitInstanceRepo{},
			wantOp:         "end template: cap supervisors",
		},
		{
			name:           "delete instances error",
			groupRepo:      &templateEndUnitGroupRepo{group: group},
			scheduleRepo:   &templateEndUnitScheduleRepo{},
			enrollmentRepo: &templateEndUnitEnrollmentRepo{},
			supervisorRepo: &templateEndUnitSupervisorRepo{},
			instanceRepo:   &templateEndUnitInstanceRepo{err: errors.New("delete failed")},
			wantOp:         "end template: delete planned instances",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := templateEndUnitService(tt.groupRepo, tt.scheduleRepo, tt.enrollmentRepo, tt.supervisorRepo, tt.instanceRepo)

			_, err := svc.EndFromDate(ctx, TemplateEndInput{TemplateID: 203, EffectiveDate: future})

			require.Error(t, err)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			var scheduleErr *ScheduleError
			require.ErrorAs(t, err, &scheduleErr)
			assert.Equal(t, tt.wantOp, scheduleErr.Op)
		})
	}
}

func templateEndUnitService(
	groupRepo *templateEndUnitGroupRepo,
	scheduleRepo *templateEndUnitScheduleRepo,
	enrollmentRepo *templateEndUnitEnrollmentRepo,
	supervisorRepo *templateEndUnitSupervisorRepo,
	instanceRepo *templateEndUnitInstanceRepo,
) *TemplateSplitService {
	return &TemplateSplitService{
		deps: TemplateSplitDependencies{
			GroupRepo:      groupRepo,
			ScheduleRepo:   scheduleRepo,
			EnrollmentRepo: enrollmentRepo,
			SupervisorRepo: supervisorRepo,
			InstanceRepo:   instanceRepo,
			Today:          func() timezone.Date { return timezone.NewDate(2026, 8, 24) },
			Logger:         slog.New(slog.DiscardHandler),
		},
		lockTenantRecurrence: func(context.Context) error { return nil },
	}
}

func templateEndUnitGroup(id int64) *activitiesModel.Group {
	group := &activitiesModel.Group{
		Name:       "Template end unit",
		IsTemplate: true,
	}
	group.ID = id
	return group
}

type templateEndUnitGroupRepo struct {
	activitiesModel.GroupRepository
	group     *activitiesModel.Group
	err       error
	findCalls int
}

func (r *templateEndUnitGroupRepo) FindByID(_ context.Context, _ any) (*activitiesModel.Group, error) {
	r.findCalls++
	if r.err != nil {
		return nil, r.err
	}
	return r.group, nil
}

// FindTemplateSeries backs the #2187 end cascade; the unit fixtures model a
// single-segment lineage (the schedule fake returns no rows, so the cascade
// finds no bounded predecessor and stays a no-op).
func (r *templateEndUnitGroupRepo) FindTemplateSeries(_ context.Context, _ int64) ([]*activitiesModel.Group, error) {
	if r.group == nil {
		return nil, nil
	}
	return []*activitiesModel.Group{r.group}, nil
}

type templateEndUnitScheduleRepo struct {
	activitiesModel.ScheduleRepository
	schedules  []*activitiesModel.Schedule
	findErr    error
	capped     int64
	err        error
	groupID    int64
	validUntil timezone.Date
}

func (r *templateEndUnitScheduleRepo) FindByGroupID(_ context.Context, _ int64) ([]*activitiesModel.Schedule, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return r.schedules, nil
}

func (r *templateEndUnitScheduleRepo) FindByGroupIDs(_ context.Context, _ []int64) ([]*activitiesModel.Schedule, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return r.schedules, nil
}

func (r *templateEndUnitScheduleRepo) CapValidUntil(_ context.Context, groupID int64, validUntil string) (int64, error) {
	r.groupID = groupID
	r.validUntil = timezone.Date(validUntil)
	if r.err != nil {
		return 0, r.err
	}
	return r.capped, nil
}

type templateEndUnitEnrollmentRepo struct {
	activitiesModel.StudentEnrollmentRepository
	capped  int64
	err     error
	groupID int64
}

func (r *templateEndUnitEnrollmentRepo) FindByGroupID(_ context.Context, _ int64) ([]*activitiesModel.StudentEnrollment, error) {
	return nil, nil
}

func (r *templateEndUnitEnrollmentRepo) CapActiveByGroup(_ context.Context, groupID int64, _ activitiesModel.Date) (int64, error) {
	r.groupID = groupID
	if r.err != nil {
		return 0, r.err
	}
	return r.capped, nil
}

type templateEndUnitSupervisorRepo struct {
	activitiesModel.SupervisorPlannedRepository
	capped  int64
	err     error
	groupID int64
}

func (r *templateEndUnitSupervisorRepo) FindByGroupID(_ context.Context, _ int64) ([]*activitiesModel.SupervisorPlanned, error) {
	return nil, nil
}

func (r *templateEndUnitSupervisorRepo) CapActiveByGroup(_ context.Context, groupID int64, _ activitiesModel.Date) (int64, error) {
	r.groupID = groupID
	if r.err != nil {
		return 0, r.err
	}
	return r.capped, nil
}

type templateEndUnitInstanceRepo struct {
	scheduleModel.ActivityInstanceRepository
	deleted            int64
	err                error
	from               scheduleModel.Date
	to                 *scheduleModel.Date
	activityGroupID    *int64
	preserveDeviations bool
}

func (r *templateEndUnitInstanceRepo) DeletePlannedNonSpontaneousInWindow(_ context.Context, from scheduleModel.Date, to *scheduleModel.Date, activityGroupID *int64, preserveDeviations bool) (int64, error) {
	r.from = from
	r.to = to
	r.activityGroupID = activityGroupID
	r.preserveDeviations = preserveDeviations
	if r.err != nil {
		return 0, r.err
	}
	return r.deleted, nil
}

func (r *templateEndUnitInstanceRepo) PropagateListKindToFutureInstances(context.Context, int64, *string, *string, scheduleModel.Date) (int64, error) {
	return 0, nil
}
