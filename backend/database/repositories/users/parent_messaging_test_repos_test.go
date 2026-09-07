package users_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// parentRepos composes only the parent-conversation stores.
//
// It deliberately avoids repositories.NewFactory: the factory is shrink-only
// under #2580, so a test that needs three repositories must not add another
// caller that builds the whole legacy graph.
func parentRepos(tb testing.TB, db *bun.DB) repositories.ParentMessagingTestRepositories {
	tb.Helper()
	repos, err := repositories.NewParentMessagingTestRepositories(db)
	require.NoError(tb, err)
	return repos
}
