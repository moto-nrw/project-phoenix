package api

import (
	"encoding/json"
	"testing"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestSessionActivitiesPreserveTimetableWireProjection(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	fixture := testpkg.CreateTestActivityGroup(t, db, "Session projection")
	_, err := NewActiveTestRepositories(testpkg.NewDatabase(t, db), owners.MustNewMembershipTestRepositories(testpkg.NewDatabase(t, db)))
	require.NoError(t, err)
	reposTimetable := NewActiveTestTimetableRows(testpkg.NewDatabase(t, db), owners.MustNewMembershipTestRepositories(testpkg.NewDatabase(t, db)))
	ctx := testpkg.Ctx(t)
	directory := owners.NewSessionActivities(reposTimetable.GroupReads())
	// The activities service serves the retained activity group rows.
	activities, err := timetableCompose.NewService(reposTimetable.Timetable, nil, reposTimetable.Directories, nil, nil, nil)
	require.NoError(t, err)
	want, err := activities.GetGroup(ctx, fixture.ID)
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
	wantRows, err := activities.ListGroups(ctx, &activityContract.GroupListQuery{IDs: []int64{fixture.ID}})
	require.NoError(t, err)
	wantJSON, err = json.Marshal(wantRows)
	require.NoError(t, err)
	gotJSON, err = json.Marshal(rows)
	require.NoError(t, err)
	require.JSONEq(t, string(wantJSON), string(gotJSON))
	listed, err := directory.ListSessionActivities(ctx)
	require.NoError(t, err)
	wantListed, err := activities.ListGroups(ctx, nil)
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
	require.NoError(t, reposTimetable.Timetable.DeleteGroup(ctx, fixture.ID))
	missing, err := directory.FindByID(ctx, fixture.ID)
	require.Nil(t, missing)
	require.ErrorContains(t, err, "database error during find by id")
	var notFound interface{ RepositoryNotFound() }
	require.ErrorAs(t, err, &notFound)
}
