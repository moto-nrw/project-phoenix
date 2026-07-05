// Package active internal tests for the actual-arrival/pickup enrichment
// helpers used by the visits-with-display endpoint.
package active

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/api/common"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
)

// adminAccess returns an access context that grants full access to every
// student — used by tests that focus on the enrichment plumbing rather than
// on the GDPR gate itself.
func adminAccess() *common.StudentAccessContext {
	return &common.StudentAccessContext{IsAdmin: true}
}

func TestCollectAuthorizedVisitStudentIDs_FiltersByAccess(t *testing.T) {
	groupA := int64(100)
	groupB := int64(200)
	results := []visitWithStudent{
		{StudentID: 11, GroupID: &groupA},
		{StudentID: 22, GroupID: &groupB},
		{StudentID: 33, GroupID: nil},     // group-less
		{StudentID: 11, GroupID: &groupA}, // duplicate
	}

	t.Run("admin sees everyone (group-less included)", func(t *testing.T) {
		ids := collectAuthorizedVisitStudentIDs(results, &common.StudentAccessContext{IsAdmin: true})
		assert.ElementsMatch(t, []int64{11, 22, 33}, ids)
	})

	t.Run("all_staff scope sees everyone", func(t *testing.T) {
		ids := collectAuthorizedVisitStudentIDs(results, &common.StudentAccessContext{AllStaffScope: true})
		assert.ElementsMatch(t, []int64{11, 22, 33}, ids)
	})

	t.Run("group supervisor sees only their group", func(t *testing.T) {
		access := &common.StudentAccessContext{
			MyGroupIDs: map[int64]struct{}{groupA: {}},
		}
		ids := collectAuthorizedVisitStudentIDs(results, access)
		assert.Equal(t, []int64{11}, ids)
	})

	t.Run("no access yields empty slice", func(t *testing.T) {
		access := &common.StudentAccessContext{MyGroupIDs: map[int64]struct{}{}}
		ids := collectAuthorizedVisitStudentIDs(results, access)
		assert.Empty(t, ids)
	})

	t.Run("nil access context yields empty slice", func(t *testing.T) {
		ids := collectAuthorizedVisitStudentIDs(results, nil)
		assert.Empty(t, ids)
	})
}

func TestBuildVisitDisplayResponses_AppliesActualTimesFromMap(t *testing.T) {
	rs := &Resource{}

	checkIn := time.Date(2026, 4, 27, 6, 30, 0, 0, time.UTC)  // 08:30 Berlin (CEST)
	checkOut := time.Date(2026, 4, 27, 14, 5, 0, 0, time.UTC) // 16:05 Berlin (CEST)

	results := []visitWithStudent{
		{
			VisitID:       1,
			StudentID:     11,
			ActiveGroupID: 100,
			EntryTime:     checkIn,
			FirstName:     "Anna",
			LastName:      "Müller",
		},
		{
			VisitID:       2,
			StudentID:     22,
			ActiveGroupID: 100,
			EntryTime:     checkIn,
			FirstName:     "Ben",
			LastName:      "Otto",
		},
	}

	statuses := map[int64]*activeService.AttendanceStatus{
		11: {
			Status:       "checked_out",
			CheckInTime:  &checkIn,
			CheckOutTime: &checkOut,
		},
		// student 22 deliberately omitted — actuals must stay nil
	}

	responses := rs.buildVisitDisplayResponses(results, statuses, adminAccess(), true)

	if assert.Len(t, responses, 2) {
		assert.Equal(t, "Anna Müller", responses[0].StudentName)
		if assert.NotNil(t, responses[0].ActualArrival) {
			assert.Equal(t, "08:30", *responses[0].ActualArrival)
		}
		if assert.NotNil(t, responses[0].ActualPickup) {
			assert.Equal(t, "16:05", *responses[0].ActualPickup)
		}

		assert.Equal(t, "Ben Otto", responses[1].StudentName)
		assert.Nil(t, responses[1].ActualArrival,
			"missing attendance status must leave actuals nil, not empty string")
		assert.Nil(t, responses[1].ActualPickup)
	}
}

