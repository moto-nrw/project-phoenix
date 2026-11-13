package behaviortest

import (
	"context"
	"testing"

	reviewbindings "github.com/moto-ogs/project-phoenix/roots/requestreviewbinding"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/api"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// composeReviewDecorations builds the student directory and the correction log
// over the owners the typed root composes.
func composeReviewDecorations(t *testing.T, db *bun.DB, people reviewbindings.ReviewPeople, allowed func(context.Context) bool) (ReviewDecorations, error) {
	t.Helper()
	groups, err := owners.NewSchoolStructure(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	students, err := reviewbindings.NewStudentDirectory(people, groups)
	if err != nil {
		return ReviewDecorations{}, err
	}
	queries, err := api.NewUnobservedCorrectionQueries(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	corrections, err := reviewbindings.NewCorrectionLog(queries, people, allowed)
	return ReviewDecorations{Students: students, Corrections: corrections}, err
}

// composeReviewSources composes the root's student graph over the package's
// test database and runs proof over it.
func composeReviewSources(t *testing.T, proof ReviewProof) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	feedback, err := api.NewFeedbackTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	module, _, people, _, _, _, err := api.NewStudentTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), feedback.Feedback)
	require.NoError(t, err)
	liveSettings := api.NewActiveTestSettings(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	svc := ReviewComposition{
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

func ignoreReviewObservation[T any](T) {}
