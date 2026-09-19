package gradetransition

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingCohortDirectory answers the class listing but fails every cohort read
// the per-class counts are derived from. The embedded nil Directory panics if
// the workflow ever reaches another owner command, so the test cannot pass by
// taking a different path.
type failingCohortDirectory struct {
	Directory
	classes []string
	err     error
}

func (d failingCohortDirectory) ListSchoolClasses(context.Context) ([]string, error) {
	return d.classes, nil
}

func (d failingCohortDirectory) ListStudentsByClasses(
	context.Context, []string,
) ([]peopledirectory.Student, error) {
	return nil, d.err
}

// TestSuggestMappingsPropagatesCountError covers the #405 review fix: a failed
// per-class student count must fail the whole suggestion instead of silently
// dropping the class — an admin could otherwise create a transition that omits
// an affected cohort without ever seeing an error.
func TestSuggestMappingsPropagatesCountError(t *testing.T) {
	t.Parallel()

	countUnavailable := errors.New("student count unavailable")
	workflow := &Workflow{deps: Dependencies{
		UnitOfWork: func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
		Authorize: func(context.Context, string) (Actor, error) {
			return Actor{TenantID: 4711, AccountID: 4712}, nil
		},
		Observe:   func(Observation) {},
		Directory: failingCohortDirectory{classes: []string{"3a"}, err: countUnavailable},
	}}

	suggestions, err := workflow.SuggestMappings(context.Background())
	require.ErrorIs(t, err, countUnavailable)
	assert.Nil(t, suggestions)
}

// TestSuggestMappingsSkipsEmptyClasses pins the other half of that contract:
// a class the count reports as empty produces no suggestion at all.
func TestSuggestMappingsSkipsEmptyClasses(t *testing.T) {
	t.Parallel()

	suggestions := suggestMappings([]string{"1a", "4b", "leer"}, map[string]int{"1a": 2, "4b": 1})
	require.Len(t, suggestions, 2)
	assert.Equal(t, "1a", suggestions[0].FromClass)
	require.NotNil(t, suggestions[0].ToClass)
	assert.Equal(t, "2a", *suggestions[0].ToClass)
	assert.Equal(t, "4b", suggestions[1].FromClass)
	assert.True(t, suggestions[1].IsGraduating)
}
