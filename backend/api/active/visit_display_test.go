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

	t.Run("staff sees everyone, group-less children included", func(t *testing.T) {
		// #2329: staff access is tenant-wide, the child's group is irrelevant.
		ids := collectAuthorizedVisitStudentIDs(results, &common.StudentAccessContext{IsStaff: true})
		assert.ElementsMatch(t, []int64{11, 22, 33}, ids)
	})

	t.Run("neither admin nor staff yields empty slice", func(t *testing.T) {
		access := &common.StudentAccessContext{}
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
// Since #2329 the gate is the caller's access level, not the child's group: a
// verified staff member sees actuals for every child (group-less included), a
// caller without full access keeps name + class + sick/excused (existing trust
// boundary) but gets nil actuals for all of them.
func TestBuildVisitDisplayResponses_ActualsGatedPerStudent(t *testing.T) {
	rs := &Resource{}

	checkIn := time.Date(2026, 4, 27, 6, 30, 0, 0, time.UTC)
	checkOut := time.Date(2026, 4, 27, 14, 5, 0, 0, time.UTC)

	groupOne := int64(100)
	groupTwo := int64(200)

	results := []visitWithStudent{
		{
			VisitID: 1, StudentID: 11, ActiveGroupID: 999,
			EntryTime: checkIn,
			FirstName: "Anna", LastName: "Müller",
			GroupID: &groupOne,
		},
		{
			VisitID: 2, StudentID: 22, ActiveGroupID: 999,
			EntryTime: checkIn,
			FirstName: "Ben", LastName: "Otto",
			GroupID: &groupTwo,
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

	// Verified staff: actuals for every child, regardless of group.
	staffResponses := rs.buildVisitDisplayResponses(
		results, statuses, &common.StudentAccessContext{IsStaff: true}, true,
	)

	if assert.Len(t, staffResponses, 3) {
		assert.Equal(t, "Anna Müller", staffResponses[0].StudentName)
		if assert.NotNil(t, staffResponses[0].ActualArrival) {
			assert.Equal(t, "08:30", *staffResponses[0].ActualArrival)
		}
		if assert.NotNil(t, staffResponses[0].ActualPickup) {
			assert.Equal(t, "16:05", *staffResponses[0].ActualPickup)
		}

		assert.Equal(t, "Ben Otto", staffResponses[1].StudentName)
		assert.NotNil(t, staffResponses[1].ActualArrival,
			"staff access is tenant-wide since #2329 — the child's group is irrelevant")

		assert.Equal(t, "Cara Pohl", staffResponses[2].StudentName)
		assert.NotNil(t, staffResponses[2].ActualArrival,
			"a child without a group is an ordinary child for staff")
	}

	// No full access (guest, guardian): names stay, actuals are redacted.
	redactedResponses := rs.buildVisitDisplayResponses(
		results, statuses, &common.StudentAccessContext{}, true,
	)

	if assert.Len(t, redactedResponses, 3) {
		for i, response := range redactedResponses {
			assert.NotEmpty(t, response.StudentName)
			assert.Nil(t, response.ActualArrival,
				"actuals must be redacted without full access (index %d)", i)
			assert.Nil(t, response.ActualPickup)
		}
	}
}
