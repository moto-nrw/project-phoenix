package supervisiondashboard

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sick and excused status with their timestamps need users:read, in the
// selected group and in the released rooms alike (ADR 0091). A guest reads
// the dashboard with groups:read only.
func TestDashboardAbsenceStatusFollowsStudentRead(t *testing.T) {
	t.Parallel()

	since := time.Date(2026, time.August, 19, 7, 0, 0, 0, time.UTC)
	entry := since.Add(time.Hour)
	yardGroupID := int64(30)
	absent := func(studentID, activeGroupID int64) VisitRecord {
		return VisitRecord{
			StudentID: studentID, ActiveGroupID: activeGroupID, EntryTime: entry, FirstName: "Erika",
			Sick: true, SickSince: &since, Excused: true, ExcusedSince: &since,
		}
	}

	for _, canReadStudents := range []bool{false, true} {
		f, _ := openRoomFakes()
		f.caller = func(context.Context) (Caller, error) {
			return Caller{AccountID: 70, CanReadSchedules: true, CanReadStudents: canReadStudents}, nil
		}
		f.fullStudentAccess = func(context.Context) (bool, error) { return true, nil }
		f.yard = func(context.Context, int64) (*SchulhofStatus, error) {
			return &SchulhofStatus{Exists: true, IsUserSupervising: true, ActiveGroupID: &yardGroupID}, nil
		}
		f.groupVisits = func(context.Context, int64) ([]VisitRecord, error) {
			return []VisitRecord{absent(1, yardGroupID)}, nil
		}
		f.openVisits = func(context.Context, []int64) ([]VisitRecord, error) {
			return []VisitRecord{absent(101, 11)}, nil
		}

		projection, err := newService(f).Dashboard(context.Background(), yardGroupID)
		require.NoError(t, err)
		require.Len(t, projection.Visits, 1)
		students := openRoomByName(t, projection, "Schulhof").Students
		require.Len(t, students, 1)

		for _, visit := range []Visit{projection.Visits[0], students[0].Visit} {
			assert.Equal(t, canReadStudents, visit.Sick, "users:read=%v", canReadStudents)
			assert.Equal(t, canReadStudents, visit.Excused, "users:read=%v", canReadStudents)
			if canReadStudents {
				assert.Equal(t, &since, visit.SickSince)
				assert.Equal(t, &since, visit.ExcusedSince)
			} else {
				assert.Nil(t, visit.SickSince)
				assert.Nil(t, visit.ExcusedSince)
			}
		}
	}
}
