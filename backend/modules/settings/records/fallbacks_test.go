package records

import "testing"

func TestReadFallbacksAreTheRegistryDefaults(t *testing.T) {
	t.Parallel()
	const roomWindowDays = 7
	if AttendanceLogEnabledFallback {
		t.Error("an unreadable attendance log switch keeps the room history off")
	}
	if RoomDetailVisibleDaysFallback != roomWindowDays {
		t.Errorf("RoomDetailVisibleDaysFallback = %d, want %d", RoomDetailVisibleDaysFallback, roomWindowDays)
	}
	if !EnrollmentCareOfferingsEnabledFallback {
		t.Error("enrollment reports without a settings resolver keep care offerings on")
	}
}
