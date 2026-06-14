package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	guardianFKRestrictVersion     = "1.15.129"
	guardianFKRestrictDescription = "Change students_guardians → guardian_profiles FK from ON DELETE CASCADE to RESTRICT to prevent unintended sibling unlinking on guardian deletion (#819)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     guardianFKRestrictVersion,
		Description: guardianFKRestrictDescription,
		DependsOn:   []string{"1.15.128", "1.15.2"}, // composite FKs created in 1.15.2
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return setGuardianLinkFKOnDelete(ctx, db, "RESTRICT")
		},
		func(ctx context.Context, db *bun.DB) error {
			return setGuardianLinkFKOnDelete(ctx, db, "CASCADE")
		},
	)
}

// setGuardianLinkFKOnDelete drops and recreates the composite FK
// fk_students_guardians_guardian_tenant with the given ON DELETE action.
//
// Why RESTRICT (up): under the original CASCADE, DELETE /guardians/{id} silently
// removed every students_guardians row for that guardian — including links to
// SIBLINGS the caller never intended to touch (#819). RESTRICT turns a blind
// guardian delete into a hard error when links still exist, which is what makes
// the handler's existing 409 ("Noch mit Kindern verknüpft") branch
// reachable. Deliberate full deletion removes the link rows first, then the
// guardian, inside one tenant transaction.
//
// Only this one inbound FK is changed. The guardian's OWN dependents
// (guardian_phone_numbers, guardian_invitations) keep CASCADE — deleting a
// guardian SHOULD clean those up; the danger was only the shared link table.
func setGuardianLinkFKOnDelete(ctx context.Context, db *bun.DB, onDelete string) error {
	fmt.Printf("Migration %s: setting fk_students_guardians_guardian_tenant ON DELETE %s...\n",
		guardianFKRestrictVersion, onDelete)

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil &&
			err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Drop the existing composite FK (created in 1.15.2). It is named, so a
	// direct DROP is safe; IF EXISTS keeps the migration idempotent.
	if _, err := tx.ExecContext(ctx,
		`ALTER TABLE users.students_guardians DROP CONSTRAINT IF EXISTS fk_students_guardians_guardian_tenant`,
	); err != nil {
		return fmt.Errorf("failed to drop fk_students_guardians_guardian_tenant: %w", err)
	}

	// Recreate it in the exact composite shape from 1.15.2, only swapping the
	// ON DELETE action.
	addSQL := fmt.Sprintf(
		`ALTER TABLE users.students_guardians
		 ADD CONSTRAINT fk_students_guardians_guardian_tenant
		 FOREIGN KEY (tenant_id, guardian_profile_id)
		 REFERENCES users.guardian_profiles(tenant_id, id)
		 ON DELETE %s`, onDelete)
	if _, err := tx.ExecContext(ctx, addSQL); err != nil {
		return fmt.Errorf("failed to recreate fk_students_guardians_guardian_tenant with ON DELETE %s: %w", onDelete, err)
	}

	return tx.Commit()
}
