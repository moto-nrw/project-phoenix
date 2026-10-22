package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func buildRoomCategories(t *testing.T, db *bun.DB) facilities.RoomCategories {
	t.Helper()
	categories, err := NewRoomCategories(Dependencies{DB: db, Observe: func(Observation) {}})
	require.NoError(t, err)
	return categories
}

func TestNewRoomCategoriesRequiresDatabaseAndObserver(t *testing.T) {
	t.Parallel()
	_, err := NewRoomCategories(Dependencies{})
	require.Error(t, err)
}

func TestRoomsShowTheirCategoryColorUnlessTheyHaveTheirOwn(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	categories := buildRoomCategories(t, db)
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))

	_, err := categories.CreateRoomCategory(ctx, facilities.RoomCategoryInput{Name: "Sport", Color: "#827710"})
	require.NoError(t, err)

	category := "sport "
	gym, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Sporthalle", Category: &category})
	require.NoError(t, err)
	require.NotNil(t, gym.CategoryColor, "a write returns the category color too")
	assert.Equal(t, "#827710", *gym.BadgeColor())

	own := "#123456"
	sportsHall, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Bewegungsraum", Category: &category, Color: &own})
	require.NoError(t, err)

	rooms, err := module.ListRoomsByID(ctx, []int64{gym.ID, sportsHall.ID})
	require.NoError(t, err)
	badges := map[int64]string{}
	for _, room := range rooms {
		badges[room.ID] = *room.BadgeColor()
	}
	assert.Equal(t, "#827710", badges[gym.ID], "the category is matched by name, ignoring case and spaces")
	assert.Equal(t, own, badges[sportsHall.ID], "the room's own color wins")

	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		locked, lockErr := module.FindRoomForUpdate(txCtx, gym.ID)
		require.NoError(t, lockErr, "locking reads lock only the room, not the joined catalog row")
		assert.Equal(t, "#827710", *locked.BadgeColor())
		return nil
	}))

	unknown := "Werkstatt"
	workshop, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Werkraum", Category: &unknown})
	require.NoError(t, err)
	assert.Nil(t, workshop.BadgeColor(), "a category outside the catalog leaves the badge default")
}

func TestRoomCategoryRenameMovesItsRooms(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	categories := buildRoomCategories(t, db)
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))

	created, err := categories.CreateRoomCategory(ctx, facilities.RoomCategoryInput{Name: " Themenraum ", Color: "6a1b90"})
	require.NoError(t, err)
	assert.Equal(t, "Themenraum", created.Name)
	assert.Equal(t, "#6A1B90", created.Color)

	name := "Themenraum"
	room, err := module.CreateRoom(ctx, facilities.CreateRoom{Name: "Musikraum", Category: &name})
	require.NoError(t, err)

	updated, err := categories.UpdateRoomCategory(ctx, created.ID, facilities.RoomCategoryInput{Name: "Kreativ", Color: "#8D3B1E"})
	require.NoError(t, err)
	assert.Equal(t, "Kreativ", updated.Name)

	found, err := module.FindRoom(ctx, room.ID)
	require.NoError(t, err)
	require.NotNil(t, found.Category)
	assert.Equal(t, "Kreativ", *found.Category)
	assert.Equal(t, "#8D3B1E", *found.BadgeColor())

	list, err := categories.ListRoomCategories(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, 1, list[0].RoomCount)
}

func TestRoomCategoryWritesRejectInvalidInput(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	categories := buildRoomCategories(t, db)
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))

	_, err := categories.CreateRoomCategory(ctx, facilities.RoomCategoryInput{Name: "Sport", Color: "#827710"})
	require.NoError(t, err)

	_, err = categories.CreateRoomCategory(ctx, facilities.RoomCategoryInput{Name: "sport", Color: "#374750"})
	require.ErrorIs(t, err, facilities.ErrDuplicateRoomCategory)
	_, err = categories.CreateRoomCategory(ctx, facilities.RoomCategoryInput{Name: "Mensa", Color: "#83CD2D"})
	require.ErrorIs(t, err, facilities.ErrRoomColorReserved, "the own-group green stays a status color")
	_, err = categories.CreateRoomCategory(ctx, facilities.RoomCategoryInput{Name: " ", Color: "#374750"})
	require.ErrorIs(t, err, facilities.ErrInvalidRoom)
	_, err = categories.CreateRoomCategory(ctx, facilities.RoomCategoryInput{Name: "Mensa"})
	require.ErrorIs(t, err, facilities.ErrInvalidRoom, "a category needs a color")
	removed, err := categories.CreateRoomCategory(ctx, facilities.RoomCategoryInput{Name: "Mensa", Color: "#374750"})
	require.NoError(t, err)
	require.NoError(t, categories.DeleteRoomCategory(ctx, removed.ID))
	_, err = categories.UpdateRoomCategory(ctx, removed.ID, facilities.RoomCategoryInput{Name: "Mensa", Color: "#374750"})
	require.ErrorIs(t, err, facilities.ErrRoomCategoryNotFound)
}

func TestRoomCategoryDeleteKeepsCategoriesRoomsStillUse(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	categories := buildRoomCategories(t, db)
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))

	used, err := categories.CreateRoomCategory(ctx, facilities.RoomCategoryInput{Name: "Gruppenraum", Color: "#283590"})
	require.NoError(t, err)
	unused, err := categories.CreateRoomCategory(ctx, facilities.RoomCategoryInput{Name: "Sport", Color: "#827710"})
	require.NoError(t, err)
	name := "Gruppenraum"
	_, err = module.CreateRoom(ctx, facilities.CreateRoom{Name: "OGS-Raum 1", Category: &name})
	require.NoError(t, err)

	require.ErrorIs(t, categories.DeleteRoomCategory(ctx, used.ID), facilities.ErrRoomCategoryInUse)
	require.NoError(t, categories.DeleteRoomCategory(ctx, unused.ID))
	require.ErrorIs(t, categories.DeleteRoomCategory(ctx, unused.ID), facilities.ErrRoomCategoryNotFound)

	list, err := categories.ListRoomCategories(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Gruppenraum", list[0].Name)
}

func TestSeedDefaultRoomCategoriesKeepsExistingOnes(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	categories := buildRoomCategories(t, db)
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t))

	own, err := categories.CreateRoomCategory(ctx, facilities.RoomCategoryInput{Name: "sport", Color: "#455A64"})
	require.NoError(t, err)

	require.NoError(t, categories.SeedDefaultRoomCategories(ctx))
	require.NoError(t, categories.SeedDefaultRoomCategories(ctx), "a second run changes nothing")

	list, err := categories.ListRoomCategories(ctx)
	require.NoError(t, err)
	require.Len(t, list, len(facilities.DefaultRoomCategories))
	for _, category := range list {
		if category.ID == own.ID {
			assert.Equal(t, "#455A64", category.Color, "an existing category keeps its color")
		}
	}

	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	otherCtx := testpkg.WithTestTenantRuntime(t, testpkg.TenantContext(otherTenant))
	foreign, err := categories.ListRoomCategories(otherCtx)
	require.NoError(t, err)
	assert.Empty(t, foreign, "the catalog is per school")
}
