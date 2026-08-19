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

func cleanupWCRoomAliasIntegrationArtifacts(t *testing.T, db *bun.DB) {
	t.Helper()

	dbCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stmts := []string{
		fmt.Sprintf(`DELETE FROM active.attendance WHERE visit_id IN (SELECT v.id FROM active.visits v JOIN active.groups ag ON ag.id = v.active_group_id JOIN facilities.rooms r ON r.id = ag.room_id WHERE r.name IN ('%s', '%s'))`, constants.WCRoomName, constants.WCRoomAliasName),
		fmt.Sprintf(`DELETE FROM active.visits WHERE active_group_id IN (SELECT ag.id FROM active.groups ag JOIN facilities.rooms r ON r.id = ag.room_id WHERE r.name IN ('%s', '%s'))`, constants.WCRoomName, constants.WCRoomAliasName),
		fmt.Sprintf(`DELETE FROM active.group_supervisors WHERE group_id IN (SELECT ag.id FROM active.groups ag JOIN facilities.rooms r ON r.id = ag.room_id WHERE r.name IN ('%s', '%s'))`, constants.WCRoomName, constants.WCRoomAliasName),
		fmt.Sprintf(`DELETE FROM active.groups WHERE room_id IN (SELECT id FROM facilities.rooms WHERE name IN ('%s', '%s'))`, constants.WCRoomName, constants.WCRoomAliasName),
		fmt.Sprintf(`DELETE FROM activities.schedules WHERE activity_group_id IN (SELECT id FROM activities.groups WHERE name = '%s')`, constants.WCActivityName),
		fmt.Sprintf(`DELETE FROM activities.student_enrollments WHERE activity_group_id IN (SELECT id FROM activities.groups WHERE name = '%s')`, constants.WCActivityName),
		fmt.Sprintf(`DELETE FROM activities.groups WHERE name = '%s'`, constants.WCActivityName),
		fmt.Sprintf(`DELETE FROM activities.categories WHERE name = '%s'`, constants.WCCategoryName),
		fmt.Sprintf(`DELETE FROM facilities.rooms WHERE name IN ('%s', '%s')`, constants.WCRoomName, constants.WCRoomAliasName),
	}

	for _, stmt := range stmts {
		_, _ = db.ExecContext(dbCtx, stmt)
	}
}

func createWCRoomAliasIntegrationRoom(t *testing.T, db *bun.DB, name string) *facilities.Room {
	t.Helper()

	ctx := testpkg.TenantContext(1)
	room := &facilities.Room{
		Name:     name,
		Building: "Test Building",
	}
	room.SetTenantID(1)

	err := db.NewInsert().
		Model(room).
		ModelTableExpr(`facilities.rooms`).
		Scan(ctx)
	require.NoError(t, err)

	return room
}

func TestDeviceCheckin_ToiletteRoomUsesWCAutoCreate(t *testing.T) {
	ctx := setupTestContext(t)

	cleanupWCRoomAliasIntegrationArtifacts(t, ctx.db)
	defer cleanupWCRoomAliasIntegrationArtifacts(t, ctx.db)

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

	rr := testutil.ExecuteRequest(router, req)
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
	ctx := setupTestContext(t)

	cleanupWCRoomAliasIntegrationArtifacts(t, ctx.db)
	defer cleanupWCRoomAliasIntegrationArtifacts(t, ctx.db)

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

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	var aliasCount int
	err := ctx.db.NewSelect().
		TableExpr(`facilities.rooms AS "room"`).
		ColumnExpr(`COUNT(*)`).
		Where(`"room".tenant_id = ?`, 1).
		Where(`"room".name IN (?, ?)`, constants.WCRoomName, constants.WCRoomAliasName).
		Scan(context.Background(), &aliasCount)
	require.NoError(t, err)
	assert.Equal(t, 1, aliasCount)
}
