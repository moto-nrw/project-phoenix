package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	manualRosterConfirmedVersion     = "1.15.693"
	manualRosterConfirmedDescription = "Regeltermin bleibt von Hand gepflegt: Hinweis auf abweichende Anmeldungen pro Serie ausblenden (#5103)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     manualRosterConfirmedVersion,
		Description: manualRosterConfirmedDescription,
		DependsOn: []string{
			classParentLettersVersion,     // the last migration before this one
			templateOfferingSourceVersion, // the offering source the hint suggests
		},
	})
	Migrations.MustRegister(manualRosterConfirmedUp, manualRosterConfirmedDown)
}

// manualRosterConfirmedUpSQL records that a school keeps a Regeltermin's
// children by hand on purpose (#5103). The flag lives on the series root
// (series_root_id IS NULL); every split segment reads it from there, so a
// later "Ab jetzt dauerhaft" edit keeps the decision.
const manualRosterConfirmedUpSQL = `
		ALTER TABLE activities.groups
			ADD COLUMN IF NOT EXISTS manual_roster_confirmed BOOLEAN NOT NULL DEFAULT false;

		COMMENT ON COLUMN activities.groups.manual_roster_confirmed IS
			'Series root only: the school keeps this Regeltermin''s children by hand on purpose, so the deviation hint stays hidden (#5103).';
`

func manualRosterConfirmedUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, manualRosterConfirmedUpSQL); err != nil {
		return fmt.Errorf("add activities.groups.manual_roster_confirmed: %w", err)
	}
	return nil
}

func manualRosterConfirmedDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `ALTER TABLE activities.groups DROP COLUMN IF EXISTS manual_roster_confirmed;`); err != nil {
		return fmt.Errorf("drop activities.groups.manual_roster_confirmed: %w", err)
	}
	return nil
}
