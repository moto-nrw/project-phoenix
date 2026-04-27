// Package active internal tests for the actual-arrival/pickup enrichment
// helpers used by the visits-with-display endpoint.
package active

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	activeService "github.com/moto-nrw/project-phoenix/services/active"
)

func TestCollectVisitStudentIDs_DedupesAndPreservesOrder(t *testing.T) {
	results := []visitWithStudent{
		{StudentID: 11},
		{StudentID: 22},
		{StudentID: 11}, // duplicate — second visit for the same student today
		{StudentID: 33},
		{StudentID: 22}, // duplicate
	}

	ids := collectVisitStudentIDs(results)

	assert.Equal(t, []int64{11, 22, 33}, ids,
		"deduped IDs should preserve first-seen order")
}

func TestCollectVisitStudentIDs_Empty(t *testing.T) {
	ids := collectVisitStudentIDs(nil)
	assert.Empty(t, ids)

	ids = collectVisitStudentIDs([]visitWithStudent{})
	assert.Empty(t, ids)
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

	responses := rs.buildVisitDisplayResponses(results, statuses)

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

	responses := rs.buildVisitDisplayResponses(results, statuses)

	if assert.Len(t, responses, 1) {
		assert.Nil(t, responses[0].ActualArrival)
		assert.Nil(t, responses[0].ActualPickup)
	}
}
