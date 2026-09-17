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

func TestPickupExtensionTargetsUsesCurrentDateForEligibility(t *testing.T) {
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
	assert.True(t, targets[1][7], "a child eligible today remains a target while the future schedule is selected separately")
}
