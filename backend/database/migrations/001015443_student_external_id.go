package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	studentExternalIDVersion     = "1.15.443"
	studentExternalIDDescription = "External key of a child from another system, such as IServ, on users.student_profiles (#1446)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentExternalIDVersion,
		Description: studentExternalIDDescription,
		DependsOn:   []string{requestChildClassSwitchVersion}, // preserves ladder order
	})

	Migrations.MustRegister(studentExternalIDUp, studentExternalIDDown)
}

// studentExternalIDUp stores the key another system knows a child by (#1446).
// The IServ user import writes "iserv:<Interne ID>" (or the account when that
// column is hidden), so a later import of the same export finds the child
// again after a class change, even without a birthday. One key names at most
// one child of a school.
func studentExternalIDUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			ALTER TABLE users.student_profiles
				ADD COLUMN IF NOT EXISTS external_id TEXT;
			CREATE UNIQUE INDEX IF NOT EXISTS uq_student_profiles_external_id
				ON users.student_profiles (tenant_id, external_id)
				WHERE external_id IS NOT NULL;
		`); err != nil {
			return fmt.Errorf("error adding external_id to users.student_profiles: %w", err)
		}
		return nil
	})
}

// studentExternalIDDown drops the keys. Later imports fall back to matching
// by name, class and birthday.
func studentExternalIDDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			DROP INDEX IF EXISTS users.uq_student_profiles_external_id;
			ALTER TABLE users.student_profiles DROP COLUMN IF EXISTS external_id;
		`); err != nil {
			return fmt.Errorf("error dropping external_id from users.student_profiles: %w", err)
		}
		return nil
	})
}
