package studentpresence

import (
	"errors"
	"slices"
	"testing"
)

func TestCurrentRoomVisitsFilterSelectsTheLatestOpenVisitInARunningSession(t *testing.T) {
	t.Parallel()
	first, second := int64(101), int64(102)
	filter := CurrentRoomVisitsFilter([]int64{first, second})
	if !slices.Equal(filter.StudentIDs, []int64{first, second}) {
		t.Errorf("student IDs = %v", filter.StudentIDs)
	}
	if !filter.OpenOnly || !filter.RunningGroupsOnly || !filter.LatestPerStudent {
		t.Errorf("filter = %+v, want open visits, running sessions, latest per student", filter)
	}
	if filter.StudentOrder || filter.NewestFirst {
		t.Errorf("filter = %+v, want the owner's latest-per-student selection, not a listing order", filter)
	}
}

func TestSchoolStatusReadFailedKeepsThePublicText(t *testing.T) {
	t.Parallel()
	err := SchoolStatusReadFailed()
	if err.Error() != "active: GetStudentsAttendanceStatuses: database operation failed" {
		t.Errorf("masked error = %q", err)
	}
	if !errors.Is(err, ErrDatabaseOperation) {
		t.Error("the masked error classifies as a database operation failure")
	}
}
