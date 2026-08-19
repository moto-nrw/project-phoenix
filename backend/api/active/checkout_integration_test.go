// Package active_test tests the web checkout flow end-to-end (issue #895).
//
// The checkout handler must close the attendance row AND end any open room
// visit in one request transaction — never the partial state where attendance
// says "checked_out" while a visit stays open (the orphaned-visit deadlock).
package active_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeCheckoutRequest creates an HTTP request with JWT auth for the checkout endpoint
func makeCheckoutRequest(t *testing.T, studentID int64, token string) *http.Request {
	t.Helper()

	path := "/visits/student/" + strconv.FormatInt(studentID, 10) + "/checkout"
	req := httptest.NewRequest(http.MethodPost, path, nil)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return req
}

func TestCheckoutStudent_Integration(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	checkoutPermissions := []string{permissions.VisitsUpdate}

	t.Run("closes attendance and ends open visit in one request", func(t *testing.T) {
		handler := setupCheckinTestHandler(t, db)

		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Checkout", "Staff")
		student := testpkg.CreateTestStudent(t, db, "Checkout", "Student", "5a")
		device := testpkg.CreateTestDevice(t, db, "web-checkout-device-001")
		activity := testpkg.CreateTestActivityGroup(t, db, "web-checkout-test")
		room := testpkg.CreateTestRoom(t, db, "Web Checkout Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

		// Student is checked in AND in a room (open attendance + open visit).
		checkInTime := time.Now().Add(-1 * time.Hour)
		testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, checkInTime, nil)
		visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, checkInTime, nil)

		token := testpkg.CreateTestJWT(t, account.ID, checkoutPermissions)
		req := makeCheckoutRequest(t, student.ID, token)

		router := handler.Router()
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code, "Response body: %s", rr.Body.String())

		// Attendance row is closed...
		var records []*activeModels.Attendance
		err := db.NewSelect().
			Model(&records).
			ModelTableExpr(`active.attendance AS "attendance"`).
			Where(`"attendance".student_id = ?`, student.ID).
			Where(`"attendance".date = ?`, timezone.TodayDate()).
			Scan(context.Background())
		require.NoError(t, err)
		require.Len(t, records, 1)
		require.NotNil(t, records[0].CheckOutTime, "checkout must close the attendance row")

		// ...AND the visit is ended in the same request (issue #895).
		endedVisit := new(activeModels.Visit)
		err = db.NewSelect().
			Model(endedVisit).
			ModelTableExpr(`active.visits AS "visit"`).
			Where(`"visit".id = ?`, visit.ID).
			Scan(context.Background())
		require.NoError(t, err)
		require.NotNil(t, endedVisit.ExitTime, "checkout must end the open room visit (issue #895)")
	})

	t.Run("second checkout is a safe no-op", func(t *testing.T) {
		handler := setupCheckinTestHandler(t, db)

		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "DoubleCheckout", "Staff")
		student := testpkg.CreateTestStudent(t, db, "DoubleCheckout", "Student", "5b")
		device := testpkg.CreateTestDevice(t, db, "web-checkout-device-002")
		activity := testpkg.CreateTestActivityGroup(t, db, "web-double-checkout-test")
		room := testpkg.CreateTestRoom(t, db, "Web Double Checkout Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

		checkInTime := time.Now().Add(-1 * time.Hour)
		testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, checkInTime, nil)
		_ = testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, checkInTime, nil)

		token := testpkg.CreateTestJWT(t, account.ID, checkoutPermissions)
		router := handler.Router()

		// First checkout succeeds.
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, makeCheckoutRequest(t, student.ID, token))
		require.Equal(t, http.StatusOK, rr.Code, "Response body: %s", rr.Body.String())

		// Second checkout: student is no longer checked in → 404 per the
		// handler contract. Crucially it must NOT flip into a fresh check-in
		// (the pre-fix Toggle bug) and must NOT leave any open state behind.
		rr = httptest.NewRecorder()
		router.ServeHTTP(rr, makeCheckoutRequest(t, student.ID, token))
		assert.Equal(t, http.StatusNotFound, rr.Code, "Response body: %s", rr.Body.String())

		// Still exactly one attendance row, still closed.
		var records []*activeModels.Attendance
		err := db.NewSelect().
			Model(&records).
			ModelTableExpr(`active.attendance AS "attendance"`).
			Where(`"attendance".student_id = ?`, student.ID).
			Where(`"attendance".date = ?`, timezone.TodayDate()).
			Scan(context.Background())
		require.NoError(t, err)
		require.Len(t, records, 1, "second checkout must not create a new attendance row")
		require.NotNil(t, records[0].CheckOutTime)

		// No open visit either.
		openCount, err := db.NewSelect().
			ModelTableExpr(`active.visits AS "visit"`).
			Where(`"visit".student_id = ?`, student.ID).
			Where(`"visit".exit_time IS NULL`).
			Count(context.Background())
		require.NoError(t, err)
		assert.Zero(t, openCount, "no open visit may remain after checkout")
	})
}
