package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	classParentLettersVersion     = "1.15.680"
	classParentLettersDescription = "Elternbriefe der Lehrkraft an ihre Klasse aus moto schule (#4800)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     classParentLettersVersion,
		Description: classParentLettersDescription,
		DependsOn: []string{
			holidayRegistrationDeletionsVersion, // the last migration before this one
			parentAnnouncementsVersion,          // the table a class letter is a row of
			lehrkraftRoleVersion,                // the role the write permission goes to
			schulverwaltungRoleVersion,          // holds every Lehrkraft permission (#4789)
		},
	})
	Migrations.MustRegister(classParentLettersUp, classParentLettersDown)
}

// classParentLettersUpSQL marks a parent announcement a Lehrkraft wrote in
// moto schule (#4800). The letter stays an ordinary announcement with one
// class target, so audience, receipts, e-mail and retention are the OGS's.
//
//   - sender_school_class is the class the letter goes to; a row with it is
//     a class letter, which the OGS reads but does not change.
//   - sender_name is the name parents read as the sender, as it stood when
//     the Lehrkraft wrote: a letter keeps its letterhead.
//
// Both are set together or not at all.
const classParentLettersUpSQL = `
		ALTER TABLE users.parent_announcements
			ADD COLUMN IF NOT EXISTS sender_name TEXT,
			ADD COLUMN IF NOT EXISTS sender_school_class TEXT;

		COMMENT ON COLUMN users.parent_announcements.sender_name IS
			'Name der Lehrkraft, die den Klassenbrief in moto schule geschrieben hat (#4800); NULL bei Mitteilungen der OGS.';
		COMMENT ON COLUMN users.parent_announcements.sender_school_class IS
			'Klasse, an die die Lehrkraft den Klassenbrief geschrieben hat (#4800); NULL bei Mitteilungen der OGS.';

		ALTER TABLE users.parent_announcements
			DROP CONSTRAINT IF EXISTS chk_parent_announcements_class_letter;
		ALTER TABLE users.parent_announcements
			ADD CONSTRAINT chk_parent_announcements_class_letter CHECK (
				(sender_name IS NULL AND sender_school_class IS NULL)
				OR (sender_name IS NOT NULL AND sender_school_class IS NOT NULL
					AND btrim(sender_name) <> '' AND btrim(sender_school_class) <> '')
			);
`

const classParentLettersDownSQL = `
		ALTER TABLE users.parent_announcements
			DROP CONSTRAINT IF EXISTS chk_parent_announcements_class_letter;
		ALTER TABLE users.parent_announcements
			DROP COLUMN IF EXISTS sender_school_class,
			DROP COLUMN IF EXISTS sender_name;
`

// classParentLettersUp adds the sender columns and grants
// class_day:parent_letter_write to the system roles lehrkraft and
// schulverwaltung. The route re-checks the class reach per letter: the
// Lehrkraft writes to her classes from education.class_teachers, the
// Schulverwaltung (class_day:all_classes) to every class. A school's own
// role of the same name gets nothing.
func classParentLettersUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, classParentLettersUpSQL); err != nil {
			return fmt.Errorf("add the class-letter sender to users.parent_announcements: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO auth.permissions (name, description, resource, action)
			VALUES ('class_day:parent_letter_write', 'Elternbrief an eine zugewiesene Klasse schreiben (moto schule)', 'class_day', 'parent_letter_write')
			ON CONFLICT (name) DO NOTHING`); err != nil {
			return fmt.Errorf("insert class_day:parent_letter_write: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO auth.role_permissions (role_id, permission_id)
			SELECT r.id, p.id
			FROM auth.roles r
			JOIN auth.permissions p ON p.name = 'class_day:parent_letter_write'
			WHERE r.name IN ('lehrkraft', 'schulverwaltung') AND r.tenant_id IS NULL
			ON CONFLICT (role_id, permission_id) DO NOTHING`); err != nil {
			return fmt.Errorf("grant class_day:parent_letter_write to the school-portal system roles: %w", err)
		}
		return nil
	})
}

// classParentLettersDown refuses while class letters exist: dropping the
// sender would turn them into letters of the OGS.
func classParentLettersDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		var stored int
		if err := tx.NewRaw(`SELECT COUNT(*) FROM users.parent_announcements WHERE sender_school_class IS NOT NULL`).Scan(ctx, &stored); err != nil {
			return fmt.Errorf("count class letters: %w", err)
		}
		if stored > 0 {
			return fmt.Errorf("users.parent_announcements holds %d class letters; remove them before rolling back %s", stored, classParentLettersVersion)
		}
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM auth.role_permissions
			WHERE permission_id IN (SELECT id FROM auth.permissions WHERE name = 'class_day:parent_letter_write')`); err != nil {
			return fmt.Errorf("revoke class_day:parent_letter_write: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM auth.permissions WHERE name = 'class_day:parent_letter_write'`); err != nil {
			return fmt.Errorf("delete class_day:parent_letter_write: %w", err)
		}
		if _, err := tx.ExecContext(ctx, classParentLettersDownSQL); err != nil {
			return fmt.Errorf("drop the class-letter sender: %w", err)
		}
		return nil
	})
}
