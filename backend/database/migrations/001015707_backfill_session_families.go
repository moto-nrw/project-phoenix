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
// live chain exactly one new family.
//
// A chain is its unrotated head plus the rotated predecessors kept for
// hand-off recovery. A predecessor names its successor in replacement_token,
// the SHA-256 the successor stores as token, and the successor carries the
// same account and generation + 1; the refresh hand-off checks the same
// (sameAccountSessionLineage). The backfill only follows such a link when
// exactly one row has that token, so it never splits a chain: a row it
// cannot place keeps no family and is refused like before. Expired rows stay
// as they are; the cleanup removes them.
//
// Only rows without family are touched, so a rerun changes nothing. Each
// statement locks the legacy rows it updates and nothing else; logins and
// rotations of other sessions do not wait. The migration connection runs
// without RLS like every data migration of auth.tokens.

// backfillSessionFamilyHeadsSQL gives each live chain head its own family.
const backfillSessionFamilyHeadsSQL = `
UPDATE auth.tokens
SET family_id = gen_random_uuid()::text
WHERE (family_id IS NULL OR family_id = '')
	AND rotated_at IS NULL
	AND expiry > NOW()`

// backfillSessionFamilyPredecessorsSQL hands a family one link down the
// chain, from a successor to its uniquely linked predecessor.
const backfillSessionFamilyPredecessorsSQL = `
UPDATE auth.tokens AS predecessor
SET family_id = successor.family_id
FROM auth.tokens AS successor
WHERE (predecessor.family_id IS NULL OR predecessor.family_id = '')
	AND predecessor.rotated_at IS NOT NULL
	AND predecessor.expiry > NOW()
	AND successor.token = predecessor.replacement_token
	AND successor.family_id IS NOT NULL AND successor.family_id <> ''
	AND successor.account_id = predecessor.account_id
	AND successor.generation = predecessor.generation + 1
	AND (SELECT count(*) FROM auth.tokens AS same WHERE same.token = predecessor.replacement_token) = 1`

// maxBackfillChainDepth bounds the predecessor passes. Rotated rows live
// only within the hand-off grace, so a live chain holds a few of them.
const maxBackfillChainDepth = 64

func backfillSessionFamiliesUp(ctx context.Context, db *bun.DB) error {
	return BackfillSessionFamilies(ctx, db)
}

// BackfillSessionFamilies runs the backfill; the root's tests call it to
// prove that a chain it reaches stays signed in and can be revoked.
func BackfillSessionFamilies(ctx context.Context, db bun.IDB) error {
	if _, err := db.ExecContext(ctx, backfillSessionFamilyHeadsSQL); err != nil {
		return fmt.Errorf("backfill session family heads: %w", err)
	}
	for pass := 0; pass < maxBackfillChainDepth; pass++ {
		result, err := db.ExecContext(ctx, backfillSessionFamilyPredecessorsSQL)
		if err != nil {
			return fmt.Errorf("backfill session family predecessors: %w", err)
		}
		if rows, err := result.RowsAffected(); err != nil || rows == 0 {
			return err
		}
	}
	return nil
}

// backfillSessionFamiliesDown keeps the families: removing them again would
// end the sessions the backfill kept alive.
func backfillSessionFamiliesDown(ctx context.Context, db *bun.DB) error {
	return nil
}
