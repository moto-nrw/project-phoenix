package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	internalports "github.com/moto-nrw/project-phoenix/modules/timetable/internal/ports"
)

var errPickupRetryable = errors.New("retryable serialization failure")

// pickupRetryTransaction replays the callback once after a retryable
// failure, like the tenant unit of work without an ambient transaction.
type pickupRetryTransaction struct{}

func (pickupRetryTransaction) RunWrite(ctx context.Context, _ bool, callback func(context.Context) error) error {
	err := callback(ctx)
	if errors.Is(err, errPickupRetryable) {
		return callback(ctx)
	}
	return err
}

type pickupRetryStore struct {
	internalports.Store
	task    domain.PickupExtensionTask
	block   domain.PickupExtensionBlock
	deletes int
}

func (s *pickupRetryStore) FindPickupExtensionTaskForUpdate(context.Context, int64) (domain.PickupExtensionTask, bool, domain.OperationStats, error) {
	return s.task, true, domain.OperationStats{}, nil
}

func (s *pickupRetryStore) ListPickupExtensionDayBlocks(context.Context, []domain.PickupExtensionTask) ([]domain.PickupExtensionBlock, domain.OperationStats, error) {
	return []domain.PickupExtensionBlock{s.block}, domain.OperationStats{}, nil
}

func (s *pickupRetryStore) LockActivityInstance(_ context.Context, id int64, _ bool) (timetable.ActivityInstance, bool, domain.OperationStats, error) {
	return timetable.ActivityInstance{
		ID: id, Date: s.task.Date.String(), Status: domain.InstanceStatusPlanned, StartTime: s.block.StartTime, EndTime: s.block.EndTime,
	}, true, domain.OperationStats{}, nil
}

func (s *pickupRetryStore) EnsureInstanceStudent(_ context.Context, instanceID, studentID int64) (timetable.InstanceStudent, bool, domain.OperationStats, error) {
	return timetable.InstanceStudent{InstanceID: instanceID, StudentID: studentID}, true, domain.OperationStats{}, nil
}

func (s *pickupRetryStore) DeletePickupExtensionTask(context.Context, int64) (domain.OperationStats, error) {
	s.deletes++
	if s.deletes == 1 {
		return domain.OperationStats{}, errPickupRetryable
	}
	return domain.OperationStats{}, nil
}

type pickupRetrySessions struct{ internalports.SessionFacts }

func (pickupRetrySessions) ExecutionFacts(context.Context, []int64, []int64) ([]int64, []int64, error) {
	return nil, nil, nil
}

// TestResolvePickupExtensionRetryReportsEachInstanceOnce replays the resolve
// after a retryable failure at the task delete (#5816 review): the
// resolution names the assigned block once, not once per attempt.
func TestResolvePickupExtensionRetryReportsEachInstanceOnce(t *testing.T) {
	t.Parallel()
	exceptionID := int64(61)
	store := &pickupRetryStore{
		task: domain.PickupExtensionTask{
			ID: 31, StudentID: 71, PickupExceptionID: &exceptionID, Date: "2099-03-03",
			PreviousPickup: "14:45", Pickup: "16:00",
		},
		block: domain.PickupExtensionBlock{TaskID: 31, ID: 41, StartTime: "14:45:00", EndTime: "16:00:00", ParticipantIDs: []int64{51}},
	}
	service := &Service{
		store: store, tx: pickupRetryTransaction{}, sessions: pickupRetrySessions{},
		today: func() string { return "2099-01-01" }, observe: func(internalports.Observation) {},
	}

	resolved, err := service.ResolvePickupExtension(context.Background(), 31, []int64{41})
	require.NoError(t, err)
	assert.Equal(t, 2, store.deletes, "the first attempt failed and was replayed")
	assert.Equal(t, []int64{41}, resolved.InstanceIDs)
	require.Len(t, resolved.Instances, 1)
	assert.Equal(t, int64(41), resolved.Instances[0].ID)
}
