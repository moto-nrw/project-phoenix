package application

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	internalports "github.com/moto-nrw/project-phoenix/modules/timetable/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ownerEdgeStore records what the owner edge hands to persistence. Every
// method it does not override panics through the nil Store, so a rejected
// request that reached persistence fails the test.
type ownerEdgeStore struct {
	internalports.Store
	create     timetable.CreateCategory
	update     timetable.UpdateCategory
	group      timetable.GroupInput
	template   timetable.TemplateUpdate
	offering   timetable.OfferingSourceInput
	targets    []timetable.GroupTargetInput
	supervisor timetable.PlannedSupervisorInput
	timeframe  timetable.TimeframeInput
	track      timetable.PlanningTrackInput
	calls      int
	rejections []string
}

func (s *ownerEdgeStore) CreateCategory(_ context.Context, input timetable.CreateCategory) (timetable.Category, domain.OperationStats, error) {
	s.calls++
	s.create = input
	return timetable.Category{Name: input.Name, Description: input.Description, Color: input.Color}, domain.OperationStats{}, nil
}

func (s *ownerEdgeStore) FindCategory(_ context.Context, id int64, _ string) (timetable.Category, bool, domain.OperationStats, error) {
	return timetable.Category{ID: id}, true, domain.OperationStats{}, nil
}

func (s *ownerEdgeStore) UpdateCategoryIfActive(_ context.Context, id int64, input timetable.CreateCategory) (timetable.Category, bool, domain.OperationStats, error) {
	s.calls++
	s.update = timetable.UpdateCategory{ID: id, Name: input.Name, Description: input.Description, Color: input.Color}
	return timetable.Category{ID: id, Name: input.Name, Description: input.Description, Color: input.Color}, true, domain.OperationStats{}, nil
}

func (s *ownerEdgeStore) CreateGroup(_ context.Context, input timetable.GroupInput) (timetable.Group, domain.OperationStats, error) {
	s.calls++
	s.group = input
	return timetable.Group{Name: input.Name, Type: input.Type, TargetGroupType: input.TargetGroupType}, domain.OperationStats{}, nil
}

func (s *ownerEdgeStore) UpdateTemplate(_ context.Context, _ int64, input timetable.TemplateUpdate) (int64, domain.OperationStats, error) {
	s.calls++
	s.template = input
	return 1, domain.OperationStats{}, nil
}

func (s *ownerEdgeStore) UpdateGroupOfferingSource(_ context.Context, _ int64, input timetable.OfferingSourceInput) (domain.OperationStats, error) {
	s.calls++
	s.offering = input
	return domain.OperationStats{}, nil
}

func (s *ownerEdgeStore) ReplaceGroupTargets(_ context.Context, _ int64, targets []timetable.GroupTargetInput) (domain.OperationStats, error) {
	s.calls++
	s.targets = targets
	return domain.OperationStats{}, nil
}

func (s *ownerEdgeStore) CreatePlannedSupervisor(_ context.Context, input timetable.PlannedSupervisorInput) (timetable.PlannedSupervisor, domain.OperationStats, error) {
	s.calls++
	s.supervisor = input
	return timetable.PlannedSupervisor{}, domain.OperationStats{}, nil
}

func (s *ownerEdgeStore) CreateTimeframe(_ context.Context, input timetable.TimeframeInput) (timetable.Timeframe, domain.OperationStats, error) {
	s.calls++
	s.timeframe = input
	return timetable.Timeframe{StartTime: input.StartTime, EndTime: input.EndTime}, domain.OperationStats{}, nil
}

func (s *ownerEdgeStore) CreatePlanningTrack(_ context.Context, input timetable.PlanningTrackInput) (timetable.PlanningTrack, domain.OperationStats, error) {
	s.calls++
	s.track = input
	return timetable.PlanningTrack{Name: input.Name, Color: input.Color, SortOrder: input.SortOrder}, domain.OperationStats{}, nil
}

type ownerEdgeTransaction struct{}

func (ownerEdgeTransaction) RunWrite(ctx context.Context, _ bool, callback func(context.Context) error) error {
	return callback(ctx)
}

type ownerEdgeSessions struct {
	internalports.SessionFacts
	calls int
}

func (s *ownerEdgeSessions) RunningSessionsByInstance(context.Context, []int64) (map[int64]int64, error) {
	s.calls++
	return nil, nil
}

