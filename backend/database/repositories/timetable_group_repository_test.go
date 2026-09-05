package repositories_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimetableGroupAdapterPreservesDatabaseErrorBehavior(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repository := repositories.NewFactory(db).ActivityGroup

	group, err := repository.FindByID(testpkg.Ctx(t), int64(9_223_372_036_854_775_000))

	assert.Nil(t, group)
	require.Error(t, err)
	assert.ErrorContains(t, err, "database error during find by id")
	var notFound interface{ RepositoryNotFound() }
	require.ErrorAs(t, err, &notFound)
}
