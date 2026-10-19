package records

import (
	"testing"
	"time"
)

func TestStaffScheduleVersionStartIsTheFirstStartDay(t *testing.T) {
	t.Parallel()
	first := newTestDate(2026, time.March, 2)
	later := newTestDate(2026, time.April, 6)
	entries := []*StaffWorkSchedule{nil, {}, {ValidFrom: first}, {ValidFrom: later}}
	if got := StaffScheduleVersionStart(entries); got != first {
		t.Errorf("StaffScheduleVersionStart = %q, want %q", got, first)
	}
	if got := StaffScheduleVersionStart([]*StaffWorkSchedule{nil, {}}); got != "" {
		t.Errorf("without a start day the version starts today, got %q", got)
	}
	if got := StaffScheduleVersionStart(nil); got != "" {
		t.Errorf("no entries start the version today, got %q", got)
	}
}
