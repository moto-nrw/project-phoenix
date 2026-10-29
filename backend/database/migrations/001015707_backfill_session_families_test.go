package migrations

// Coverage for 1.15.707 (#5194): every live refresh chain without family gets
// exactly one family in one run, head and linked predecessors alike, however
// long it is; a predecessor without family takes the family of its head;
// rows the backfill cannot place, expired rows and rows with a family stay
// as they are, and a rerun changes nothing.

import (
	"context"
	"fmt"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackfillSessionFamilies(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	account := testpkg.CreateTestAccount(t, db, "backfill-session-families")
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	hash := func(raw string) string { return testpkg.TokenHash(t, db, raw) }
	future, past := time.Now().Add(time.Hour), time.Now().Add(-time.Hour)
	insert := func(raw string, generation int, family any, rotatedTo string, expiry time.Time) int64 {
		t.Helper()
		var replacement any
		var rotatedAt any
		if rotatedTo != "" {
			replacement, rotatedAt = hash(rotatedTo), time.Now()
		}
		var id int64
		require.NoError(t, db.NewRaw(`INSERT INTO auth.tokens
			(account_id, tenant_id, token, expiry, family_id, generation, rotated_at, replacement_token, portal_scope)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'tenant') RETURNING id`,
			account.ID, tenantID, hash(raw), expiry, family, generation, rotatedAt, replacement).Scan(ctx, &id))
		return id
	}
	familyOf := func(id int64) string {
		t.Helper()
		var family *string
		require.NoError(t, db.NewRaw(`SELECT family_id FROM auth.tokens WHERE id = ?`, id).Scan(ctx, &family))
		if family == nil {
			return ""
		}
		return *family
	}

	// A legacy chain of three generations: two rotated predecessors, one head.
	chainOld := insert("chain-g0", 0, nil, "chain-g1", future)
	chainMid := insert("chain-g1", 1, "", "chain-g2", future)
	chainHead := insert("chain-g2", 2, nil, "", future)
	// A second legacy chain of one row.
	single := insert("single", 0, nil, "", future)
	// A predecessor whose successor is gone cannot be placed.
	orphan := insert("orphan", 0, nil, "missing-successor", future)
	// A predecessor whose successor token is not unique cannot be placed.
	ambiguous := insert("ambiguous", 0, nil, "twin", future)
	twinA := insert("twin", 1, nil, "", future)
	insert("twin", 1, nil, "", future)
	// A link with the wrong generation is no lineage.
	skipped := insert("skip-g0", 0, nil, "skip-g2", future)
	skipHead := insert("skip-g2", 2, nil, "", future)
	// A rotation that committed before the backfill read the chain left a
	// family on the new head and its predecessor; the older predecessor
	// takes the same family.
	rotatedOld := insert("rotated-g0", 0, nil, "rotated-g1", future)
	rotatedMid := insert("rotated-g1", 1, "family-rotated", "rotated-g2", future)
	rotatedHead := insert("rotated-g2", 2, "family-rotated", "", future)
	// Expired rows and rows with a family stay as they are.
	expired := insert("expired", 0, nil, "", past)
	modern := insert("modern", 0, "family-modern", "", future)

	require.NoError(t, backfillSessionFamiliesUp(ctx, db))

	head := familyOf(chainHead)
	require.NotEmpty(t, head)
	assert.Equal(t, head, familyOf(chainMid), "the chain keeps one family")
	assert.Equal(t, head, familyOf(chainOld), "the chain keeps one family")
	assert.NotEmpty(t, familyOf(single))
	assert.NotEqual(t, head, familyOf(single), "each chain gets its own family")
	assert.Empty(t, familyOf(orphan))
	assert.Empty(t, familyOf(ambiguous), "a link that is not unique is not followed")
	assert.NotEmpty(t, familyOf(twinA), "each head gets a family")
	assert.Empty(t, familyOf(skipped), "a link across a generation gap is not followed")
	assert.NotEmpty(t, familyOf(skipHead))
	assert.Empty(t, familyOf(expired))
	assert.Equal(t, "family-modern", familyOf(modern))
	assert.Equal(t, "family-rotated", familyOf(rotatedOld), "a predecessor takes the family of its chain")
	assert.Equal(t, "family-rotated", familyOf(rotatedMid))
	assert.Equal(t, "family-rotated", familyOf(rotatedHead))

	before := map[int64]string{}
	for _, id := range []int64{chainOld, chainMid, chainHead, single, orphan, ambiguous, twinA, skipped, skipHead, expired, modern, rotatedOld, rotatedMid, rotatedHead} {
		before[id] = familyOf(id)
	}
	require.NoError(t, backfillSessionFamiliesUp(ctx, db))
	for id, family := range before {
		assert.Equal(t, family, familyOf(id), fmt.Sprintf("a rerun keeps row %d", id))
	}
}

// TestBackfillSessionFamiliesLongChain pins that a chain far longer than any
// fixed depth gets one family in a single run, and that a second run
// changes nothing.
func TestBackfillSessionFamiliesLongChain(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	account := testpkg.CreateTestAccount(t, db, "backfill-long-chain")
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	const length = 200
	_, err := db.ExecContext(ctx, `
		INSERT INTO auth.tokens (account_id, tenant_id, token, expiry, generation, rotated_at, replacement_token, portal_scope)
		SELECT ?, ?, encode(sha256(convert_to('long-' || g, 'UTF8')), 'hex'), NOW() + INTERVAL '1 hour', g,
			CASE WHEN g < ? THEN NOW() END,
			CASE WHEN g < ? THEN encode(sha256(convert_to('long-' || (g + 1), 'UTF8')), 'hex') END,
			'tenant'
		FROM generate_series(0, ?) AS g`, account.ID, tenantID, length-1, length-1, length-1)
	require.NoError(t, err)

	families := func() (distinct, missing int) {
		t.Helper()
		require.NoError(t, db.NewRaw(`SELECT count(DISTINCT family_id), count(*) FILTER (WHERE family_id IS NULL OR family_id = '')
			FROM auth.tokens WHERE account_id = ?`, account.ID).Scan(ctx, &distinct, &missing))
		return distinct, missing
	}
	require.NoError(t, backfillSessionFamiliesUp(ctx, db))
	distinct, missing := families()
	assert.Equal(t, 1, distinct, "the whole chain shares one family")
	assert.Zero(t, missing, "every one of the %d rows is placed in one run", length)

	var before string
	require.NoError(t, db.NewRaw(`SELECT string_agg(family_id, ',' ORDER BY id) FROM auth.tokens WHERE account_id = ?`, account.ID).Scan(ctx, &before))
	require.NoError(t, backfillSessionFamiliesUp(ctx, db))
	var after string
	require.NoError(t, db.NewRaw(`SELECT string_agg(family_id, ',' ORDER BY id) FROM auth.tokens WHERE account_id = ?`, account.ID).Scan(ctx, &after))
	assert.Equal(t, before, after, "a second run changes nothing")
}
