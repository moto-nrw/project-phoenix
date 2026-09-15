package education_test

import (
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSuggestMappingsQueryBudget(t *testing.T) {
	t.Parallel()
	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()
	add := func(from, to int) {
		for i := from; i < to; i++ {
			testpkg.CreateTestStudent(t, db, fmt.Sprintf("Budget%d", i), "Transition", fmt.Sprintf("%da", i+1))
		}
	}
	add(0, 3)
	counter := testpkg.CaptureQueriesForContext(t, db)
	ctx := counter.Context(testpkg.Ctx(t))
	run := func() []string {
		counter.Reset()
		_, err := service.SuggestMappings(ctx)
		require.NoError(t, err)
		return counter.Operation("SELECT")
	}
	small := run()
	add(3, 8)
	large := run()
	assert.Equal(t, len(small), len(large))
	testpkg.AssertQueryBudget(t, "services.education.suggest_mappings.reads", large)
}