type ownerEdgeDependency struct {
	internalports.StudentDirectory
	internalports.RoomDirectory
	internalports.EnrollmentProvenanceStore
}

func newOwnerEdgeService() (*Service, *ownerEdgeStore) {
	service, store, _ := newOwnerEdgeServiceWithSessions()
	return service, store
}

// newOwnerEdgeServiceWithSessions builds the real service over the recording
// store. The observer keeps the failed operations: on these paths only the
// owner edge refuses a request.
func newOwnerEdgeServiceWithSessions() (*Service, *ownerEdgeStore, *ownerEdgeSessions) {
	store := &ownerEdgeStore{}
	sessions := &ownerEdgeSessions{}
	dependency := ownerEdgeDependency{}
	service := New(store, ownerEdgeTransaction{}, dependency, dependency, sessions,
		func(context.Context, int64) error { return nil }, func() string { return "2026-09-01" },
		func(observation internalports.Observation) {
			if observation.Err != nil {
				store.rejections = append(store.rejections, observation.Operation)
			}
		}, dependency)
	return service, store, sessions
}

func TestSessionsRosteringStudentListsNobodyWithoutSessions(t *testing.T) {
	t.Parallel()
	service, store, sessions := newOwnerEdgeServiceWithSessions()
	result, err := service.SessionsRosteringStudent(t.Context(), 0, nil)
	if err != nil || result != nil || sessions.calls != 0 || store.calls != 0 {
		t.Fatalf("SessionsRosteringStudent(no sessions) = %v, %v after %d session and %d store calls", result, err, sessions.calls, store.calls)
	}
}

func TestNewPanicsWithoutDependencies(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { New(nil, nil, nil, nil, nil, nil, nil, nil, nil) })
}

func TestActivityExceptionRejectsInvalidWrites(t *testing.T) {
	t.Parallel()
	service, store := newOwnerEdgeService()
	override := "13:00:00"
	cases := []timetable.ActivityExceptionInput{
		{},
		{ActivityGroupID: 1, ExceptionDate: "not-a-date", ExceptionType: timetable.ActivityExceptionCancelled},
		{ActivityGroupID: 1, ExceptionDate: "2027-01-01", ExceptionType: timetable.ActivityExceptionCancelled, StartTime: &override},
		{ActivityGroupID: 1, ExceptionDate: "2027-01-01", ExceptionType: timetable.ActivityExceptionModified},
	}
	for _, input := range cases {
		_, err := service.CreateActivityException(context.Background(), input)
		require.ErrorIs(t, err, timetable.ErrInvalidActivityException)
	}
	_, err := service.DeleteActivityExceptionsBefore(context.Background(), "not-a-date")
	require.ErrorIs(t, err, timetable.ErrInvalidActivityException)
	assert.Zero(t, store.calls)
}

func TestActivityExceptionRejectsInvalidQueries(t *testing.T) {
	t.Parallel()
	service, store := newOwnerEdgeService()
	_, err := service.FindActivityException(context.Background(), 0)
	require.ErrorIs(t, err, timetable.ErrInvalidActivityExceptionQuery)
	badGroupID := int64(-1)
	_, err = service.ListActivityExceptions(context.Background(), timetable.ActivityExceptionFilter{ActivityGroupID: &badGroupID})
	require.ErrorIs(t, err, timetable.ErrInvalidActivityExceptionQuery)
	badDate := "not-a-date"
	_, err = service.CountActivityExceptions(context.Background(), &badDate)
	require.ErrorIs(t, err, timetable.ErrInvalidActivityExceptionQuery)
	assert.Zero(t, store.calls)
}

