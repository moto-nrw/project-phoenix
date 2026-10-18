package records

import "testing"

func TestModesReadUnknownValuesAsTheDefault(t *testing.T) {
	t.Parallel()
	for value, want := range map[string]string{PresenceModeBinary: PresenceModeBinary, PresenceModeDetailed: PresenceModeDetailed, "": PresenceModeDetailed, "other": PresenceModeDetailed} {
		if got := PresenceModeOf(value); got != want {
			t.Errorf("PresenceModeOf(%q) = %q, want %q", value, got, want)
		}
	}
	for value, want := range map[string]string{GroupModeOpenCare: GroupModeOpenCare, GroupModeFixedGroups: GroupModeFixedGroups, "": GroupModeFixedGroups, "other": GroupModeFixedGroups} {
		if got := GroupModeOf(value); got != want {
			t.Errorf("GroupModeOf(%q) = %q, want %q", value, got, want)
		}
	}
}
