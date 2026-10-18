package schoolstructure_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type dayRecordingStaffGroups struct {
	schoolstructure.StaffGroupQuery
	staffID int64
	day     string
}

func (q *dayRecordingStaffGroups) ListSubstitutedGroups(_ context.Context, staffID int64, day string) ([]schoolstructure.SubstitutedGroup, error) {
	q.staffID, q.day = staffID, day
	return nil, nil
}

// The read names a calendar day in its YYYY-MM-DD form for the staff member;
// which day the clock yields is calendar.TodayDate's contract.
func TestListSubstitutedGroupsTodayAsksForACalendarDay(t *testing.T) {
	t.Parallel()

	staffID := int64(4082)
	query := &dayRecordingStaffGroups{}
	if _, err := schoolstructure.ListSubstitutedGroupsToday(context.Background(), query, staffID); err != nil {
		t.Fatalf("ListSubstitutedGroupsToday() = %v", err)
	}
	if query.staffID != staffID {
		t.Fatalf("asked for staff %d, want %d", query.staffID, staffID)
	}
	if _, err := calendar.ParseDate(query.day); err != nil {
		t.Fatalf("asked for day %q, want a calendar date: %v", query.day, err)
	}
}
