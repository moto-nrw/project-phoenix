package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	authTokenSwitchedFromFamilyVersion     = "1.15.721"
	authTokenSwitchedFromFamilyDescription = "Record the family a school switch minted its refresh family from (#5422)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     authTokenSwitchedFromFamilyVersion,
		Description: authTokenSwitchedFromFamilyDescription,
		DependsOn: []string{
			parentConferencesVersion,       // the last migration before this one
			AuthTokenFamilyVersion,         // family_id the new column names
			backfillSessionFamiliesVersion, // every live chain carries a family
		},
	})
	Migrations.MustRegister(authTokenSwitchedFromFamilyUp, authTokenSwitchedFromFamilyDown)
}

// A school switch mints a family of its own and retires the presented one
// with a grace; a Träger Absprung opens a school family out of the Träger
// family. switched_from_family_id retains the origin of that lineage, so a
// logout revokes it even after the origin's rows are gone (#5422).
// Rows from before this migration carry NULL: their switches stay unlinked,
// as they were, and their source families end with the grace.
func authTokenSwitchedFromFamilyUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		ALTER TABLE auth.tokens
			ADD COLUMN IF NOT EXISTS switched_from_family_id VARCHAR(100);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("add refresh-token switch source family: %w", err)
	}
	return nil
}

func authTokenSwitchedFromFamilyDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		ALTER TABLE auth.tokens
			DROP COLUMN IF EXISTS switched_from_family_id;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("drop refresh-token switch source family: %w", err)
	}
	return nil
}
