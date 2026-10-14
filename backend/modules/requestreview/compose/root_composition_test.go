package compose_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	requestreviewcompose "github.com/moto-nrw/project-phoenix/modules/requestreview/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The typed root composes the owners the internal review tests run over. It
// imports this package, so the binding lives in the external test package.
func init() {
	requestreviewcompose.ComposeReviewSources = func(t *testing.T) (*bun.DB, requestreviewcompose.ReviewComposition) {
		t.Helper()
		db := testpkg.SetupTestDB(t)
		feedback, err := api.NewFeedbackTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
		require.NoError(t, err)
		module, err := api.NewStudentTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), feedback.Feedback)
		require.NoError(t, err)
		return db, requestreviewcompose.ReviewComposition{
			PeopleDirectory: module.PeopleDirectory, FamilyProtection: requestreviewcompose.NewFamilyProtection(module.PeopleDirectory),
			Settings: module.Settings, ExcusedRequests: module.ExcusedRequests,
		}
	}
}
