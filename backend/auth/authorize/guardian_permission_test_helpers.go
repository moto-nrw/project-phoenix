//go:build testhooks

package authorize

// Fixture role assignment stays available to test builds only. Production
// workflows bind canonical role decisions through public Security Runtime.
func ApplyStudentGuardianRole(sg studentGuardianRecord, role string) {
	if sg == nil {
		return
	}
	normalized := NormalizeGuardianRole(role)
	sg.SetGuardianAuthorizationData(normalized, StudentGuardianPermissionSet(normalized))
}

func ApplyDefaultStudentGuardianRole(sg studentGuardianRecord) {
	if sg == nil {
		return
	}
	relationshipType, _, primary, emergency, pickup, _ := sg.GuardianAuthorizationData()
	role := DefaultStudentGuardianRole(relationshipType, primary, emergency, pickup)
	ApplyStudentGuardianRole(sg, role)
}
