// Package active_test tests the checkin stale-visit self-heal (issue #895).
//
// An orphaned visit (attendance "checked_out" but visit still open) used to
// deadlock a student: checkin returned 409, checkout returned 404. The
// checkin validation now heals the orphan (ends the stale visit) and lets the
// check-in proceed, while genuine conflicts keep their 409 responses.
package active_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/active"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckinStudent_SelfHealsOrphanVisit(t *testing.T) {
	t.Parallel()
	setupViperForTest()

	db := testpkg.SetupTestDB(t)

	checkinPermissions := []string{permissions.VisitsUpdate}

	t.Run("orphaned visit is healed and checkin proceeds", func(t *testing.T) {
		handler := setupCheckinTestHandler(t, db)

		webDevice := testpkg.EnsureWebManualDevice(t, db)
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "SelfHeal", "Teacher")
		educationGroup := testpkg.CreateTestEducationGroup(t, db, "Self Heal Group")
		testpkg.CreateTestGroupTeacher(t, db, educationGroup.ID, teacher.ID)

		student := testpkg.CreateTestStudent(t, db, "SelfHeal", "Student", "5a")
		testpkg.AssignStudentToGroup(t, db, student.ID, educationGroup.ID)

		// Two rooms: the orphaned visit is "stuck" in one, the new checkin
		// targets the other.
		orphanActivity := testpkg.CreateTestActivityGroup(t, db, "self-heal-orphan")
		orphanRoom := testpkg.CreateTestRoom(t, db, "Self Heal Orphan Room")
		orphanGroup := testpkg.CreateTestActiveGroup(t, db, orphanActivity.ID, orphanRoom.ID)
		targetActivity := testpkg.CreateTestActivityGroup(t, db, "self-heal-target")
		targetRoom := testpkg.CreateTestRoom(t, db, "Self Heal Target Room")
		targetGroup := testpkg.CreateTestActiveGroup(t, db, targetActivity.ID, targetRoom.ID)

		// The issue #895 deadlock state: attendance closed, visit still open.
		checkInTime := time.Now().Add(-2 * time.Hour)
		checkOutTime := time.Now().Add(-30 * time.Minute)
		testpkg.CreateTestAttendance(t, db, student.ID, teacher.Staff.ID, webDevice.ID, checkInTime, &checkOutTime)
		orphanVisit := testpkg.CreateTestVisit(t, db, student.ID, orphanGroup.ID, checkInTime, nil)

		token := testpkg.CreateTestJWT(t, account.ID, checkinPermissions)
		body := active.CheckinRequest{ActiveGroupID: targetGroup.ID}
		req := makeCheckinRequest(t, student.ID, body, token)

		router := handler.Router()
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		// Pre-fix this was a 409 deadlock; now the orphan is healed.
		require.Equal(t, http.StatusOK, rr.Code, "Response body: %s", rr.Body.String())

		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "data should be a map")
		assert.Equal(t, "checked_in", data["action"])
		assert.NotZero(t, data["visit_id"])

		// The orphaned visit is closed...
		healedVisit := new(activeModels.Visit)
		err := db.NewSelect().
			Model(healedVisit).
			ModelTableExpr(`active.visits AS "visit"`).
			Where(`"visit".id = ?`, orphanVisit.ID).
			Scan(context.Background())
		require.NoError(t, err)
		require.NotNil(t, healedVisit.ExitTime, "orphaned visit must be ended during checkin (issue #895)")

		// ...and exactly one new open visit exists, in the target group.
		var openVisits []*activeModels.Visit
		err = db.NewSelect().
			Model(&openVisits).
			ModelTableExpr(`active.visits AS "visit"`).
			Where(`"visit".student_id = ?`, student.ID).
			Where(`"visit".exit_time IS NULL`).
			Scan(context.Background())
		require.NoError(t, err)
		require.Len(t, openVisits, 1)
		assert.Equal(t, targetGroup.ID, openVisits[0].ActiveGroupID)

		// Cleanup the visit created by the request.
	})

	t.Run("capacity rejection rolls back orphan cleanup", func(t *testing.T) {
		handler := setupCheckinTestHandler(t, db)

		webDevice := testpkg.EnsureWebManualDevice(t, db)
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "CapacityRollback", "Teacher")
		educationGroup := testpkg.CreateTestEducationGroup(t, db, "Capacity Rollback Group")
		testpkg.CreateTestGroupTeacher(t, db, educationGroup.ID, teacher.ID)
		student := testpkg.CreateTestStudent(t, db, "CapacityRollback", "Student", "5a")
		testpkg.AssignStudentToGroup(t, db, student.ID, educationGroup.ID)

		orphanActivity := testpkg.CreateTestActivityGroup(t, db, "capacity-rollback-orphan")
		orphanRoom := testpkg.CreateTestRoom(t, db, "Capacity Rollback Orphan Room")
		orphanGroup := testpkg.CreateTestActiveGroup(t, db, orphanActivity.ID, orphanRoom.ID)
		targetActivity := testpkg.CreateTestActivityGroup(t, db, "capacity-rollback-target")
		targetRoom := testpkg.CreateTestRoom(t, db, "Capacity Rollback Target Room")
		capacity := 1
		targetRoom.Capacity = &capacity
		_, err := db.NewUpdate().Model(targetRoom).Column("capacity").WherePK().Exec(t.Context())
		require.NoError(t, err)
		targetGroup := testpkg.CreateTestActiveGroup(t, db, targetActivity.ID, targetRoom.ID)
		occupant := testpkg.CreateTestStudent(t, db, "CapacityRollback", "Occupant", "5a")
		_ = testpkg.CreateTestVisit(t, db, occupant.ID, targetGroup.ID, time.Now().Add(-time.Hour), nil)

		checkInTime := time.Now().Add(-2 * time.Hour)
		checkOutTime := time.Now().Add(-30 * time.Minute)
		testpkg.CreateTestAttendance(t, db, student.ID, teacher.Staff.ID, webDevice.ID, checkInTime, &checkOutTime)
		orphanVisit := testpkg.CreateTestVisit(t, db, student.ID, orphanGroup.ID, checkInTime, nil)

		token := testpkg.CreateTestJWT(t, account.ID, checkinPermissions)
		req := makeCheckinRequest(t, student.ID, active.CheckinRequest{ActiveGroupID: targetGroup.ID}, token)
		rr := httptest.NewRecorder()
		handler.Router().ServeHTTP(rr, req)

		require.Equal(t, http.StatusConflict, rr.Code, "Response body: %s", rr.Body.String())
		persisted := new(activeModels.Visit)
		err = db.NewSelect().Model(persisted).ModelTableExpr(`active.visits AS "visit"`).Where(`"visit".id = ?`, orphanVisit.ID).Scan(t.Context())
		require.NoError(t, err)
		assert.Nil(t, persisted.ExitTime)
	})

	t.Run("still 409 when checked in without a visit", func(t *testing.T) {
		handler := setupCheckinTestHandler(t, db)

		webDevice := testpkg.EnsureWebManualDevice(t, db)
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "NoVisit", "Teacher")
		educationGroup := testpkg.CreateTestEducationGroup(t, db, "No Visit Conflict Group")
		testpkg.CreateTestGroupTeacher(t, db, educationGroup.ID, teacher.ID)

		student := testpkg.CreateTestStudent(t, db, "NoVisit", "Student", "5c")
		testpkg.AssignStudentToGroup(t, db, student.ID, educationGroup.ID)

		targetActivity := testpkg.CreateTestActivityGroup(t, db, "no-visit-target")
		targetRoom := testpkg.CreateTestRoom(t, db, "No Visit Target Room")
		targetGroup := testpkg.CreateTestActiveGroup(t, db, targetActivity.ID, targetRoom.ID)

		// Checked in for the day but not in any room (open attendance, no visit).
		checkInTime := time.Now().Add(-1 * time.Hour)
		testpkg.CreateTestAttendance(t, db, student.ID, teacher.Staff.ID, webDevice.ID, checkInTime, nil)

		token := testpkg.CreateTestJWT(t, account.ID, checkinPermissions)
		body := active.CheckinRequest{ActiveGroupID: targetGroup.ID}
		req := makeCheckinRequest(t, student.ID, body, token)

		router := handler.Router()
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusConflict, rr.Code, "Response body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Student is already checked in")
	})

	t.Run("still 409 when student is genuinely in another room", func(t *testing.T) {
		handler := setupCheckinTestHandler(t, db)

		webDevice := testpkg.EnsureWebManualDevice(t, db)
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Genuine", "Teacher")
		educationGroup := testpkg.CreateTestEducationGroup(t, db, "Genuine Conflict Group")
		testpkg.CreateTestGroupTeacher(t, db, educationGroup.ID, teacher.ID)

		student := testpkg.CreateTestStudent(t, db, "Genuine", "Student", "5b")
		testpkg.AssignStudentToGroup(t, db, student.ID, educationGroup.ID)

		currentActivity := testpkg.CreateTestActivityGroup(t, db, "genuine-current")
		currentRoom := testpkg.CreateTestRoom(t, db, "Genuine Current Room")
		currentGroup := testpkg.CreateTestActiveGroup(t, db, currentActivity.ID, currentRoom.ID)
		targetActivity := testpkg.CreateTestActivityGroup(t, db, "genuine-target")
		targetRoom := testpkg.CreateTestRoom(t, db, "Genuine Target Room")
		targetGroup := testpkg.CreateTestActiveGroup(t, db, targetActivity.ID, targetRoom.ID)

		// Genuinely present: attendance open AND visit open.
		checkInTime := time.Now().Add(-1 * time.Hour)
		testpkg.CreateTestAttendance(t, db, student.ID, teacher.Staff.ID, webDevice.ID, checkInTime, nil)
		visit := testpkg.CreateTestVisit(t, db, student.ID, currentGroup.ID, checkInTime, nil)

		token := testpkg.CreateTestJWT(t, account.ID, checkinPermissions)
		body := active.CheckinRequest{ActiveGroupID: targetGroup.ID}
		req := makeCheckinRequest(t, student.ID, body, token)

		router := handler.Router()
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusConflict, rr.Code, "Response body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Student already has an active visit in another room")

		// The genuine visit must remain open — no healing here.
		untouched := new(activeModels.Visit)
		err := db.NewSelect().
			Model(untouched).
			ModelTableExpr(`active.visits AS "visit"`).
			Where(`"visit".id = ?`, visit.ID).
			Scan(context.Background())
		require.NoError(t, err)
		assert.Nil(t, untouched.ExitTime, "genuine active visit must not be ended by validation")
	})
}
