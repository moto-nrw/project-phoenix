package timetablehttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// External caregivers (#4573) can be planned and marked absent like any
// staff member, but are never replacement staff, with or without an account
// (D6): the Vertretungsplan and the Sammel-Vertretung refuse them as
// substitute with timetable.deviation_selection_invalid.

// devAccountBackedExternal creates an external caregiver with an account in
// the test's tenant: a staff member with a login and an active external caregiver record.
func devAccountBackedExternal(t *testing.T, s *devSetup) int64 {
	t.Helper()
	staff, _ := testpkg.CreateTestStaffWithAccount(t, s.db, "Extern", fmt.Sprintf("MitKonto-%d", time.Now().UnixNano()))
	record := &testpkg.ExternalCaregiverRecord{TenantID: testpkg.Tenant(t), StaffID: staff.ID, ActivityExpertise: "Musik"}
	_, err := s.db.NewInsert().Model(record).ModelTableExpr(`users.external_caregivers`).Exec(s.ctx)
	require.NoError(t, err)
	return staff.ID
}

func requireRefusal(t *testing.T, w *httptest.ResponseRecorder, wantCode, wantField string) {
	t.Helper()
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	var body struct {
		Code   string `json:"code"`
		Errors []struct {
			Field string `json:"field"`
		} `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, wantCode, body.Code)
	require.Len(t, body.Errors, 1)
	assert.Equal(t, wantField, body.Errors[0].Field)
}

func TestApplyDeviations_MarksExternalCaregiverAbsent(t *testing.T) {
	t.Parallel()

	s := buildDevModule(t)
	router := devRouter(s.ctx, s.res, s.db)
	_, date := futureSubDate(1)
	externalID := devAccountBackedExternal(t, s)
	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "AG Musik"})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, externalID, testpkg.InstanceStaffOpts{IsPrimary: true})

	w := doDev(t, router, inst.ID, map[string]any{
		"absences": []map[string]any{{"staff_id": externalID, "instance_ids": []int64{inst.ID}}},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	rows := devInstanceStaff(t, s.db, s.ctx, inst.ID)
	require.Len(t, rows, 1)
	assert.Equal(t, externalID, rows[0].StaffID)
	assert.True(t, rows[0].IsAbsent, "the external is absent on this occurrence")
}

func TestApplyDeviations_RejectsExternalCaregiverAsSubstitute(t *testing.T) {
	t.Parallel()

	s := buildDevModule(t)
	router := devRouter(s.ctx, s.res, s.db)
	_, date := futureSubDate(1)
	externalID := devAccountBackedExternal(t, s)
	accountless := testpkg.CreateTestExternalCaregiverRecord(t, s.db, "Trommeln")
	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Vertretung"})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffA, testpkg.InstanceStaffOpts{})

	for name, substituteID := range map[string]int64{"with account": externalID, "without account": accountless.StaffID} {
		t.Run(name, func(t *testing.T) {
			w := doDev(t, router, inst.ID, map[string]any{
				"substitutions": []map[string]any{{"absent_staff_id": s.staffA, "substitute_staff_id": substituteID}},
			})
			requireRefusal(t, w, "timetable.deviation_selection_invalid", "substitute_staff_id")
		})
	}

	rows := devInstanceStaff(t, s.db, s.ctx, inst.ID)
	require.Len(t, rows, 1, "no substitute row was written")
	assert.False(t, rows[0].IsAbsent, "the refused save left the planned person present")

	// Positive anchor: a regular colleague still substitutes.
	w := doDev(t, router, inst.ID, map[string]any{
		"substitutions": []map[string]any{{"absent_staff_id": s.staffA, "substitute_staff_id": s.staffY}},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

func TestBulkSubstitution_RejectsExternalCaregiverAsSubstitute(t *testing.T) {
	t.Parallel()

	s := buildDevModule(t)
	router := bulkRouter(s.ctx, s.res, s.db)
	d1Str, d1 := futureSubDate(1)
	externalID := devAccountBackedExternal(t, s)
	inst := testpkg.CreateTestActivityInstance(t, s.db, d1, s.roomID, testpkg.ActivityInstanceOpts{Title: "Sammel"})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffA, testpkg.InstanceStaffOpts{})

	w := doBulk(t, router, map[string]any{
		"absent_staff_id":     s.staffA,
		"substitute_staff_id": externalID,
		"dates":               []string{d1Str},
	})
	requireRefusal(t, w, "timetable.deviation_selection_invalid", "substitute_staff_id")

	absentA, subRow, _ := bulkStaffState(t, s, inst.ID)
	assert.False(t, absentA, "the refused save marked nobody absent")
	assert.Nil(t, subRow)
	assert.Len(t, devInstanceStaff(t, s.db, s.ctx, inst.ID), 1, "no row for the external")

	// An external may still be marked absent across several days.
	extInst := testpkg.CreateTestActivityInstance(t, s.db, d1, s.roomID, testpkg.ActivityInstanceOpts{Title: "AG Musik"})
	testpkg.CreateTestInstanceStaff(t, s.db, extInst.ID, externalID, testpkg.InstanceStaffOpts{})
	w = doBulk(t, router, map[string]any{"absent_staff_id": externalID, "dates": []string{d1Str}})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	rows := devInstanceStaff(t, s.db, s.ctx, extInst.ID)
	require.Len(t, rows, 1)
	assert.True(t, rows[0].IsAbsent)
}
