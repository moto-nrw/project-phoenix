package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const demoSchoolFileErasuresVersion = "1.15.735"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     demoSchoolFileErasuresVersion,
		Description: "Demo-Betrieb: Vermerk, dass die Dateien einer ausgemusterten Demo-Schule gelöscht sind (#5544)",
		DependsOn: []string{
			payrollWageTypesVersion, // the last migration before this one
			"1.15.403",              // the demo role this grants to
		},
	})
	Migrations.MustRegister(demoSchoolFileErasuresUp, demoSchoolFileErasuresDown)
}

// demoSchoolFileErasuresUpSQL records, per retired demo school, that the
// demo process removed every upload namespace of the school and counted none
// left (#5544). The row is the work marker: a retired school without one is
// still to be done, so a failed run leaves no row and the next run retries.
// file_counts holds the number of objects removed per namespace, never a
// file name. Only the demo process reads and writes the table; it never
// updates or deletes a row.
//
// school_id has no foreign key on purpose: the school erasure fingerprints
// every incoming foreign key of platform.schools and refuses to run when one
// is added without a reviewed plan (school_erasure_preflight). The marker
// holds no personal data; a marker whose school is gone names nobody.
const demoSchoolFileErasuresUpSQL = `
	CREATE TABLE platform.demo_school_file_erasures (
		school_id BIGINT PRIMARY KEY,
		erased_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
		file_counts JSONB NOT NULL CHECK (jsonb_typeof(file_counts) = 'object')
	);
	COMMENT ON TABLE platform.demo_school_file_erasures IS
		'Demo process only (#5544): the uploads of this retired demo school are removed and verified empty. Counts per namespace, no file names.';
	REVOKE ALL ON platform.demo_school_file_erasures FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_admin;
	GRANT SELECT, INSERT ON platform.demo_school_file_erasures TO phoenix_demo;
`

func demoSchoolFileErasuresUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, demoSchoolFileErasuresUpSQL); err != nil {
		return fmt.Errorf("create the file erasure marker of retired demo schools: %w", err)
	}
	return nil
}

func demoSchoolFileErasuresDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS platform.demo_school_file_erasures`); err != nil {
		return fmt.Errorf("drop the file erasure marker of retired demo schools: %w", err)
	}
	return nil
}
