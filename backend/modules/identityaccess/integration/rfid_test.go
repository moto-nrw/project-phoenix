package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestRFIDLookupNormalizesAndIsolatesTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	card := testpkg.CreateTestRFIDCard(t, db, "ABCD1234")
	var observations []identityCompose.Observation
	access, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: func(observation identityCompose.Observation) {
		observations = append(observations, observation)
	}})
	require.NoError(t, err)

	tag := strings.ToLower(card.ID)
	tag = " " + tag[:4] + ":- " + tag[4:] + " "
	id, found, err := access.FindRFIDCard(testpkg.Ctx(t), tag)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, card.ID, id)
	// The import resolves card identity, not device-use eligibility. The old
	// lookup also returned inactive cards, and assigning one must stay possible.
	_, err = db.NewRaw(`UPDATE users.rfid_cards SET active = FALSE WHERE id = ? AND tenant_id = ?`, card.ID, testpkg.Tenant(t)).Exec(context.Background())
	require.NoError(t, err)
	id, found, err = access.FindRFIDCard(testpkg.Ctx(t), card.ID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, card.ID, id)

	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	id, found, err = access.FindRFIDCard(testpkg.ContextForTenant(testpkg.Ctx(t), otherTenant), card.ID)
	require.NoError(t, err)
	require.False(t, found)
	require.Empty(t, id)

	id, found, err = access.FindRFIDCard(testpkg.Ctx(t), "not-a-card")
	require.NoError(t, err)
	require.False(t, found)
	require.Empty(t, id)

	_, _, err = access.FindRFIDCard(context.Background(), card.ID)
	require.ErrorIs(t, err, identityaccess.ErrTenantRequired)
	require.Len(t, observations, 5)
	require.ErrorIs(t, observations[4].Err, identityaccess.ErrTenantRequired)
	require.Zero(t, observations[4].Stats.Queries, "missing tenant must fail before database access")
}
