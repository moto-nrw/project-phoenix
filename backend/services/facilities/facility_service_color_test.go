package facilities_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/facilities"
	facilitiesSvc "github.com/moto-nrw/project-phoenix/services/facilities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// uniqueRoomName is duplicated here (instead of imported) so the color suite
// stays independent of the existing facility_service_test.go ordering. Random
// enough to avoid colliding inside the same test run.
func uniqueRoomName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// TestFacilitiesService_CreateRoom_RejectsReservedColor confirms that a hex
// claimed by the frontend status palette gets rejected before the row is
// written. Mirrors the user-facing behaviour: form pickers offering, e.g.,
// the SCHOOLYARD orange should fail with the German message even if a sneaky
// caller bypasses the picker and posts the hex directly.
func TestFacilitiesService_CreateRoom_RejectsReservedColor(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := setupFacilitiesService(t, db)
	tenantID := createFacilityTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)

	cases := []struct {
		label    string
		hex      string
		expected error
	}{
		{"OTHER_ROOM blue", "#5080D8", facilitiesSvc.ErrColorReserved},
		{"SCHOOLYARD orange", "#F78C10", facilitiesSvc.ErrColorReserved},
		{"TRANSIT magenta", "#D946EF", facilitiesSvc.ErrColorReserved},
		{"GROUP_ROOM green", "#83CD2D", facilitiesSvc.ErrColorReserved},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			color := tc.hex
			room := &facilities.Room{
				Name:  uniqueRoomName("Reserved"),
				Color: &color,
			}

			err := service.CreateRoom(ctx, room)
			require.Error(t, err)
			require.True(t, errors.Is(err, tc.expected),
				"expected ErrColorReserved, got %v", err)
		})
	}
}

// TestFacilitiesService_CreateRoom_AcceptsCustomColor sanity-checks the happy
// path: a color that is neither reserved nor in use gets persisted as-is.
func TestFacilitiesService_CreateRoom_AcceptsCustomColor(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := setupFacilitiesService(t, db)
	tenantID := createFacilityTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)

	color := "#A3D977"
	room := &facilities.Room{
		Name:  uniqueRoomName("CustomColor"),
		Color: &color,
	}

	err := service.CreateRoom(ctx, room)
	require.NoError(t, err)

	retrieved, err := service.GetRoom(ctx, room.ID)
	require.NoError(t, err)
	require.NotNil(t, retrieved.Color)
	assert.Equal(t, "#A3D977", *retrieved.Color)
}

// TestFacilitiesService_UpdateRoom_RejectsDuplicateColor exercises the partial
// unique index from migration 1.15.45. Two rooms in the same school cannot
// share a color; the service translates the 23505 to ErrColorAlreadyInUse so
// the frontend toast can surface the German message instead of a 500.
func TestFacilitiesService_UpdateRoom_RejectsDuplicateColor(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := setupFacilitiesService(t, db)
	tenantID := createFacilityTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)

	// ARRANGE — Room A claims the color first.
	colorA := "#A3D977"
	roomA := &facilities.Room{Name: uniqueRoomName("RoomA"), Color: &colorA}
	require.NoError(t, service.CreateRoom(ctx, roomA))

	// Second room created without color (so the create succeeds)
	roomB := &facilities.Room{Name: uniqueRoomName("RoomB")}
	require.NoError(t, service.CreateRoom(ctx, roomB))

	// ACT — try to update Room B to the same color as Room A.
	colorBAttempt := colorA
	roomB.Color = &colorBAttempt
	err := service.UpdateRoom(ctx, roomB)

	// ASSERT — surface the German conflict, not a raw DB error.
	require.Error(t, err)
	require.True(t, errors.Is(err, facilitiesSvc.ErrColorAlreadyInUse),
		"expected ErrColorAlreadyInUse, got %v", err)
}

