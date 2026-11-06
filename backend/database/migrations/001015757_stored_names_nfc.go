package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const storedNamesNFCVersion = "1.15.757"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     storedNamesNFCVersion,
		Description: "Gespeicherte Namen in Unicode NFC bringen, damit Suche und Duplikatprüfung sie finden (#5791)",
		DependsOn: []string{
			unaccentNameSearchVersion, // the last migration before this one
		},
	})
	Migrations.MustRegister(storedNamesNFCUp, storedNamesNFCDown)
}

// storedNamesNFCUpSQL composes the names saved before #4949 normalized them
// on write. Name searches, the enrollment duplicate check and the contact
// match normalize only their input and compare it with the stored column, so
// a name stored decomposed (NFD, as macOS pastes it) would never match. The
// WHERE clauses touch only rows that are not NFC already; production had none
// when #4949 measured it, so on most databases this changes nothing.
const storedNamesNFCUpSQL = `
UPDATE users.persons
   SET first_name = NORMALIZE(first_name, NFC), last_name = NORMALIZE(last_name, NFC)
 WHERE first_name IS NOT NFC NORMALIZED OR last_name IS NOT NFC NORMALIZED;
UPDATE users.guardian_profiles
   SET first_name = NORMALIZE(first_name, NFC), last_name = NORMALIZE(last_name, NFC)
 WHERE first_name IS NOT NFC NORMALIZED OR last_name IS NOT NFC NORMALIZED;
UPDATE enrollment.request_children
   SET first_name = NORMALIZE(first_name, NFC), last_name = NORMALIZE(last_name, NFC)
 WHERE first_name IS NOT NFC NORMALIZED OR last_name IS NOT NFC NORMALIZED;
`

func storedNamesNFCUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, storedNamesNFCUpSQL); err != nil {
		return fmt.Errorf("normalize stored names to NFC: %w", err)
	}
	return nil
}

// storedNamesNFCDown keeps the composed names: NFC and NFD spell the same
// text, and the decomposed form is not recorded anywhere to restore it.
func storedNamesNFCDown(context.Context, *bun.DB) error {
	return nil
}
