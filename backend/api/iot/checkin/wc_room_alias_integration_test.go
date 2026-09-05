package checkin_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func createWCRoomAliasIntegrationRoom(t *testing.T, db *bun.DB, name string) *facilities.Room {
	t.Helper()

	ctx := testpkg.Ctx(t)
	room := &facilities.Room{
		Name:     name,
		Building: "Test Building",
	}
	room.SetTenantID(testpkg.Tenant(t))

	err := db.NewInsert().
		Model(room).
		ModelTableExpr(`facilities.rooms`).
		Scan(ctx)
	require.NoError(t, err)

	return room
}

func TestDeviceCheckin_ToiletteRoomUsesWCAutoCreate(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	ctx := setupCheckinRoute(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "toilette-auto")

	staff := testpkg.CreateTestStaff(t, ctx.db, "Toilette", "Staff")

	student := testpkg.CreateTestStudent(t, ctx.db, "Toilette", "Student", "1a")

	tagID := fmt.Sprintf("TOI%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	room := createWCRoomAliasIntegrationRoom(t, ctx.db, constants.WCRoomAliasName)

	router := chi.NewRouter()
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequestForTest(t, router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "checked_in", data["action"])
	assert.Equal(t, constants.WCRoomAliasName, data["room_name"])

	activeGroup := new(active.Group)
	err := ctx.db.NewSelect().
		Model(activeGroup).
		ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".room_id = ?`, room.ID).
		OrderExpr(`"group".id DESC`).
		Limit(1).
		Scan(context.Background())
	require.NoError(t, err)
	assert.Nil(t, activeGroup.DeviceID)
}

func TestDeviceCheckin_ToiletteRoomDoesNotCreateDuplicateAlias(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	ctx := setupCheckinRoute(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "toilette-no-dup")

	staff := testpkg.CreateTestStaff(t, ctx.db, "Alias", "Staff")

	student := testpkg.CreateTestStudent(t, ctx.db, "Alias", "Student", "1b")

	tagID := fmt.Sprintf("TOD%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	room := createWCRoomAliasIntegrationRoom(t, ctx.db, constants.WCRoomAliasName)

	router := chi.NewRouter()
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequestForTest(t, router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	var aliasCount int
	err := ctx.db.NewSelect().
		TableExpr(`facilities.rooms AS "room"`).
		ColumnExpr(`COUNT(*)`).
		Where(`"room".tenant_id = ?`, testpkg.Tenant(t)).
		Where(`"room".name IN (?, ?)`, constants.WCRoomName, constants.WCRoomAliasName).
		Scan(context.Background(), &aliasCount)
	require.NoError(t, err)
	assert.Equal(t, 1, aliasCount)
}
