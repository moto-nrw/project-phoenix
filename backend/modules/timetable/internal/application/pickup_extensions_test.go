package application

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pickupExtensionTargetStore struct {
	ports.Store
	targets map[int64][]domain.GroupTarget
}

func (s pickupExtensionTargetStore) ListGroupTargets(_ context.Context, _ []int64) (map[int64][]domain.GroupTarget, domain.OperationStats, error) {
	return s.targets, domain.OperationStats{}, nil
}

type pickupExtensionTargetStudents struct {
	ports.StudentDirectory
	students []domain.TargetStudent
}

func (d pickupExtensionTargetStudents) ListEnrolledStudents(context.Context) ([]domain.TargetStudent, domain.OperationStats, error) {
	return d.students, domain.OperationStats{}, nil
}

type pickupExtensionInstanceStore struct {
	ports.Store
	instance  domain.ActivityInstance
	lockedID  int64
	exclusive bool
}

func (s *pickupExtensionInstanceStore) LockActivityInstance(_ context.Context, id int64, exclusive bool) (domain.ActivityInstance, bool, domain.OperationStats, error) {
	s.lockedID = id
	s.exclusive = exclusive
	return s.instance, true, domain.OperationStats{}, nil
}

func TestPickupExtensionInstanceLockRejectsLifecycleChange(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"cancelled", "completed"} {
		t.Run(status, func(t *testing.T) {
			store := &pickupExtensionInstanceStore{instance: domain.ActivityInstance{
				ID: 7, Date: "2099-03-03", Status: status,
			}}
			service := &Service{store: store}

			err := service.lockPickupExtensionInstance(context.Background(), domain.PickupExtensionInstance{
				ID: 7, Date: "2099-03-03",
			}, 0, &domain.OperationStats{})

			require.ErrorIs(t, err, domain.ErrPickupExtensionBlockGone)
			assert.Equal(t, int64(7), store.lockedID)
			assert.True(t, store.exclusive)
		})
	}
}

func TestPickupExtensionTargetsUsesTaskOperationalDateForEligibility(t *testing.T) {
	t.Parallel()
	class := "1a"
	service := &Service{
		store: pickupExtensionTargetStore{targets: map[int64][]domain.GroupTarget{
			7: {{TargetGroupType: "klasse", TargetSchoolClass: &class}},
		}},
		students: pickupExtensionTargetStudents{students: []domain.TargetStudent{{
			ID: 42, SchoolClass: class, EnrolledUntil: "2026-09-30",
		}}},
		today: func() string { return "2026-09-17" },
	}

	targets, err := service.pickupExtensionTargets(context.Background(), []domain.PickupExtensionBlock{{TaskID: 1, ID: 7}}, map[int64]domain.PickupExtensionTask{
		1: {ID: 1, StudentID: 42, EffectiveFrom: "2026-10-01"},
	}, &domain.OperationStats{})

	require.NoError(t, err)
	matched, found := targets[1][7]
	require.True(t, found)
	assert.False(t, matched, "a child whose care ends before the task takes effect is not a target")
}
