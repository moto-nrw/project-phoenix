package compose_test

import (
	"context"
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
	requestreviewcompose.ComposeReviewDecorations = func(t *testing.T, db *bun.DB, people requestreviewcompose.ReviewPeople, allowed func(context.Context) bool) (requestreviewcompose.ReviewDecorations, error) {
		t.Helper()
		groups, err := api.NewSchoolStructure(testpkg.NewDatabase(t, db))
		require.NoError(t, err)
		students, err := requestreviewcompose.NewStudentDirectory(people, groups)
		if err != nil {
			return requestreviewcompose.ReviewDecorations{}, err
		}
		queries, err := api.NewUnobservedCorrectionQueries(testpkg.NewDatabase(t, db))
		require.NoError(t, err)
		corrections, err := requestreviewcompose.NewCorrectionLog(queries, people, allowed)
		return requestreviewcompose.ReviewDecorations{Students: students, Corrections: corrections}, err
	}
	requestreviewcompose.ComposeReviewSources = func(t *testing.T) (*bun.DB, requestreviewcompose.ReviewComposition) {
		t.Helper()
		db := testpkg.SetupTestDB(t)
		feedback, err := api.NewFeedbackTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
		require.NoError(t, err)
		module, live, people, _, _, _, err := api.NewStudentTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), feedback.Feedback)
		require.NoError(t, err)
		return db, requestreviewcompose.ReviewComposition{
			PeopleDirectory: people.PeopleDirectory, FamilyProtection: requestreviewcompose.NewFamilyProtection(people.PeopleDirectory),
			Settings: live.Settings, ExcusedRequests: module.ExcusedRequests,
		}
	}
}