func TestModuleNormalizesCategoryWrites(t *testing.T) {
	t.Parallel()
	service, store := newOwnerEdgeService()

	created, err := service.CreateCategory(context.Background(), timetable.CreateCategory{
		Name: "  Werken  ", Description: "  Holz  ", Color: "abc",
	})
	require.NoError(t, err)
	assert.Equal(t, "Werken", created.Name)
	assert.Equal(t, "Holz", created.Description)
	assert.Equal(t, "#abc", created.Color)
	assert.Equal(t, store.create.Name, created.Name)

	updated, err := service.UpdateCategory(context.Background(), timetable.UpdateCategory{
		ID: 17, Name: "  Sport  ", Color: "#123456",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(17), updated.ID)
	assert.Equal(t, "Sport", store.update.Name)
}

func TestModuleValidatesPlannedSupervisorBoundary(t *testing.T) {
	t.Parallel()
	service, store := newOwnerEdgeService()
	ctx := context.Background()

	_, err := service.CreatePlannedSupervisor(ctx, timetable.PlannedSupervisorInput{StaffID: 1, GroupID: 2, ValidFrom: "not-a-date"})
	require.ErrorIs(t, err, timetable.ErrInvalidPlannedSupervisor)
	assert.Zero(t, store.calls)

	weekday := 7
	_, err = service.CreatePlannedSupervisor(ctx, timetable.PlannedSupervisorInput{StaffID: 1, GroupID: 2, Weekday: &weekday})
	require.NoError(t, err)
	assert.Equal(t, &weekday, store.supervisor.Weekday)
}

func TestModuleValidatesStudentEnrollmentBoundary(t *testing.T) {
	t.Parallel()
	service, store := newOwnerEdgeService()
	ctx := context.Background()

	_, err := service.CreateStudentEnrollment(ctx, timetable.StudentEnrollmentInput{
		StudentID: 1, ActivityGroupID: 2, ValidFrom: "not-a-date",
	})
	require.ErrorIs(t, err, timetable.ErrInvalidStudentEnrollment)
	_, err = service.CreateStudentEnrollment(ctx, timetable.StudentEnrollmentInput{
		StudentID: 1, ActivityGroupID: 2, ValidFrom: "2026-09-01", SelectedWeekdays: []int{2, 2},
	})
	require.ErrorIs(t, err, timetable.ErrInvalidStudentEnrollment)
	_, err = service.ListStudentEnrollments(ctx, timetable.StudentEnrollmentFilter{StudentIDs: []int64{0}})
	require.ErrorIs(t, err, timetable.ErrInvalidStudentEnrollmentQuery)
	assert.Zero(t, store.calls)
	assert.Equal(t, []string{
		"create_student_enrollment", "create_student_enrollment", "list_student_enrollments",
	}, store.rejections)
}

func TestModuleValidatesConflictAckBoundary(t *testing.T) {
	t.Parallel()
	service, store := newOwnerEdgeService()
	ctx := context.Background()

	_, err := service.ListConflictAcks(ctx, 0)
	require.ErrorIs(t, err, timetable.ErrInvalidConflictAck)
	err = service.AcknowledgeConflict(ctx, 1, "not-hex")
	require.ErrorIs(t, err, timetable.ErrInvalidConflictAck)
	err = service.AcknowledgeConflict(ctx, 0, "0123456789abcdef")
	require.ErrorIs(t, err, timetable.ErrInvalidConflictAck)
	err = service.UnacknowledgeConflict(ctx, 1, "ABCDEF0123456789")
	require.ErrorIs(t, err, timetable.ErrInvalidConflictAck)
	assert.Zero(t, store.calls)
	assert.Equal(t, []string{
		"list_conflict_acks", "acknowledge_conflict", "acknowledge_conflict", "unacknowledge_conflict",
	}, store.rejections)

	assert.True(t, timetable.ValidConflictAckFingerprint("0123456789abcdef0123456789abcdef"))
	assert.False(t, timetable.ValidConflictAckFingerprint("0123456789abcde"))
}

func TestModuleValidatesTimeframeBoundary(t *testing.T) {
	t.Parallel()
	service, store := newOwnerEdgeService()
	ctx := context.Background()

	_, err := service.CreateTimeframe(ctx, timetable.TimeframeInput{StartTime: "not-a-clock"})
	require.ErrorIs(t, err, timetable.ErrInvalidTimeframe)
	_, err = service.CreateTimeframe(ctx, timetable.TimeframeInput{StartTime: "12:00:00", EndTime: ptr("08:00:00")})
	require.ErrorIs(t, err, timetable.ErrInvalidTimeframe)
	_, err = service.ListTimeframes(ctx, timetable.TimeframeFilter{Limit: -1})
	require.ErrorIs(t, err, timetable.ErrInvalidTimeframeQuery)
	assert.Zero(t, store.calls)

	created, err := service.CreateTimeframe(ctx, timetable.TimeframeInput{StartTime: "08:00:00", EndTime: ptr("12:00:00")})
	require.NoError(t, err)
	assert.Equal(t, "08:00:00", created.StartTime)
	assert.Equal(t, "08:00:00", store.timeframe.StartTime)
}

func TestModuleNormalizesAndValidatesPlanningTracks(t *testing.T) {
	t.Parallel()
	service, store := newOwnerEdgeService()
	ctx := context.Background()

	_, err := service.CreatePlanningTrack(ctx, timetable.PlanningTrackInput{Name: " ", Color: "#5080D8"})
	require.ErrorIs(t, err, timetable.ErrInvalidPlanningTrack)
	_, err = service.CreatePlanningTrack(ctx, timetable.PlanningTrackInput{Name: "Spur", Color: "blue"})
	require.ErrorIs(t, err, timetable.ErrInvalidPlanningTrack)
	err = service.ReorderPlanningTracks(ctx, []int64{1, 1})
	require.ErrorIs(t, err, timetable.ErrInvalidPlanningTrack)
	assert.Zero(t, store.calls)

	created, err := service.CreatePlanningTrack(ctx, timetable.PlanningTrackInput{Name: " Spur ", Color: "#5080D8"})
	require.NoError(t, err)
	assert.Equal(t, "Spur", created.Name)
	assert.Equal(t, "Spur", store.track.Name)
}

func TestModuleNormalizesAndValidatesGroupWrites(t *testing.T) {
	t.Parallel()
	service, store := newOwnerEdgeService()
	ctx := context.Background()

	created, err := service.CreateGroup(ctx, timetable.GroupInput{Name: "Werkstatt", CategoryID: 7})
	require.NoError(t, err)
	assert.Equal(t, timetable.GroupTypeActivity, created.Type)
	assert.Equal(t, timetable.TargetGroupTypeNone, created.TargetGroupType)

	negative := -1
	_, err = service.UpdateGroup(ctx, 12, timetable.GroupInput{
		Name: "Werkstatt", CategoryID: 7, RequiredStaff: &negative,
	})
	require.ErrorIs(t, err, timetable.ErrInvalidGroup)
	assert.Equal(t, 1, store.calls, "invalid write must not reach the store")
	assert.Contains(t, store.rejections, "update_group")
}

func TestModuleNormalizesAndValidatesTemplateWrites(t *testing.T) {
	t.Parallel()
	service, store := newOwnerEdgeService()
	ctx := context.Background()
	class := " 2b "

	_, err := service.UpdateTemplate(ctx, 9, timetable.TemplateUpdate{
		Name: "Template", CategoryID: 7, TargetGroupType: timetable.TargetGroupTypeSchoolClass,
		TargetSchoolClass: &class,
	})
	require.NoError(t, err)
	assert.Equal(t, timetable.GroupTypeActivity, store.template.Type)
	assert.Equal(t, "2b", *store.template.TargetSchoolClass)
	err = service.UpdateGroupOfferingSource(ctx, 9, timetable.OfferingSourceInput{
		CareOfferingIDs: []int64{4}, SchoolClasses: []string{" 3A "},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"3A"}, store.offering.SchoolClasses)

	_, err = service.UpdateTemplate(ctx, 9, timetable.TemplateUpdate{Name: "", CategoryID: 7})
	require.ErrorIs(t, err, timetable.ErrInvalidGroup)
	err = service.UpdateGroupOfferingSource(ctx, 9, timetable.OfferingSourceInput{GradeLevels: []int{2}})
	require.ErrorIs(t, err, timetable.ErrInvalidGroup)
	_, err = service.ArchiveTemplate(ctx, 0)
	require.ErrorIs(t, err, timetable.ErrInvalidGroup)
	assert.Equal(t, 2, store.calls, "invalid writes must not reach the store")
}

func TestModuleRejectsInvalidAndReservedCategoryWrites(t *testing.T) {
	t.Parallel()
	service, store := newOwnerEdgeService()
	ctx := context.Background()

	_, err := service.CreateCategory(ctx, timetable.CreateCategory{Name: " ", Color: "#abc"})
	require.ErrorIs(t, err, timetable.ErrInvalidCategory)
	_, err = service.CreateCategory(ctx, timetable.CreateCategory{Name: "wc", Color: "#abc"})
	require.ErrorIs(t, err, timetable.ErrSystemCategoryName)
	_, err = service.UpdateCategory(ctx, timetable.UpdateCategory{ID: 3, Name: "Schulhof", Color: "#abc"})
	require.ErrorIs(t, err, timetable.ErrSystemCategoryName)
	_, err = service.UpdateCategory(ctx, timetable.UpdateCategory{ID: 0, Name: "Sport", Color: "#abc"})
	require.ErrorIs(t, err, timetable.ErrInvalidCategory)
	require.ErrorIs(t, service.SetCategoryShiftTypeLinks(ctx, 4, []int64{2, 0}), timetable.ErrInvalidCategory)
	require.ErrorIs(t, service.LockStudentEnrollmentsForCareExit(ctx, nil, "2026-09-04"), timetable.ErrInvalidCareExitEnrollment)
	_, err = service.EndStudentEnrollmentsForCareExit(ctx, []int64{1}, "04.09.2026")
	require.ErrorIs(t, err, timetable.ErrInvalidCareExitEnrollment)
	_, err = service.RestoreStudentEnrollmentsForCareExit(ctx, []int64{1}, nil, []timetable.CareExitEnrollmentRemoval{{
		CareExitEnrollment: timetable.CareExitEnrollment{ID: 1, TenantID: 7, StudentID: 1, ActivityGroupID: 1},
		WasDeleted:         true,
	}})
	require.ErrorIs(t, err, timetable.ErrInvalidCareExitEnrollment)

	assert.Zero(t, store.calls, "invalid input must never reach persistence")
	assert.Equal(t, []string{
		"create_category", "create_category", "update_category", "update_category", "set_category_shift_type_links",
		"lock_student_enrollments_for_care_exit", "end_student_enrollments_for_care_exit",
		"restore_student_enrollments_for_care_exit",
	}, store.rejections)
}

func TestModuleRejectsInvalidGroupQueries(t *testing.T) {
	t.Parallel()
	service, store := newOwnerEdgeService()
	ctx := context.Background()

	_, err := service.FindGroup(ctx, 0)
	require.ErrorIs(t, err, timetable.ErrInvalidGroupQuery)
	_, err = service.ListGroups(ctx, timetable.GroupFilter{IDs: []int64{1, 0}})
	require.ErrorIs(t, err, timetable.ErrInvalidGroupQuery)
	_, err = service.ListGroups(ctx, timetable.GroupFilter{SourceOfferingIDs: []int64{-1}})
	require.ErrorIs(t, err, timetable.ErrInvalidGroupQuery)
	invalidSeriesID := int64(0)
	_, err = service.ListGroups(ctx, timetable.GroupFilter{SeriesForGroupID: &invalidSeriesID})
	require.ErrorIs(t, err, timetable.ErrInvalidGroupQuery)
	_, err = service.ListGroupTargets(ctx, []int64{-1})
	require.ErrorIs(t, err, timetable.ErrInvalidGroupQuery)

	assert.Zero(t, store.calls)
	assert.Equal(t, []string{"find_group", "list_groups", "list_groups", "list_groups", "list_group_targets"}, store.rejections)
}

func TestModuleNormalizesAndRejectsGroupTargets(t *testing.T) {
	t.Parallel()
	service, store := newOwnerEdgeService()
	ctx := context.Background()
	class := " 2b "

	require.NoError(t, service.ReplaceGroupTargets(ctx, 7, []timetable.GroupTargetInput{{
		TargetGroupType: "klasse", TargetSchoolClass: &class,
	}}))
	require.Equal(t, 1, store.calls)
	require.Len(t, store.targets, 1)
	assert.Equal(t, "2b", *store.targets[0].TargetSchoolClass)

	grade := int16(14)
	require.ErrorIs(t, service.ReplaceGroupTargets(ctx, 7, []timetable.GroupTargetInput{{
		TargetGroupType: "jahrgang", TargetGradeLevel: &grade,
	}}), timetable.ErrInvalidGroupTarget)
	require.ErrorIs(t, service.ReplaceGroupTargets(ctx, 0, nil), timetable.ErrInvalidGroupTarget)
	assert.Equal(t, 1, store.calls, "invalid targets must not reach persistence")
}

func TestSystemProvisionerMayCreateReservedCategory(t *testing.T) {
	t.Parallel()
	service, store := newOwnerEdgeService()

	_, err := service.CreateCategory(context.Background(), timetable.CreateCategory{
		Name: timetable.WCCategoryName, IsSystem: true,
	})
	require.NoError(t, err)
	assert.True(t, store.create.IsSystem)
}

func ptr[T any](value T) *T { return &value }
