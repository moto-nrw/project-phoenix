package compose_test

import (
	"context"
	"testing"

	reviewbindings "github.com/moto-nrw/project-phoenix/roots/requestreviewbinding"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	requestreviewcompose "github.com/moto-nrw/project-phoenix/modules/requestreview/compose"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The typed root composes the owners the internal review tests run over. It
// imports this package, so the binding lives in the external test package.
func init() {
	requestreviewcompose.ComposeReviewDecorations = func(t *testing.T, db *bun.DB, people reviewbindings.ReviewPeople, allowed func(context.Context) bool) (requestreviewcompose.ReviewDecorations, error) {
		t.Helper()
		groups, err := owners.NewSchoolStructure(testpkg.NewDatabase(t, db))
		require.NoError(t, err)
		students, err := reviewbindings.NewStudentDirectory(people, groups)
		if err != nil {
			return requestreviewcompose.ReviewDecorations{}, err
		}
		queries, err := api.NewUnobservedCorrectionQueries(testpkg.NewDatabase(t, db))
		require.NoError(t, err)
		corrections, err := reviewbindings.NewCorrectionLog(queries, people, allowed)
		return requestreviewcompose.ReviewDecorations{Students: students, Corrections: corrections}, err
	}
	requestreviewcompose.ComposeReviewSources = func(t *testing.T, proof requestreviewcompose.ReviewProof) {
		t.Helper()
		db := testpkg.SetupTestDB(t)
		feedback, err := api.NewFeedbackTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
		require.NoError(t, err)
		module, _, people, _, _, _, err := api.NewStudentTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), feedback.Feedback)
		require.NoError(t, err)
		liveSettings := api.NewActiveTestSettings(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
		svc := requestreviewcompose.ReviewComposition{
			PeopleDirectory: people.PeopleDirectory, FamilyProtection: reviewbindings.NewFamilyProtection(people.PeopleDirectory),
			Settings: liveSettings.Settings, ExcusedRequests: module.ExcusedRequests,
		}
		masterData, err := api.NewRequestReviewMasterData(testpkg.NewDatabase(t, db),
			testpkg.StudentDirectory{},
			svc.PeopleDirectory,
			proof.Scope,
			proof.Today,
			ignoreReviewObservation)
		require.NoError(t, err)
		careReviews, err := api.NewRequestReviewSchedules(testpkg.NewDatabase(t, db),
			testpkg.StudentDirectory{},
			svc.PeopleDirectory,
			proof.Scope,
			func(ctx context.Context) (bool, error) {
				return svc.Settings.ResolveBool(ctx, "enrollment.bookings_authoritative")
			},
			proof.Today,
			ignoreReviewObservation,
			ignoreReviewObservation,
			proof)
		require.NoError(t, err)
		offeringReviews, err := api.NewRequestReviewOfferings(testpkg.NewDatabase(t, db),
			svc.PeopleDirectory,
			proof.Scope,
			proof.Today,
			ignoreReviewObservation,
			ignoreReviewObservation)
		require.NoError(t, err)

		proof.Run(db, svc, masterData, careReviews, offeringReviews)
	}
}

func ignoreReviewObservation[T any](T) {}
