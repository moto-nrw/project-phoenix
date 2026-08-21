package students

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
)

// These tests cover the small pure helpers that thread today's actual
// arrival/pickup times from the attendance service into the student list and
// detail responses. They are mock-free — exempted from the hermetic ID check
// because no DB IDs are used.

func TestApplyActualTimesFromAttendance_NilGuards(t *testing.T) {
	t.Parallel()

	t.Run("nil response is a no-op", func(t *testing.T) {
		// Should not panic.
		applyActualTimesFromAttendance(nil, &activeService.AttendanceStatus{})
	})

	t.Run("nil status leaves response untouched", func(t *testing.T) {
		response := &StudentResponse{
			ActualArrivalTime: ptr("07:55"),
			ActualPickupTime:  ptr("15:42"),
		}

		applyActualTimesFromAttendance(response, nil)

		if response.ActualArrivalTime == nil || *response.ActualArrivalTime != "07:55" {
			t.Fatalf("expected ActualArrivalTime untouched, got %v", response.ActualArrivalTime)
		}
		if response.ActualPickupTime == nil || *response.ActualPickupTime != "15:42" {
			t.Fatalf("expected ActualPickupTime untouched, got %v", response.ActualPickupTime)
		}
	})
}

func TestApplyActualTimesFromAttendance_FormatsBothClocks(t *testing.T) {
	t.Parallel()

	checkIn := time.Date(2026, 4, 27, 6, 5, 0, 0, time.UTC)    // 08:05 Berlin (CEST)
	checkOut := time.Date(2026, 4, 27, 13, 42, 0, 0, time.UTC) // 15:42 Berlin (CEST)

	response := &StudentResponse{}
	status := &activeService.AttendanceStatus{
		Status:       "checked_out",
		CheckInTime:  &checkIn,
		CheckOutTime: &checkOut,
	}

	applyActualTimesFromAttendance(response, status)

	if response.ActualArrivalTime == nil || *response.ActualArrivalTime != "08:05" {
		t.Fatalf("ActualArrivalTime: want 08:05, got %v", response.ActualArrivalTime)
	}
	if response.ActualPickupTime == nil || *response.ActualPickupTime != "15:42" {
		t.Fatalf("ActualPickupTime: want 15:42, got %v", response.ActualPickupTime)
	}
}

func TestApplyActualTimesFromAttendance_CheckedInOnly(t *testing.T) {
	t.Parallel()

	checkIn := time.Date(2026, 1, 15, 7, 12, 0, 0, timezone.Berlin) // already in Berlin

	response := &StudentResponse{}
	applyActualTimesFromAttendance(response, &activeService.AttendanceStatus{
		Status:      "checked_in",
		CheckInTime: &checkIn,
	})

	if response.ActualArrivalTime == nil || *response.ActualArrivalTime != "07:12" {
		t.Fatalf("ActualArrivalTime: want 07:12, got %v", response.ActualArrivalTime)
	}
	if response.ActualPickupTime != nil {
		t.Fatalf("ActualPickupTime should be nil while still checked in, got %v", response.ActualPickupTime)
	}
}

func TestApplyActualTimesFromSnapshot_NilGuards(t *testing.T) {
	t.Parallel()

	response := &StudentResponse{ID: 42}

	// All three nil-paths must be no-ops without panicking.
	applyActualTimesFromSnapshot(response, nil)

	applyActualTimesFromSnapshot(response, &common.StudentDataSnapshot{})

	applyActualTimesFromSnapshot(response, &common.StudentDataSnapshot{
		LocationSnapshot: &common.StudentLocationSnapshot{
			Attendances: map[int64]*activeService.AttendanceStatus{},
		},
	})

	if response.ActualArrivalTime != nil || response.ActualPickupTime != nil {
		t.Fatalf("expected response untouched, got arrival=%v pickup=%v",
			response.ActualArrivalTime, response.ActualPickupTime)
	}
}

func TestApplyActualTimesFromSnapshot_LooksUpByResponseID(t *testing.T) {
	t.Parallel()

	const studentID int64 = 42
	checkIn := time.Date(2026, 4, 27, 6, 30, 0, 0, time.UTC)  // 08:30 Berlin
	checkOut := time.Date(2026, 4, 27, 14, 0, 0, 0, time.UTC) // 16:00 Berlin

	snapshot := &common.StudentDataSnapshot{
		LocationSnapshot: &common.StudentLocationSnapshot{
			Attendances: map[int64]*activeService.AttendanceStatus{
				studentID: {
					Status:       "checked_out",
					CheckInTime:  &checkIn,
					CheckOutTime: &checkOut,
				},
				// Sibling entry must not bleed into this student's response.
				studentID + 1: {
					Status:      "checked_in",
					CheckInTime: timePtr(2026, 4, 27, 5, 0, 0),
				},
			},
		},
	}

	response := &StudentResponse{ID: studentID}
	applyActualTimesFromSnapshot(response, snapshot)

	if response.ActualArrivalTime == nil || *response.ActualArrivalTime != "08:30" {
		t.Fatalf("ActualArrivalTime: want 08:30, got %v", response.ActualArrivalTime)
	}
	if response.ActualPickupTime == nil || *response.ActualPickupTime != "16:00" {
		t.Fatalf("ActualPickupTime: want 16:00, got %v", response.ActualPickupTime)
	}
}

func TestApplyActualTimesFromSnapshot_StudentMissingFromSnapshot(t *testing.T) {
	t.Parallel()

	snapshot := &common.StudentDataSnapshot{
		LocationSnapshot: &common.StudentLocationSnapshot{
			Attendances: map[int64]*activeService.AttendanceStatus{
				99: {Status: "checked_in"},
			},
		},
	}

	response := &StudentResponse{ID: 42}
	applyActualTimesFromSnapshot(response, snapshot)

	if response.ActualArrivalTime != nil || response.ActualPickupTime != nil {
		t.Fatalf("expected response untouched when student not in snapshot, got arrival=%v pickup=%v",
			response.ActualArrivalTime, response.ActualPickupTime)
	}
}

func ptr(s string) *string { return &s }

func timePtr(year int, month time.Month, day, hour, min, sec int) *time.Time {
	t := time.Date(year, month, day, hour, min, sec, 0, time.UTC)
	return &t
}
