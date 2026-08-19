package active

import (
	"context"
	"errors"
	"testing"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capacityRoomRepository struct {
	facilityModels.RoomRepository
	room     *facilityModels.Room
	err      error
	lockedID int64
}

func TestEnsureCapacityForStudentMoveDoesNotCountSameRoomTransfers(t *testing.T) {
	t.Parallel()

	capacity := 1
	roomRepo := &capacityRoomRepository{room: &facilityModels.Room{
		Model:    base.Model{ID: 12},
		Name:     "Mensa",
		Capacity: &capacity,
	}}
	visitRepo := &mockVisitRepository{countActiveByRoomIDFunc: func(context.Context, int64) (int, error) {
		return 1, nil
	}}
	groupRepo := &groupRepoForActiveWrapperTest{groups: map[int64]*activeModels.Group{
		20: {Model: base.Model{ID: 20}, RoomID: 12},
	}}
	svc := &service{ServiceDependencies: ServiceDependencies{
		RoomRepo:  roomRepo,
		VisitRepo: visitRepo,
		GroupRepo: groupRepo,
	}}
	targetGroup := &activeModels.Group{Model: base.Model{ID: 21}, RoomID: 12}

	err := svc.ensureCapacityForStudentMove(
		context.Background(),
		targetGroup,
		[]int64{30},
		map[int64]*activeModels.Attendance{30: {}},
		map[int64]*activeModels.Visit{30: {ActiveGroupID: 20}},
	)

	require.NoError(t, err)
	assert.Zero(t, roomRepo.lockedID)
}

func (r *capacityRoomRepository) FindByIDForUpdate(_ context.Context, id int64) (*facilityModels.Room, error) {
	r.lockedID = id
	return r.room, r.err
}

func TestEnsureRoomCapacity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("allows rooms without a limit", func(t *testing.T) {
		roomRepo := &capacityRoomRepository{room: &facilityModels.Room{Model: base.Model{ID: 12}, Name: "Turnhalle"}}
		visitRepo := &mockVisitRepository{countActiveByRoomIDFunc: func(context.Context, int64) (int, error) {
			return 200, nil
		}}
		svc := &service{ServiceDependencies: ServiceDependencies{RoomRepo: roomRepo, VisitRepo: visitRepo}}

		err := svc.ensureRoomCapacity(ctx, 12, 30)

		require.NoError(t, err)
		assert.Equal(t, int64(12), roomRepo.lockedID)
	})

	t.Run("allows filling the last available places", func(t *testing.T) {
		capacity := 43
		roomRepo := &capacityRoomRepository{room: &facilityModels.Room{Model: base.Model{ID: 12}, Name: "Mensa", Capacity: &capacity}}
		visitRepo := &mockVisitRepository{countActiveByRoomIDFunc: func(context.Context, int64) (int, error) {
			return 40, nil
		}}
		svc := &service{ServiceDependencies: ServiceDependencies{RoomRepo: roomRepo, VisitRepo: visitRepo}}

		err := svc.ensureRoomCapacity(ctx, 12, 3)

		require.NoError(t, err)
	})

	t.Run("rejects a request that would exceed the limit", func(t *testing.T) {
		capacity := 43
		roomRepo := &capacityRoomRepository{room: &facilityModels.Room{Model: base.Model{ID: 12}, Name: "Mensa", Capacity: &capacity}}
		visitRepo := &mockVisitRepository{countActiveByRoomIDFunc: func(context.Context, int64) (int, error) {
			return 42, nil
		}}
		svc := &service{ServiceDependencies: ServiceDependencies{RoomRepo: roomRepo, VisitRepo: visitRepo}}

		err := svc.ensureRoomCapacity(ctx, 12, 2)

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrRoomCapacityExceeded)
		var capacityErr *RoomCapacityError
		require.True(t, errors.As(err, &capacityErr))
		assert.Equal(t, 42, capacityErr.CurrentOccupancy)
		assert.Equal(t, 43, capacityErr.MaxCapacity)
	})
}
