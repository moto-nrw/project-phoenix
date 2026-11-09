package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const declarationAlwaysRevocableVersion = "1.15.846"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     declarationAlwaysRevocableVersion,
		Description: "Einverständnisse sind immer widerrufbar: Einstellung declaration_revocable entfällt (#6303)",
		DependsOn: []string{
			demoVisitorErasureOutboxKindsVersion, // the newest migration before this one
		},
	})
	Migrations.MustRegister(declarationAlwaysRevocableUp, declarationAlwaysRevocableDown)
}

// declarationAlwaysRevocableUpSQL drops the school setting "Widerruf erlauben"
// (#6303). The only kind of Erklärung is a consent, and a consent can always
// be withdrawn, as easily as it was given (Art. 7 Abs. 3 DSGVO); a
// confirmation of reading is the Elternbrief's Lesebestätigung, not a kind.
// No proof hashes the setting (record and content hashes cover the
// submission and the frozen version only), so dropping it changes no stored
// proof. The shape constraint loses its clause on the column.
const declarationAlwaysRevocableUpSQL = `
ALTER TABLE users.parent_announcements
	DROP CONSTRAINT IF EXISTS chk_parent_announcements_declaration_shape;
ALTER TABLE users.parent_announcements
	ADD CONSTRAINT chk_parent_announcements_declaration_shape CHECK (
		(delivery_mode = 'declaration') = (declaration_kind IS NOT NULL AND declaration_signers IS NOT NULL)
		AND (declaration_kind IS NULL OR declaration_kind = 'consent')
		AND (declaration_signers IS NULL OR declaration_signers IN ('any','all'))
		AND (NOT declaration_requires_password OR delivery_mode = 'declaration')
	);
ALTER TABLE users.parent_announcements DROP COLUMN IF EXISTS declaration_revocable;
`

// The down path restores the column as true for every declaration: since
// this migration every consent was revocable, whatever the old value said.
const declarationAlwaysRevocableDownSQL = `
ALTER TABLE users.parent_announcements
	ADD COLUMN IF NOT EXISTS declaration_revocable BOOLEAN NOT NULL DEFAULT false;
UPDATE users.parent_announcements SET declaration_revocable = COALESCE(declaration_kind = 'consent', false);
ALTER TABLE users.parent_announcements
	DROP CONSTRAINT IF EXISTS chk_parent_announcements_declaration_shape;
ALTER TABLE users.parent_announcements
	ADD CONSTRAINT chk_parent_announcements_declaration_shape CHECK (
		(delivery_mode = 'declaration') = (declaration_kind IS NOT NULL AND declaration_signers IS NOT NULL)
		AND (declaration_kind IS NULL OR declaration_kind = 'consent')
		AND (declaration_signers IS NULL OR declaration_signers IN ('any','all'))
		AND (NOT declaration_revocable OR declaration_kind = 'consent')
		AND (NOT declaration_requires_password OR delivery_mode = 'declaration')
	);
`

func declarationAlwaysRevocableUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, declarationAlwaysRevocableUpSQL); err != nil {
		return fmt.Errorf("drop users.parent_announcements.declaration_revocable: %w", err)
	}
	return nil
}

func declarationAlwaysRevocableDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, declarationAlwaysRevocableDownSQL); err != nil {
		return fmt.Errorf("restore users.parent_announcements.declaration_revocable: %w", err)
	}
	return nil
}
