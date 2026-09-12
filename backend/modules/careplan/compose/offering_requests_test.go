package compose

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestOfferingRequestQueriesPreserveTenantCursorAndEmptyScope(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	studentID, childID, accountID := insertOfferingChangeFixture(t, db, ctx, testpkg.Tenant(t))
	created, err := module.CreateOfferingChange(ctx, careplan.OfferingChangeRequest{
		StudentID: studentID, RequestChildID: childID, SubmittedBy: accountID,
		Payload: json.RawMessage(`{"offerings":[]}`), EffectiveFrom: "2030-09-01", Status: "pending",
	})
	require.NoError(t, err)
	var observations []Observation
	query, err := NewOfferingChangeRequestQueries(db, func(value Observation) { observations = append(observations, value) })
	require.NoError(t, err)
	rows, err := query.ListOfferingChanges(ctx, careplan.OfferingChangeFilter{Statuses: []string{"pending"}, Limit: 1})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, created.ID, rows[0].ID)
	require.Equal(t, created.Payload, rows[0].Payload)
	require.Equal(t, int64(1), observations[len(observations)-1].Stats.Queries)
	rows, err = query.ListOfferingChanges(ctx, careplan.OfferingChangeFilter{BeforeInstant: created.CreatedAt, BeforeID: created.ID, Limit: 1})
	require.NoError(t, err)
	require.Empty(t, rows)
	rows, err = query.ListOfferingChanges(ctx, careplan.OfferingChangeFilter{StudentIDs: []int64{}, Limit: 1})
	require.NoError(t, err)
	require.Empty(t, rows)
	require.Zero(t, observations[len(observations)-1].Stats.Queries)
	foreign := tenantContext(t, db, testpkg.UniqueTestTenantID(t))
	rows, err = query.ListOfferingChanges(foreign, careplan.OfferingChangeFilter{IDs: []int64{created.ID}, Limit: 1})
	require.NoError(t, err)
	require.Empty(t, rows)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = query.ListOfferingChanges(canceled, careplan.OfferingChangeFilter{Limit: 1})
	require.ErrorIs(t, err, context.Canceled)
	_, err = NewOfferingChangeRequestQueries(nil, func(Observation) {})
	require.Error(t, err)
	_, err = NewOfferingChangeRequestQueries(db, nil)
	require.Error(t, err)
}
