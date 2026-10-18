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

func TestStudentAbsenceEditOpensOnlyForAllStaff(t *testing.T) {
	t.Parallel()
	if !StudentAbsenceEditOpenToAllStaff(StudentAbsenceEditScopeAllStaff) || StudentAbsenceEditOpenToAllStaff(StudentAbsenceEditScopeAdmins) || StudentAbsenceEditOpenToAllStaff("") {
		t.Error("only the all-staff scope opens direct absence reports")
	}
}

func TestScopePredicatesOpenOnlyForTheirValue(t *testing.T) {
	t.Parallel()
	if !OverviewOpenToAllStaff(OverviewScopeAllStaff) || OverviewOpenToAllStaff(OverviewScopeOwn) || OverviewOpenToAllStaff(OverviewScopeAdmins) || OverviewOpenToAllStaff("") {
		t.Error("only the all-staff overview scope opens every group")
	}
	if !SchoolPortalWritesClassArrivalExceptions(SchoolPortalWriteScopeClassArrivalExceptions) || SchoolPortalWritesClassArrivalExceptions(SchoolPortalWriteScopeNone) || SchoolPortalWritesClassArrivalExceptions("") {
		t.Error("only the class arrival exception scope opens the school portal write")
	}
	if !IsBinaryPresence(PresenceModeBinary) || IsBinaryPresence(PresenceModeDetailed) || IsBinaryPresence("") {
		t.Error("only the binary value is the binary presence mode")
	}
}
