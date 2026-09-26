package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const demoVisitorFirstLastNameVersion = "1.15.425"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     demoVisitorFirstLastNameVersion,
		Description: "Store the demo visitor's first and last name instead of one person name",
		DependsOn:   []string{"1.15.412", staffQualificationOrderVersion},
	})
	Migrations.MustRegister(demoVisitorFirstLastNameUp, demoVisitorFirstLastNameDown)
}

// The public demo asks for first and last name separately; the visitor's
// caregiver and parent carry exactly these names. The deploy stops the
// application before it migrates and restores the database on a rollback,
// so no running image still reads person_name when it is dropped.
//
// Stored rows keep the name the seeder showed: runs of whitespace become one
// space, the first word is the first name and the rest the last name. A
// single word got the family name of the visitor's child from the seeder,
// Schneider, and keeps it, so a restart of such a demo school shows the name
// the visitor already knows. The standing school's state has no visitor; its
// empty name stays empty.
func demoVisitorFirstLastNameUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		ALTER TABLE auth.demo_accesses
			ADD COLUMN first_name TEXT NOT NULL DEFAULT '',
			ADD COLUMN last_name TEXT NOT NULL DEFAULT '';
		ALTER TABLE platform.demo_school_states
			ADD COLUMN first_name TEXT NOT NULL DEFAULT '',
			ADD COLUMN last_name TEXT NOT NULL DEFAULT '';

		UPDATE auth.demo_accesses AS target SET first_name = split_part(parts.collapsed, ' ', 1), last_name = CASE
				WHEN parts.collapsed = '' THEN ''
				WHEN strpos(parts.collapsed, ' ') = 0 THEN 'Schneider'
				ELSE substr(parts.collapsed, strpos(parts.collapsed, ' ') + 1)
			END
		FROM (SELECT id, btrim(regexp_replace(person_name, '\s+', ' ', 'g')) AS collapsed FROM auth.demo_accesses) AS parts
		WHERE parts.id = target.id;
		UPDATE platform.demo_school_states AS target SET first_name = split_part(parts.collapsed, ' ', 1), last_name = CASE
				WHEN parts.collapsed = '' THEN ''
				WHEN strpos(parts.collapsed, ' ') = 0 THEN 'Schneider'
				ELSE substr(parts.collapsed, strpos(parts.collapsed, ' ') + 1)
			END
		FROM (SELECT name, btrim(regexp_replace(person_name, '\s+', ' ', 'g')) AS collapsed FROM platform.demo_school_states) AS parts
		WHERE parts.name = target.name;

		-- Every access names its visitor, as person_name did.
		ALTER TABLE auth.demo_accesses
			ALTER COLUMN first_name DROP DEFAULT,
			ALTER COLUMN last_name DROP DEFAULT,
			DROP COLUMN person_name;
		-- The serving role queues an order with the visitor's name, as it did
		-- with person_name, whose column grant goes with the column.
		ALTER TABLE platform.demo_school_states DROP COLUMN person_name;
		GRANT INSERT (first_name, last_name) ON platform.demo_school_states TO phoenix_admin;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("split demo visitor names: %w", err)
	}
	return nil
}

func demoVisitorFirstLastNameDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		ALTER TABLE auth.demo_accesses ADD COLUMN IF NOT EXISTS person_name TEXT NOT NULL DEFAULT '';
		UPDATE auth.demo_accesses SET person_name = btrim(first_name || ' ' || last_name);
		ALTER TABLE auth.demo_accesses ALTER COLUMN person_name DROP DEFAULT;
		ALTER TABLE platform.demo_school_states ADD COLUMN IF NOT EXISTS person_name TEXT NOT NULL DEFAULT '';
		UPDATE platform.demo_school_states SET person_name = btrim(first_name || ' ' || last_name);
		REVOKE INSERT (first_name, last_name) ON platform.demo_school_states FROM phoenix_admin;
		GRANT INSERT (person_name) ON platform.demo_school_states TO phoenix_admin;
		ALTER TABLE auth.demo_accesses DROP COLUMN IF EXISTS first_name, DROP COLUMN IF EXISTS last_name;
		ALTER TABLE platform.demo_school_states DROP COLUMN IF EXISTS first_name, DROP COLUMN IF EXISTS last_name;
	`).Exec(ctx)
	return err
}
