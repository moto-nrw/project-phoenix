package authorize

import "testing"

type testGuardian struct {
	relationship string
	role         string
	primary      bool
	emergency    bool
	pickup       bool
	permissions  map[string]interface{}
}

func (g *testGuardian) GuardianAuthorizationData() (string, string, bool, bool, bool, map[string]interface{}) {
	if g == nil {
		return "", "", false, false, false, nil
	}
	return g.relationship, g.role, g.primary, g.emergency, g.pickup, g.permissions
}

func (g *testGuardian) SetGuardianAuthorizationData(role string, permissions map[string]interface{}) {
	if g != nil {
		g.role, g.permissions = role, permissions
	}
}

func TestGuardianRolePreset(t *testing.T) {
	t.Parallel()
	g := &testGuardian{relationship: "parent"}
	ApplyDefaultStudentGuardianRole(g)
	if g.role != GuardianRoleLegalGuardian || !StudentGuardianHasPermission(g, GuardianPermissionPortalAccess) {
		t.Fatal("parent relationship must receive the legal-guardian preset")
	}
	ApplyStudentGuardianRole(g, GuardianRoleSocialWorker)
	if g.role != GuardianRoleSocialWorker || StudentGuardianHasPermission(g, GuardianPermissionPortalAccess) {
		t.Fatal("restricted presets must clear portal permissions")
	}
}

func TestDefaultGuardianRoleIsConservative(t *testing.T) {
	t.Parallel()
	tests := []struct {
		relationship       string
		primary, emergency bool
		pickup             bool
		want               string
	}{
		{"other", true, false, false, GuardianRolePrimaryGuardian},
		{"guardian", false, false, false, GuardianRoleLegalGuardian},
		{"other", false, false, true, GuardianRolePickupOnly},
		{"other", false, true, false, GuardianRoleEmergency},
		{"other", false, false, false, GuardianRoleCustom},
	}
	for _, tt := range tests {
		if got := DefaultStudentGuardianRole(tt.relationship, tt.primary, tt.emergency, tt.pickup); got != tt.want {
			t.Errorf("DefaultStudentGuardianRole() = %q, want %q", got, tt.want)
		}
	}
}

func TestGuardianPermissionValuesFailClosed(t *testing.T) {
	t.Parallel()
	g := &testGuardian{permissions: map[string]interface{}{"yes": true, "no": false, "legacy": "granted", "nil": nil}}
	if !StudentGuardianHasPermission(g, "yes") || StudentGuardianHasPermission(g, "no") || !StudentGuardianHasPermission(g, "legacy") || StudentGuardianHasPermission(g, "nil") {
		t.Fatal("guardian permission map semantics changed")
	}
}

func TestFullGuardianCanManageStudentConsents(t *testing.T) {
	t.Parallel()

	for _, role := range []string{GuardianRolePrimaryGuardian, GuardianRoleLegalGuardian, GuardianRoleCoGuardian} {
		guardian := &testGuardian{}
		ApplyStudentGuardianRole(guardian, role)
		if !StudentGuardianHasPermission(guardian, GuardianPermissionConsentManage) {
			t.Fatalf("full guardian role %q cannot manage student consents", role)
		}
	}
}

func TestFullGuardianCanManageMealParticipation(t *testing.T) {
	t.Parallel()

	for _, role := range []string{GuardianRolePrimaryGuardian, GuardianRoleLegalGuardian, GuardianRoleCoGuardian} {
		guardian := &testGuardian{}
		ApplyStudentGuardianRole(guardian, role)
		if !StudentGuardianHasPermission(guardian, GuardianPermissionMealParticipationManage) {
			t.Fatalf("full guardian role %q cannot manage meal participation", role)
		}
	}
}

func TestGuardianRoleNormalizationAndPermissionSets(t *testing.T) {
	t.Parallel()
	for _, role := range []string{GuardianRolePrimaryGuardian, GuardianRoleLegalGuardian, GuardianRoleCoGuardian} {
		if !IsFullGuardianRole(role) || len(StudentGuardianPermissionSet(role)) != len(fullParentPortalPermissions) {
			t.Fatalf("full role %q did not receive the full permission set", role)
		}
	}
	for _, role := range []string{GuardianRoleEmergency, GuardianRolePickupOnly, GuardianRoleSocialWorker, GuardianRoleCustom, "unknown"} {
		if len(StudentGuardianPermissionSet(role)) != 0 {
			t.Fatalf("restricted role %q received permissions", role)
		}
	}
	if got := NormalizeGuardianRole("  LEGAL_GUARDIAN "); got != GuardianRoleLegalGuardian {
		t.Fatalf("NormalizeGuardianRole() = %q", got)
	}
	var missing *testGuardian
	ApplyStudentGuardianRole(missing, GuardianRolePrimaryGuardian)
	ApplyDefaultStudentGuardianRole(missing)
}