// TestFacilitiesService_UpdateRoom_BlocksColorOnToiletRooms catches a
// specific regression: someone removes the toilet-room color check and "WC"
// silently gets a yellow badge — configuring a colour for a room that has no
// badge of its own.
//
// The Schulhof deliberately left this set with #2405 — see
// TestFacilitiesService_UpdateRoom_AllowsColorOnSchulhof below.
//
// We use createRoomWithExactName to produce actual system-room records
// (constants.IsWCRoomName matches the WC aliases by exact name);
// CreateTestRoom would suffix a timestamp and dodge the check.
func TestFacilitiesService_UpdateRoom_BlocksColorOnToiletRooms(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := setupFacilitiesService(t, db)
	tenantID := createFacilityTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)

	t.Run("rejects color change on WC", func(t *testing.T) {
		room := createRoomWithExactName(t, db, tenantID, "WC")

		newColor := "#A3D977"
		room.Color = &newColor
		err := service.UpdateRoom(ctx, room)

		require.Error(t, err)
		require.True(t, errors.Is(err, facilitiesSvc.ErrSystemRoomProtected),
			"expected ErrSystemRoomProtected, got %v", err)
	})

	t.Run("allows benign updates that do not touch color", func(t *testing.T) {
		// Regression guard: the toilet-room block must trigger on the *color
		// field*, not on every update. Editing Capacity must still succeed,
		// otherwise admins lose the ability to change any non-name field.
		room := createRoomWithExactName(t, db, tenantID, "WC")

		newCapacity := 200
		room.Capacity = &newCapacity
		// Color stays nil — no change in the protected field.
		require.NoError(t, service.UpdateRoom(ctx, room))
	})

	t.Run("allows clearing a non-existent color (no-op)", func(t *testing.T) {
		// If a toilet room somehow had Color=nil already, sending nil again
		// should not be flagged as a change. equalStringPtr handles this —
		// without it, every update on a colorless toilet room would 403.
		room := createRoomWithExactName(t, db, tenantID, "WC")

		room.Color = nil // unchanged
		room.Building = "Updated"
		require.NoError(t, service.UpdateRoom(ctx, room))
	})

	t.Run("benign update on a toilet room with leftover legacy color succeeds", func(t *testing.T) {
		// Reproduces the review-flagged #2 regression: production system-room
		// rows almost certainly carried "#4F46E5" from the rooms.config bug.
		// The frontend strips the color picker for toilet rooms, so a
		// capacity-only update sends room.Color = nil. Earlier code treated
		// nil != &"#4F46E5" as a forbidden colour change and 403'd every such
		// edit. Now the service preserves the existing colour when the
		// request omits it.
		room := createRoomWithExactName(t, db, tenantID, "WC")

		// Bypass Validate() (would reject the reserved colour) by writing
		// the legacy hex directly with a raw UPDATE — this matches what a
		// pre-migration production DB looks like.
		legacy := "#4F46E5"
		_, err := db.NewUpdate().
			TableExpr("facilities.rooms").
			Set("color = ?", legacy).
			Where("id = ?", room.ID).
			Exec(ctx)
		require.NoError(t, err)

		// Reload to mirror what the handler does (FindByID then mutate).
		fresh, err := service.GetRoom(ctx, room.ID)
		require.NoError(t, err)
		require.NotNil(t, fresh.Color)
		require.Equal(t, "#4F46E5", *fresh.Color)

		// Frontend strips the color field -> request body has no colour ->
		// handler sets room.Color = nil. Capacity bumps must still go
		// through.
		fresh.Color = nil
		newCap := 250
		fresh.Capacity = &newCap

		require.NoError(t, service.UpdateRoom(ctx, fresh),
			"benign capacity update on WC must succeed even when the "+
				"row carries a legacy color and the request omits the field")

		// Existing colour should be preserved (defensive: confirms we
		// silently kept the legacy value rather than nulling it).
		retrieved, err := service.GetRoom(ctx, room.ID)
		require.NoError(t, err)
		require.NotNil(t, retrieved.Color)
		assert.Equal(t, "#4F46E5", *retrieved.Color)
		assert.Equal(t, 250, *retrieved.Capacity)
	})
}

