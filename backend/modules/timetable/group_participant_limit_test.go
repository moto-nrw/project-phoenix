package timetable

import "testing"

func TestGroupParticipantLimit(t *testing.T) {
	t.Parallel()
	if (Group{MaxParticipants: 0}).ParticipantLimit() != nil {
		t.Error("a group without a cap has no participant limit")
	}
	capped := 12
	if limit := (Group{MaxParticipants: capped}).ParticipantLimit(); limit == nil || *limit != capped {
		t.Errorf("participant limit = %v, want %d", limit, capped)
	}
}
