// Package staffclock_test exercises the staff-clock HTTP boundary with real
// repositories and tenant-scoped transactions.
package staffclock_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	staffclockAPI "github.com/moto-nrw/project-phoenix/api/iot/staffclock"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type testContext struct {
	db       *bun.DB
	services *services.Factory
	device   *iotModels.Device
}

func setupTestContext(t *testing.T) *testContext {
	t.Helper()
	db, serviceFactory := testutil.SetupAPITest(t)
	testDevice := testpkg.CreateTestDevice(t, db, "staff-clock-device")
	return &testContext{db: db, services: serviceFactory, device: testDevice}
}

func (ctx *testContext) execute(t *testing.T, path string, body map[string]any) map[string]any {
	t.Helper()
	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", staffclockAPI.NewResource(ctx.services.StaffClock).Router())
	req := testutil.NewAuthenticatedRequest(t, http.MethodPost, path, body, testutil.WithDeviceContext(ctx.device))
	response := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusOK, response.Code, "response: %s", response.Body.String())
	return testutil.ParseJSONResponse(t, response.Body.Bytes())
}

func TestStaffClock_FullNFCFlow(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	staff := testpkg.CreateTestStaff(t, ctx.db, "Nora", "Kiosk")
	card := testpkg.CreateTestRFIDCard(t, ctx.db, "A1654BEEF")
	testpkg.LinkRFIDToStudent(t, ctx.db, staff.PersonID, card.ID)

	initial := ctx.execute(t, "/staff-clock/state", map[string]any{"rfid_tag": card.ID})
	initialData := initial["data"].(map[string]any)
	assert.Equal(t, "checked_out", initialData["state"])
	assert.Equal(t, "Nora Kiosk", initialData["staff_name"])

	checkedIn := ctx.execute(t, "/staff-clock", map[string]any{
		"rfid_tag": card.ID,
		"action":   "checkin",
		"status":   "present",
	})
	checkedInData := checkedIn["data"].(map[string]any)
	assert.Equal(t, "checked_in", checkedInData["state"])
	session := checkedInData["session"].(map[string]any)
	assert.Equal(t, activeModels.WorkSessionSourceNFC, session["source"])
	assert.Equal(t, activeModels.WorkSessionStatusPresent, session["status"])

	duplicateRouter := testutil.NewTenantRouter(ctx.db)
	duplicateRouter.Mount("/", staffclockAPI.NewResource(ctx.services.StaffClock).Router())
	duplicateReq := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/staff-clock", map[string]any{
		"rfid_tag": card.ID,
		"action":   "checkin",
		"status":   "present",
	}, testutil.WithDeviceContext(ctx.device))
	duplicateResponse := testutil.ExecuteRequest(duplicateRouter, duplicateReq)
	require.Equal(t, http.StatusConflict, duplicateResponse.Code, "response: %s", duplicateResponse.Body.String())
	assert.Equal(t, "invalid_staff_clock_state", testutil.ParseJSONResponse(t, duplicateResponse.Body.Bytes())["code"])

	onBreak := ctx.execute(t, "/staff-clock", map[string]any{
		"rfid_tag": card.ID,
		"action":   "break_start",
	})
	assert.Equal(t, "on_break", onBreak["data"].(map[string]any)["state"])

	resumed := ctx.execute(t, "/staff-clock", map[string]any{
		"rfid_tag": card.ID,
		"action":   "break_end",
	})
	assert.Equal(t, "checked_in", resumed["data"].(map[string]any)["state"])

	checkedOut := ctx.execute(t, "/staff-clock", map[string]any{
		"rfid_tag": card.ID,
		"action":   "checkout",
	})
	assert.Equal(t, "checked_out", checkedOut["data"].(map[string]any)["state"])

	// Reopening with a different work location must never silently rewrite
	// status. The first request returns a typed conflict; a reason-bearing
	// retry reopens and records the audited status change atomically.
	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", staffclockAPI.NewResource(ctx.services.StaffClock).Router())
	conflictReq := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/staff-clock", map[string]any{
		"rfid_tag": card.ID,
		"action":   "checkin",
		"status":   "home_office",
	}, testutil.WithDeviceContext(ctx.device))
	conflictResponse := testutil.ExecuteRequest(router, conflictReq)
	require.Equal(t, http.StatusConflict, conflictResponse.Code, "response: %s", conflictResponse.Body.String())
	assert.Equal(t, "reopen_status_conflict", testutil.ParseJSONResponse(t, conflictResponse.Body.Bytes())["code"])

	reopened := ctx.execute(t, "/staff-clock", map[string]any{
		"rfid_tag": card.ID,
		"action":   "checkin",
		"status":   "home_office",
		"reason":   "Nachmittags im Homeoffice",
	})
	reopenedSession := reopened["data"].(map[string]any)["session"].(map[string]any)
	assert.Equal(t, activeModels.WorkSessionStatusHomeOffice, reopenedSession["status"])
	assert.Equal(t, activeModels.WorkSessionSourceNFC, reopenedSession["source"])
}

func TestStaffClock_RejectsStudentCard(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	student := testpkg.CreateTestStudent(t, ctx.db, "Sam", "Schueler", "1a")
	card := testpkg.CreateTestRFIDCard(t, ctx.db, "B1654CAFE")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", staffclockAPI.NewResource(ctx.services.StaffClock).Router())
	req := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/staff-clock/state", map[string]any{"rfid_tag": card.ID}, testutil.WithDeviceContext(ctx.device))
	response := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusConflict, response.Code)
	parsed := testutil.ParseJSONResponse(t, response.Body.Bytes())
	assert.Equal(t, "rfid_tag_not_staff", parsed["code"])
}
