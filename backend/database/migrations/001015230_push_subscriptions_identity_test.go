package migrations

import "testing"

func TestMigrationVersionCollisionRecovery(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "shipped push subscriptions", got: pushSubscriptionsVersion, want: "1.15.230"},
		{name: "phase eligibility columns", got: phaseEligibilityColumnsVersion, want: "1.15.234"},
		{name: "existing-student audience", got: phaseAudienceExistingStudentsVersion, want: "1.15.235"},
		{name: "matched student", got: requestChildrenMatchedStudentVersion, want: "1.15.236"},
		{name: "eligible grade levels", got: phaseEligibleGradeLevelsVersion, want: "1.15.237"},
		{name: "push subscriptions reconciliation", got: reconcilePushSubscriptionsVersion, want: "1.15.238"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("migration version = %q, want %q", test.got, test.want)
			}
		})
	}
}
