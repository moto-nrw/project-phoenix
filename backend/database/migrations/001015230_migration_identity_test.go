package migrations

import "testing"

func TestPublishedMigrationVersionsRemainStable(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "phase eligibility columns", got: phaseEligibilityColumnsVersion, want: "1.15.230"},
		{name: "existing-student audience", got: phaseAudienceExistingStudentsVersion, want: "1.15.231"},
		{name: "matched student", got: requestChildrenMatchedStudentVersion, want: "1.15.232"},
		{name: "eligible grade levels", got: phaseEligibleGradeLevelsVersion, want: "1.15.233"},
		{name: "push subscriptions", got: pushSubscriptionsVersion, want: "1.15.234"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("migration version = %q, want %q", test.got, test.want)
			}
		})
	}
}
