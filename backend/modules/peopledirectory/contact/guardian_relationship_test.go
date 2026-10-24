package contact

import "testing"

func TestGuardianRelationshipNormalizationKeepsFailureMutationOrder(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                        string
		input                       GuardianRelationshipValues
		message, relationship, role string
		permissions                 bool
	}{
		{"student missing", GuardianRelationshipValues{GuardianProfileID: 2, RelationshipType: "PARENT", GuardianRole: " LEGAL_GUARDIAN "}, "student ID is required", "PARENT", " LEGAL_GUARDIAN ", false},
		{"profile missing", GuardianRelationshipValues{StudentID: 1, RelationshipType: "PARENT"}, "guardian profile ID is required", "PARENT", "", false},
		{"relationship missing", GuardianRelationshipValues{StudentID: 1, GuardianProfileID: 2}, "relationship type is required", "", "", false},
		{"unknown relationship", GuardianRelationshipValues{StudentID: 1, GuardianProfileID: 2, RelationshipType: "UNKNOWN ", GuardianRole: " LEGAL_GUARDIAN "}, "invalid relationship type", "unknown ", " LEGAL_GUARDIAN ", false},
		{"spaces retained", GuardianRelationshipValues{StudentID: 1, GuardianProfileID: 2, RelationshipType: " PARENT ", GuardianRole: " LEGAL_GUARDIAN "}, "", " parent ", "legal_guardian", true},
		{"default role", GuardianRelationshipValues{StudentID: 1, GuardianProfileID: 2, RelationshipType: "RELATIVE"}, "", "relative", "custom", true},
		{"existing permission object", GuardianRelationshipValues{GuardianProfileID: 2, RelationshipType: "OTHER", PermissionsPresent: true}, "student ID is required", "OTHER", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			value := c.input
			err := NormalizeGuardianRelationship(&value)
			message := ""
			if err != nil {
				message = err.Error()
			}
			if message != c.message || value.RelationshipType != c.relationship || value.GuardianRole != c.role || value.PermissionsPresent != c.permissions {
				t.Fatalf("got error=%q state=%+v; want error=%q relationship=%q role=%q permissions=%t", message, value, c.message, c.relationship, c.role, c.permissions)
			}
		})
	}
}

func TestMapRelationshipTypeNormalizesLabels(t *testing.T) {
	t.Parallel()
	for label, want := range map[string]string{
		" Mutter ": "parent", "VORMUND": "guardian", "Großmutter": "relative",
		"andere": "other", "relative": "relative", "unbekannt": "other", "": "other",
	} {
		if got := MapRelationshipType(label); got != want {
			t.Errorf("MapRelationshipType(%q) = %q, want %q", label, got, want)
		}
	}
}
