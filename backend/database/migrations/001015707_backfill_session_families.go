package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	backfillSessionFamiliesVersion     = "1.15.707"
	backfillSessionFamiliesDescription = "Give every live refresh chain without family one family (#5194)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     backfillSessionFamiliesVersion,
		Description: backfillSessionFamiliesDescription,
		DependsOn: []string{
			holidayPeriodFormsVersion, // the last migration before this one
			AuthTokenFamilyVersion,    // family_id and generation
			hashStoredTokensVersion,   // token and replacement_token as SHA-256
			refreshTokenRotationRecoveryVersion,
		},
	})
	Migrations.MustRegister(backfillSessionFamiliesUp, backfillSessionFamiliesDown)
}

// Every authenticated request now checks the refresh-token family of its
// access token (#5194). A chain begun before 1.4.7 carries no family_id, and
// rotation copies the family of the predecessor, so a chain in use ever since
// still has none and would have to sign in again. This backfill gives each
// live chain exactly one new family, in one statement.
//
// A chain is its unrotated head plus the rotated predecessors kept for
// hand-off recovery. A predecessor names its successor in replacement_token,
// the SHA-256 the successor stores as token, and the successor carries the
// same account and generation + 1; the refresh hand-off checks the same
// (sameAccountSessionLineage). The recursive walk starts at every live head
// without family, gives it a new family and hands that family down each such
// link, however long the chain. It follows a link only when exactly one row
// has that token, so it never splits a chain. The generation falls by one
// with every step, so the walk ends and a cycle cannot loop; UNION drops a
// row reached twice. A row it cannot place keeps no family: its access
// tokens are refused, its refresh is refused, and a logout presenting it
// revokes the whole chain it links into (domain.SessionChain), as the
// runtime does for every row without family. Expired rows stay as they are;
// the cleanup removes them.
//
// A session that rotates while the statement runs is handled by the
// rotation itself: it gives a chain without family a family and writes it
// onto the predecessor too. The UPDATE rechecks the family on the locked row
// version, so it never overwrites the family such a rotation committed, and
// the new head, which the statement's snapshot does not see, already has
// one.
//
// Only rows without family are touched, so a rerun finds nothing to do. The
// statement locks the legacy rows it updates and nothing else; logins and
// rotations of other sessions do not wait. The migration connection runs
// without RLS like every data migration of auth.tokens, and no tenant or
// portal column changes.

const backfillSessionFamiliesSQL = `
WITH RECURSIVE heads AS MATERIALIZED (
	SELECT id, token, account_id, generation, gen_random_uuid()::text AS family_id
	FROM auth.tokens
	WHERE (family_id IS NULL OR family_id = '')
		AND rotated_at IS NULL
		AND expiry > NOW()
), chain AS (
	SELECT id, token, account_id, generation, family_id FROM heads
	UNION
	SELECT predecessor.id, predecessor.token, predecessor.account_id, predecessor.generation, chain.family_id
	FROM auth.tokens AS predecessor
	JOIN chain ON predecessor.replacement_token = chain.token
	WHERE (predecessor.family_id IS NULL OR predecessor.family_id = '')
		AND predecessor.rotated_at IS NOT NULL
		AND predecessor.expiry > NOW()
		AND predecessor.account_id = chain.account_id
		AND predecessor.generation = chain.generation - 1
		AND (SELECT count(*) FROM auth.tokens AS same WHERE same.token = predecessor.replacement_token) = 1
)
UPDATE auth.tokens
SET family_id = chain.family_id
FROM chain
WHERE auth.tokens.id = chain.id
	AND (auth.tokens.family_id IS NULL OR auth.tokens.family_id = '')`

func backfillSessionFamiliesUp(ctx context.Context, db *bun.DB) error {
	return BackfillSessionFamilies(ctx, db)
}

// BackfillSessionFamilies runs the backfill; the root's tests call it to
// prove that a chain it reaches stays signed in and can be revoked.
func BackfillSessionFamilies(ctx context.Context, db bun.IDB) error {
	if _, err := db.ExecContext(ctx, backfillSessionFamiliesSQL); err != nil {
		return fmt.Errorf("backfill session families: %w", err)
	}
	return nil
}

// backfillSessionFamiliesDown is forward-only on purpose. Down cannot tell a
// backfilled family from one a login minted, and clearing families would only
// send the backfilled chains back to a new sign-in; it would restore no
// state anyone depends on. Keeping them is harmless: an older backend ignores
// family_id on access and keeps copying it on rotation.
func backfillSessionFamiliesDown(ctx context.Context, db *bun.DB) error {
	return nil
}
