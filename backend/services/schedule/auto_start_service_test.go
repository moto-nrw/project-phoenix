package schedule

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutoStart_RunForTenant_StartsOnlyDueStaffedConflictFreeInstances(t *testing.T) {
	now := time.Date(2026, 4, 20, 13, 30, 0, 0, time.Local)
	repo := &autoStartInstanceRepo{instances: []*scheduleModel.ActivityInstance{
		autoStartInstance(101, scheduleModel.InstanceStatusPlanned, 14, 0, 15, 0),
		autoStartInstance(102, scheduleModel.InstanceStatusPlanned, 13, 0, 14, 0),
		autoStartInstance(103, scheduleModel.InstanceStatusPlanned, 13, 0, 14, 0),
		autoStartInstance(104, scheduleModel.InstanceStatusPlanned, 13, 0, 14, 0),
		autoStartInstance(105, scheduleModel.InstanceStatusPlanned, 12, 0, 13, 0),
		autoStartInstance(106, scheduleModel.InstanceStatusActive, 13, 0, 14, 0),
	}}
	staffRepo := &autoStartStaffRepo{counts: map[int64]int{
		102: 1,
		104: 1,
	}}
	starter := &autoStartInstanceStarter{}
	svc := NewAutoStartService(AutoStartDependencies{
		InstanceRepo:      repo,
		InstanceStaffRepo: staffRepo,
		InstanceService:   starter,
		ConflictDetector: func(_ context.Context, _ ConflictDependencies, inst *scheduleModel.ActivityInstance, _ *slog.Logger) []InstanceConflictWarning {
			if inst.ID == 104 {
				return []InstanceConflictWarning{{Kind: ConflictKindRoom, ResourceID: inst.RoomID, CanOverride: true}}
			}
			return nil
		},
		Logger: slog.Default(),
	})

	result, err := svc.RunForTenant(context.Background(), now)

	require.NoError(t, err)
	assert.Equal(t, 6, result.Checked)
	assert.Equal(t, 1, result.Started)
	assert.Equal(t, 1, result.SkippedBeforeWindow)
	assert.Equal(t, 1, result.SkippedAfterWindow)
	assert.Equal(t, 1, result.SkippedNoStaff)
	assert.Equal(t, 1, result.SkippedConflict)
	assert.Equal(t, 1, result.SkippedNonPlanned)
	assert.Equal(t, []int64{102}, starter.startedIDs)
	assert.Equal(t, []int64{0}, starter.startedByStaffIDs, "auto-start must not claim a human starter")
	assert.Equal(t, []int64{101, 102, 103, 104, 105}, staffRepo.requestedIDs)
}

func TestAutoStart_RunForTenant_ReturnsStartError(t *testing.T) {
	now := time.Date(2026, 4, 20, 13, 30, 0, 0, time.Local)
	startErr := errors.New("start failed")
	repo := &autoStartInstanceRepo{instances: []*scheduleModel.ActivityInstance{
		autoStartInstance(201, scheduleModel.InstanceStatusPlanned, 13, 0, 14, 0),
	}}
	staffRepo := &autoStartStaffRepo{counts: map[int64]int{201: 1}}
	starter := &autoStartInstanceStarter{err: startErr}
	svc := NewAutoStartService(AutoStartDependencies{
		InstanceRepo:      repo,
		InstanceStaffRepo: staffRepo,
		InstanceService:   starter,
		ConflictDetector: func(context.Context, ConflictDependencies, *scheduleModel.ActivityInstance, *slog.Logger) []InstanceConflictWarning {
			return nil
		},
		Logger: slog.Default(),
	})

	result, err := svc.RunForTenant(context.Background(), now)

	require.Error(t, err)
	assert.ErrorIs(t, err, startErr)
	assert.Equal(t, 1, result.Checked)
	assert.Equal(t, 1, result.Failed)
	assert.Equal(t, 0, result.Started)
}

func autoStartInstance(id int64, status string, startHour, startMinute, endHour, endMinute int) *scheduleModel.ActivityInstance {
	inst := &scheduleModel.ActivityInstance{
		Date:      timezone.NewDate(2026, 4, 20),
		Title:     "Auto-start test",
		StartTime: time.Date(1, 1, 1, startHour, startMinute, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, endHour, endMinute, 0, 0, time.UTC),
		RoomID:    301,
		Status:    status,
	}
	inst.ID = id
	return inst
}

type autoStartInstanceRepo struct {
	instances []*scheduleModel.ActivityInstance
}

