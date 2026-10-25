package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	testTraegerVersion     = "1.15.560"
	testTraegerDescription = "Mark test Träger on platform.organizations (#4658)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     testTraegerVersion,
		Description: testTraegerDescription,
		DependsOn:   []string{traegerErasureOrderVersion},
	})
	Migrations.MustRegister(testTraegerUp, testTraegerDown)
}

// testTraegerUp marks a Träger the moto team set up as a test system for a
// new Träger (#4658): test schools with invented sample data. The mark is set
// once at creation and never changes, so a real Träger can never become a
// test Träger and the reverse. A test Träger is erased like every other one,
// through the erasure order and `tenant erase --organization` (#4657).
func testTraegerUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE platform.organizations
			ADD COLUMN IF NOT EXISTS test_system BOOLEAN NOT NULL DEFAULT false;
		COMMENT ON COLUMN platform.organizations.test_system IS
			'#4658: test Träger with test schools and invented sample data; set at creation, never changed';
	`); err != nil {
		return fmt.Errorf("add platform.organizations test_system: %w", err)
	}
	return nil
}

func testTraegerDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE platform.organizations DROP COLUMN IF EXISTS test_system;
	`); err != nil {
		return fmt.Errorf("drop platform.organizations test_system: %w", err)
	}
	return nil
}