// TestFacilitiesService_UpdateRoom_AllowsColorOnSchulhof pins the #2405
// business-rule change: the Schulhof is still a protected system room (no
// rename, no delete) but its colour follows the ordinary room rules, because
// schools colour-code their rooms and tablets and need the yard in that
// scheme.
//
// The old behaviour — a blanket ErrSystemRoomProtected on any Schulhof colour
// change — is what this test replaces; see
// TestFacilitiesService_UpdateRoom_BlocksColorOnToiletRooms for the part of
// the rule that survived.
func TestFacilitiesService_UpdateRoom_AllowsColorOnSchulhof(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := setupFacilitiesService(t, db)
	tenantID := createFacilityTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)

	t.Run("accepts a valid color", func(t *testing.T) {
		room := createRoomWithExactName(t, db, tenantID, "Schulhof")

		newColor := "#A3D977"
		room.Color = &newColor
		require.NoError(t, service.UpdateRoom(ctx, room))

		retrieved, err := service.GetRoom(ctx, room.ID)
		require.NoError(t, err)
		require.NotNil(t, retrieved.Color)
		assert.Equal(t, "#A3D977", *retrieved.Color)
	})

	t.Run("allows clearing the color back to the orange default", func(t *testing.T) {
		// "No colour" is the state the orange Schulhof default renders for,
		// so the picker's reset button has to be able to reach it.
		room := createRoomWithExactName(t, db, tenantID, "Schulhof")

		newColor := "#B3D977"
		room.Color = &newColor
		require.NoError(t, service.UpdateRoom(ctx, room))

		fresh, err := service.GetRoom(ctx, room.ID)
		require.NoError(t, err)
		fresh.Color = nil
		require.NoError(t, service.UpdateRoom(ctx, fresh))

		retrieved, err := service.GetRoom(ctx, room.ID)
		require.NoError(t, err)
		assert.Nil(t, retrieved.Color)
	})

	t.Run("still rejects a reserved status color", func(t *testing.T) {
		// The Schulhof is not exempt from the palette rules — including the
		// SCHOOLYARD orange itself, which stays reserved as a status hex.
		room := createRoomWithExactName(t, db, tenantID, "Schulhof")

		reserved := "#F78C10"
		room.Color = &reserved
		err := service.UpdateRoom(ctx, room)

		require.Error(t, err)
		require.True(t, errors.Is(err, facilitiesSvc.ErrColorReserved),
			"expected ErrColorReserved, got %v", err)
	})

	t.Run("still rejects a malformed color", func(t *testing.T) {
		room := createRoomWithExactName(t, db, tenantID, "Schulhof")

		bad := "not-a-hex"
		room.Color = &bad
		require.Error(t, service.UpdateRoom(ctx, room))
	})

	t.Run("still rejects a color another room already uses", func(t *testing.T) {
		// The per-tenant uniqueness index applies to the yard like any
		// other room.
		taken := "#C3D977"
		other := createRoomWithExactName(t, db, tenantID, uniqueRoomName("Farbbelegt"))
		other.Color = &taken
		require.NoError(t, service.UpdateRoom(ctx, other))

		room := createRoomWithExactName(t, db, tenantID, "Schulhof")
		room.Color = &taken
		err := service.UpdateRoom(ctx, room)

		require.Error(t, err)
		require.True(t, errors.Is(err, facilitiesSvc.ErrColorAlreadyInUse),
			"expected ErrColorAlreadyInUse, got %v", err)
	})

	t.Run("still refuses to rename the Schulhof", func(t *testing.T) {
		room := createRoomWithExactName(t, db, tenantID, "Schulhof")

		room.Name = "Hinterhof"
		err := service.UpdateRoom(ctx, room)

		require.Error(t, err)
		require.True(t, errors.Is(err, facilitiesSvc.ErrSystemRoomProtected),
			"expected ErrSystemRoomProtected, got %v", err)
	})

	t.Run("still refuses to delete the Schulhof", func(t *testing.T) {
		room := createRoomWithExactName(t, db, tenantID, "Schulhof")

		err := service.DeleteRoom(ctx, room.ID)

		require.Error(t, err)
		require.True(t, errors.Is(err, facilitiesSvc.ErrSystemRoomProtected),
			"expected ErrSystemRoomProtected, got %v", err)
	})
}

// TestFacilitiesService_UpdateRoom_AllowsClearingColor confirms a room can
// drop its color back to NULL even after a custom one was set. The partial
// unique index ignores NULLs, so multiple rooms can hold "no color" — the
// blue fallback in the badge renderer covers them.
func TestFacilitiesService_UpdateRoom_AllowsClearingColor(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := setupFacilitiesService(t, db)
	tenantID := createFacilityTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)

	// ARRANGE — start with a custom color
	color := "#1ABC9C"
	room := &facilities.Room{Name: uniqueRoomName("ClearColor"), Color: &color}
	require.NoError(t, service.CreateRoom(ctx, room))

	// ACT — clear to nil (frontend's "Zurücksetzen" button)
	room.Color = nil
	require.NoError(t, service.UpdateRoom(ctx, room))

	// ASSERT — DB shows null
	retrieved, err := service.GetRoom(ctx, room.ID)
	require.NoError(t, err)
	assert.Nil(t, retrieved.Color, "color should be cleared after update")
}
