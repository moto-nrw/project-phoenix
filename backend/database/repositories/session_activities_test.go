package repositories_test

import (
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestSessionActivitiesPreserveTimetableWireProjection(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	fixture := testpkg.CreateTestActivityGroup(t, db, "Session projection")
	repos, err := repositories.NewActiveTestRepositories(db)
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	directory := repositories.NewSessionActivities(repos.ActivityGroup)
	want, err := repos.ActivityGroup.FindByID(ctx, fixture.ID)
	require.NoError(t, err)
	got, err := directory.FindByID(ctx, fixture.ID)
	require.NoError(t, err)
	wantJSON, err := json.Marshal(want)
	require.NoError(t, err)
	gotJSON, err := json.Marshal(got)
	require.NoError(t, err)
	require.JSONEq(t, string(wantJSON), string(gotJSON))
	rows, err := directory.FindByIDs(ctx, []int64{fixture.ID})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	wantRows, err := repos.ActivityGroup.FindByIDs(ctx, []int64{fixture.ID})
	require.NoError(t, err)
	wantJSON, err = json.Marshal(wantRows)
	require.NoError(t, err)
	gotJSON, err = json.Marshal(rows)
	require.NoError(t, err)
	require.JSONEq(t, string(wantJSON), string(gotJSON))
	listed, err := directory.ListSessionActivities(ctx)
	require.NoError(t, err)
	wantListed, err := repos.ActivityGroup.ListWithCategory(ctx, nil)
	require.NoError(t, err)
	wantJSON, err = json.Marshal(wantListed)
	require.NoError(t, err)
	gotJSON, err = json.Marshal(listed)
	require.NoError(t, err)
	require.JSONEq(t, string(wantJSON), string(gotJSON))
	empty, err := directory.FindByIDs(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, empty)
	require.Empty(t, empty)
	require.NoError(t, repos.ActivityGroup.Delete(ctx, fixture.ID))
	missing, err := directory.FindByID(ctx, fixture.ID)
	require.Nil(t, missing)
	require.ErrorContains(t, err, "database error during find by id")
	var notFound interface{ RepositoryNotFound() }
	require.ErrorAs(t, err, &notFound)
}
