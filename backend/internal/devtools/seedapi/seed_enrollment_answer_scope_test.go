package api

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The submit keeps a child's answer only for a question asked per child and
// an enrollment answer only for a question asked once per enrollment; every
// other key is dropped without an error (#5027). Every seeded answer must
// therefore belong to a question of its own scope, or the demo silently loses
// it, like Ama Mensah's allergy did.
func TestSeedEnrollmentAnswersMatchTheScopeOfTheirQuestion(t *testing.T) {
	t.Parallel()

	offerings := map[string]int64{"ogs-ganztag": 3, "ogs-kurz": 1, "mittagessen": 2, "ag-fussball": 5, "ag-theater": 6, "ferienbetreuung": 4}
	step := parentEnrollmentSeedStep{}
	ganztag := seedEnrollmentFormFields(nil)
	cases := []struct {
		name   string
		fields []map[string]any
		body   map[string]any
	}{
		{"Anmeldeformular Ganztag, website request", ganztag, step.enrollmentSubmissionWithDays(1, offerings, "Luka", "Berisha", "2016-08-21", 4,
			"Arta", "Berisha", "arta.berisha@example.test", "public-reject", []int64{3, 1, 2}, nil)},
		{"Anmeldeformular Ganztag, income proof", ganztag, seedIncomeProofSubmission(1, offerings, "upload-token")},
		{"Ferienbetreuung mit Unterschrift", seedSignedEnrollmentFormFields(), seedSignedEnrollmentSubmission(1, "signature")},
	}
	// Regular bookings and every other Ganztag request answer the child
	// questions through the same helper, once per demo child.
	for _, child := range DemoStudents {
		cases = append(cases, struct {
			name   string
			fields []map[string]any
			body   map[string]any
		}{"Anmeldeformular Ganztag, " + child.FirstName + " " + child.LastName, ganztag, map[string]any{
			"children": []map[string]any{{"custom_data": seedEnrollmentChildAnswers(child.FirstName, child.LastName)}},
		}})
	}

	for _, tc := range cases {
		perChild, perEnrollment := seedQuestionKeysByScope(tc.fields)
		if answers, ok := tc.body["custom_data"].(map[string]any); ok {
			for key := range answers {
				assert.Contains(t, perEnrollment, key, "%s: enrollment answer %q has no question asked once per enrollment", tc.name, key)
			}
		}
		children, ok := tc.body["children"].([]map[string]any)
		require.True(t, ok, tc.name)
		for _, child := range children {
			answers, _ := child["custom_data"].(map[string]any)
			for key := range answers {
				assert.Contains(t, perChild, key, "%s: child answer %q has no question asked per child", tc.name, key)
			}
		}
	}
}

// Only the two forms above are created by the seeder; a new one needs its
// own case in the scope test.
func TestSeedCreatesOnlyTheFormsTheAnswerScopeTestCovers(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	var creators []string
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, err := os.ReadFile(file)
		require.NoError(t, err)
		for range strings.Count(string(source), `"/api/enrollment/schema/"`) {
			creators = append(creators, file)
		}
	}
	sort.Strings(creators)
	assert.Equal(t, []string{"parent_enrollment.go", "seed_enrollment_signature.go"}, creators)
}

// seedQuestionKeysByScope splits the answer-collecting questions of a form
// into those asked per child and those asked once per enrollment.
func seedQuestionKeysByScope(fields []map[string]any) (perChild, perEnrollment []string) {
	for _, field := range fields {
		if field["type"] == "information" {
			continue
		}
		key, _ := field["key"].(string)
		if appliesToChild, _ := field["applies_to_child"].(bool); appliesToChild {
			perChild = append(perChild, key)
		} else {
			perEnrollment = append(perEnrollment, key)
		}
	}
	return perChild, perEnrollment
}