func TestBuildVisitDisplayResponses_NilStatusEntryIsSkipped(t *testing.T) {
	rs := &Resource{}

	results := []visitWithStudent{{
		VisitID:   1,
		StudentID: 11,
		EntryTime: time.Now(),
		FirstName: "Anna",
		LastName:  "Müller",
	}}

	// Map contains the key but the pointer is nil — must not panic and must
	// leave the actuals as nil rather than dereferencing.
	statuses := map[int64]*activeService.AttendanceStatus{11: nil}

	responses := rs.buildVisitDisplayResponses(results, statuses, adminAccess(), true)

	if assert.Len(t, responses, 1) {
		assert.Nil(t, responses[0].ActualArrival)
		assert.Nil(t, responses[0].ActualPickup)
	}
}

// TestBuildVisitDisplayResponses_ActualsGatedPerStudent locks in the GDPR
// guarantee: actual arrival/pickup times must mirror planned-time gating.
// A caller who supervises one education group must see actuals only for
// that group's students; other students in the active group keep name +
// class + sick/excused (existing trust boundary) but get nil actuals.
func TestBuildVisitDisplayResponses_ActualsGatedPerStudent(t *testing.T) {
	rs := &Resource{}

	checkIn := time.Date(2026, 4, 27, 6, 30, 0, 0, time.UTC)
	checkOut := time.Date(2026, 4, 27, 14, 5, 0, 0, time.UTC)

	groupSupervised := int64(100)
	groupOther := int64(200)

	results := []visitWithStudent{
		{
			VisitID: 1, StudentID: 11, ActiveGroupID: 999,
			EntryTime: checkIn,
			FirstName: "Anna", LastName: "Müller",
			GroupID: &groupSupervised,
		},
		{
			VisitID: 2, StudentID: 22, ActiveGroupID: 999,
			EntryTime: checkIn,
			FirstName: "Ben", LastName: "Otto",
			GroupID: &groupOther,
		},
		{
			VisitID: 3, StudentID: 33, ActiveGroupID: 999,
			EntryTime: checkIn,
			FirstName: "Cara", LastName: "Pohl",
			GroupID: nil, // group-less student
		},
	}

	statuses := map[int64]*activeService.AttendanceStatus{
		11: {Status: "checked_out", CheckInTime: &checkIn, CheckOutTime: &checkOut},
		22: {Status: "checked_in", CheckInTime: &checkIn},
		33: {Status: "checked_in", CheckInTime: &checkIn},
	}

	access := &common.StudentAccessContext{
		MyGroupIDs: map[int64]struct{}{groupSupervised: {}},
	}

	responses := rs.buildVisitDisplayResponses(results, statuses, access, true)

	if assert.Len(t, responses, 3) {
		// Supervised student → actuals visible
		assert.Equal(t, "Anna Müller", responses[0].StudentName)
		if assert.NotNil(t, responses[0].ActualArrival) {
			assert.Equal(t, "08:30", *responses[0].ActualArrival)
		}
		if assert.NotNil(t, responses[0].ActualPickup) {
			assert.Equal(t, "16:05", *responses[0].ActualPickup)
		}

		// Other-group student → name still visible (existing trust boundary)
		// but actuals MUST be nil to match planned-time gating.
		assert.Equal(t, "Ben Otto", responses[1].StudentName)
		assert.Nil(t, responses[1].ActualArrival,
			"actuals must be redacted for students outside the caller's group scope")
		assert.Nil(t, responses[1].ActualPickup)

		// Group-less student → only admins / all_staff scope may see actuals
		assert.Equal(t, "Cara Pohl", responses[2].StudentName)
		assert.Nil(t, responses[2].ActualArrival,
			"group-less students must be redacted unless caller is admin or all_staff")
		assert.Nil(t, responses[2].ActualPickup)
	}
}
