package compose

import (
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestStudentFieldReviewPreservesBulkAndBaselineFacts(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Field", "Review", "1a")
	_, err := module.UpdatePerson(ctx, peopledirectory.UpdatePerson{ID: student.PersonID, FirstName: "Field", LastName: "Review"})
	require.NoError(t, err)
	require.NoError(t, module.ApplyEnrollmentProfile(ctx, student.ID, peopledirectory.EnrollmentProfilePatch{
		DepartureSet:          true,
		AllowedDepartureModes: map[string][]string{"mon": {"alone", "bus"}},
	}))

	for _, tc := range []struct {
		name, target, field, oldValue, newValue string
		eligible                                bool
		reason                                  string
		changed                                 *bool
	}{
		{"name", "person", "first_name", `"Field"`, `"New"`, true, "", new(false)},
		{"stale name", "person", "first_name", `"Earlier"`, `"New"`, false, "stale", new(true)},
		{"invalid name", "person", "first_name", `"Field"`, `""`, false, "single_only", new(false)},
		{"class", "student", "school_class", `"1a"`, `"2a"`, true, "", new(false)},
		{"unknown field", "student", "unknown", `null`, `"New"`, false, "single_only", nil},
		{"invalid birthday", "person", "birthday", `null`, `"2026-02-30"`, false, "single_only", new(false)},
		{"valid birthday", "person", "birthday", `null`, `"2024-02-29"`, true, "", new(false)},
		{"departure order", "departure", "allowed_departure_modes", `{"mon":["bus","alone"]}`, `{"mon":["pickup"]}`, true, "", new(false)},
		{"departure stale", "departure", "allowed_departure_modes", `{"mon":["alone"]}`, `{"mon":["pickup"]}`, false, "stale", new(true)},
		{"departure companion", "departure", "allowed_departure_modes", `{"mon":["bus","alone"]}`, `{"mon":["accompanied"]}`, false, "single_only", new(false)},
		{"departure invalid baseline", "departure", "allowed_departure_modes", `{"sat":["alone"]}`, `{}`, false, "single_only", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reviews, err := module.ReviewStudentFields(ctx, []peopledirectory.StudentFieldChange{{
				RequestID: student.ID, StudentID: student.ID, Target: tc.target, Field: tc.field,
				OldValue: json.RawMessage(tc.oldValue), NewValue: json.RawMessage(tc.newValue),
			}})
			require.NoError(t, err)
			require.Len(t, reviews, 1)
			review := reviews[student.ID]
			require.Equal(t, student.ID, review.Student.ID)
			require.Equal(t, "Field", review.FirstName)
			require.Equal(t, tc.eligible, review.BulkEligible)
			require.Equal(t, tc.reason, review.BulkIneligibleReason)
			if tc.changed != nil {
				require.NotNil(t, review.CurrentValueChanged)
				require.Equal(t, *tc.changed, *review.CurrentValueChanged)
			} else {
				require.Nil(t, review.CurrentValueChanged)
			}
		})
	}
	person, err := module.FindPerson(ctx, student.PersonID)
	require.NoError(t, err)
	require.Equal(t, "Field", person.FirstName, "reviewing must not apply any proposed field")
}