func (r *autoStartInstanceRepo) Create(context.Context, *scheduleModel.ActivityInstance) error {
	return nil
}
func (r *autoStartInstanceRepo) CreateTemplateBackedIfAbsent(context.Context, *scheduleModel.ActivityInstance) (bool, error) {
	return false, nil
}
func (r *autoStartInstanceRepo) FindByID(context.Context, any) (*scheduleModel.ActivityInstance, error) {
	return nil, nil
}
func (r *autoStartInstanceRepo) Update(context.Context, *scheduleModel.ActivityInstance) error {
	return nil
}
func (r *autoStartInstanceRepo) Delete(context.Context, any) error {
	return nil
}
func (r *autoStartInstanceRepo) List(context.Context, *base.QueryOptions) ([]*scheduleModel.ActivityInstance, error) {
	return nil, nil
}
func (r *autoStartInstanceRepo) FindByTenantAndDate(context.Context, timezone.Date) ([]*scheduleModel.ActivityInstance, error) {
	return r.instances, nil
}
func (r *autoStartInstanceRepo) FindByTenantAndDateRange(context.Context, timezone.Date, timezone.Date) ([]*scheduleModel.ActivityInstance, error) {
	return nil, nil
}
func (r *autoStartInstanceRepo) FindByActivityGroupAndDate(context.Context, int64, timezone.Date) ([]*scheduleModel.ActivityInstance, error) {
	return nil, nil
}
func (r *autoStartInstanceRepo) FindByActiveGroupID(context.Context, int64) (*scheduleModel.ActivityInstance, error) {
	return nil, nil
}
func (r *autoStartInstanceRepo) MarkCompleted(context.Context, int64, time.Time) error {
	return nil
}

type autoStartStaffRepo struct {
	counts       map[int64]int
	requestedIDs []int64
}

func (r *autoStartStaffRepo) Create(context.Context, *scheduleModel.InstanceStaff) error {
	return nil
}
func (r *autoStartStaffRepo) FindByID(context.Context, any) (*scheduleModel.InstanceStaff, error) {
	return nil, nil
}
func (r *autoStartStaffRepo) Update(context.Context, *scheduleModel.InstanceStaff) error {
	return nil
}
func (r *autoStartStaffRepo) Delete(context.Context, any) error {
	return nil
}
func (r *autoStartStaffRepo) List(context.Context, *base.QueryOptions) ([]*scheduleModel.InstanceStaff, error) {
	return nil, nil
}
func (r *autoStartStaffRepo) FindByInstanceID(context.Context, int64) ([]*scheduleModel.InstanceStaff, error) {
	return nil, nil
}
func (r *autoStartStaffRepo) FindByStaffAndDate(context.Context, int64, timezone.Date) ([]*scheduleModel.InstanceStaff, error) {
	return nil, nil
}
func (r *autoStartStaffRepo) CountNonAbsentByInstanceIDs(_ context.Context, ids []int64) (map[int64]int, error) {
	r.requestedIDs = append(r.requestedIDs, ids...)
	return r.counts, nil
}
func (r *autoStartStaffRepo) DeleteByInstanceID(context.Context, int64) error {
	return nil
}
func (r *autoStartStaffRepo) DeleteUpcomingByStaffID(context.Context, int64, timezone.Date) (int64, error) {
	return 0, nil
}

type autoStartInstanceStarter struct {
	startedIDs        []int64
	startedByStaffIDs []int64
	err               error
}

func (s *autoStartInstanceStarter) Start(_ context.Context, instanceID, startedByStaffID int64) (*StartInstanceResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.startedIDs = append(s.startedIDs, instanceID)
	s.startedByStaffIDs = append(s.startedByStaffIDs, startedByStaffID)
	return &StartInstanceResult{Instance: &scheduleModel.ActivityInstance{}, ActiveGroupID: 401}, nil
}
func (s *autoStartInstanceStarter) Complete(context.Context, int64) (*scheduleModel.ActivityInstance, error) {
	return nil, nil
}
func (s *autoStartInstanceStarter) Cancel(context.Context, int64) (*scheduleModel.ActivityInstance, error) {
	return nil, nil
}
func (s *autoStartInstanceStarter) DeleteCancelled(context.Context, int64) error {
	return nil
}
func (s *autoStartInstanceStarter) ReplanWeek(context.Context, timezone.Date, timezone.Date) (*ReplanWeekResult, error) {
	return nil, nil
}
func (s *autoStartInstanceStarter) Create(context.Context, CreateInstanceInput) (*scheduleModel.ActivityInstance, error) {
	return nil, nil
}
func (s *autoStartInstanceStarter) UpdatePlanned(context.Context, int64, UpdateInstanceInput) (*scheduleModel.ActivityInstance, error) {
	return nil, nil
}

// Stubs for the issue #585 cleanup refactor interface additions — unused by
// the auto-start tests.
func (r *autoStartInstanceRepo) CompleteActiveByActiveGroupIDs(context.Context, []int64, time.Time) (int64, error) {
	return 0, nil
}

func (r *autoStartInstanceRepo) CountWithOptions(context.Context, *base.QueryOptions) (int, error) {
	return 0, nil
}

func (r *autoStartInstanceRepo) OldestBefore(context.Context, string, *timezone.Date) (*timezone.Date, error) {
	return nil, nil
}

func (r *autoStartInstanceRepo) DeleteOlderThan(context.Context, string, timezone.Date) (int64, error) {
	return 0, nil
}

func (r *autoStartInstanceRepo) DeletePlannedNonSpontaneousInWindow(context.Context, timezone.Date, timezone.Date) (int64, error) {
	return 0, nil
}

func (r *autoStartInstanceRepo) UpdateColumns(context.Context, *scheduleModel.ActivityInstance, ...string) (int64, error) {
	return 0, nil
}
