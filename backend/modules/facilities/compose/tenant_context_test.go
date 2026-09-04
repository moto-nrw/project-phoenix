package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestEveryOperationRejectsMissingTenantContext(t *testing.T) {
	t.Parallel()

	const id int64 = 1
	operations := []struct {
		name string
		call func(context.Context, *facilities.Module) error
	}{
		{name: "find room", call: func(ctx context.Context, module *facilities.Module) error {
			_, err := module.FindRoom(ctx, id)
			return err
		}},
		{name: "find room for update", call: func(ctx context.Context, module *facilities.Module) error {
			_, err := module.FindRoomForUpdate(ctx, id)
			return err
		}},
		{name: "find room by name", call: func(ctx context.Context, module *facilities.Module) error {
			_, err := module.FindRoomByName(ctx, "Igelraum")
			return err
		}},
		{name: "find toilet room", call: func(ctx context.Context, module *facilities.Module) error {
			_, err := module.FindToiletRoom(ctx, 0)
			return err
		}},
		{name: "list rooms", call: func(ctx context.Context, module *facilities.Module) error {
			_, err := module.ListRooms(ctx, facilities.RoomFilter{})
			return err
		}},
		{name: "list room page", call: func(ctx context.Context, module *facilities.Module) error {
			_, err := module.ListRoomsPage(ctx, facilities.RoomFilter{}, 0, 10)
			return err
		}},
		{name: "list rooms by ID", call: func(ctx context.Context, module *facilities.Module) error {
			_, err := module.ListRoomsByID(ctx, nil)
			return err
		}},
		{name: "lock rooms by ID", call: func(ctx context.Context, module *facilities.Module) error {
			_, err := module.LockRoomsByID(ctx, nil)
			return err
		}},
		{name: "create room", call: func(ctx context.Context, module *facilities.Module) error {
			_, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Igelraum"})
			return err
		}},
		{name: "update room", call: func(ctx context.Context, module *facilities.Module) error {
			_, err := module.UpdateRoom(ctx, facilities.UpdateRoom{ID: id, Name: "Igelraum"})
			return err
		}},
		{name: "delete room", call: func(ctx context.Context, module *facilities.Module) error { return module.DeleteRoom(ctx, id) }},
	}
	db := testpkg.SetupTestDB(t)
	checkOperations := func(name string, ctx context.Context) {
		for _, operation := range operations {
			t.Run(name+"/"+operation.name, func(t *testing.T) {
				observations := &observationLog{}
				module := buildModule(t, db, observations.record)

				err := operation.call(ctx, module)

				require.ErrorIs(t, err, tenant.ErrTenantRequired)
				require.Len(t, observations.operations(), 1)
			})
		}
	}
	checkOperations("shared connection", context.Background())
	checkOperations("unsupported transaction", tenant.WithTransactionForTest(context.Background(), struct{}{}))
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		checkOperations("tenant transaction", tenant.ContextWithoutTenant(ctx))
		return nil
	}))
	require.NoError(t, testpkg.WithAdminTx(t, context.Background(), db, func(ctx context.Context, _ bun.Tx) error {
		checkOperations("admin transaction", ctx)
		return nil
	}))
}
