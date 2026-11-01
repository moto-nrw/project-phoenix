package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	authTokenSwitchedFromFamilyVersion     = "1.15.725"
	authTokenSwitchedFromFamilyDescription = "Record the origin of a refresh family's school-switch or Absprung lineage (#5422)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     authTokenSwitchedFromFamilyVersion,
		Description: authTokenSwitchedFromFamilyDescription,
		DependsOn: []string{
			staffQualificationTypesVersion, // the last migration before this one
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
// Every family issued after it names at least itself, so NULL marks a row
// from before: its switch stays unlinked, and the application ends such a
// family that may descend from a switch one refresh lifetime after the
// deploy instead of renewing it (capLegacyLineage). lineage_deadline carries
// that transition deadline along the lineage, through rotations and further
// switches, apart from the retirement grace in family_expiry_cap.
func authTokenSwitchedFromFamilyUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		ALTER TABLE auth.tokens
			ADD COLUMN IF NOT EXISTS switched_from_family_id VARCHAR(100),
			ADD COLUMN IF NOT EXISTS lineage_deadline TIMESTAMPTZ;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("add refresh-token switch source family: %w", err)
	}
	return nil
}

func authTokenSwitchedFromFamilyDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		ALTER TABLE auth.tokens
			DROP COLUMN IF EXISTS lineage_deadline,
			DROP COLUMN IF EXISTS switched_from_family_id;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("drop refresh-token switch source family: %w", err)
	}
	return nil
}
