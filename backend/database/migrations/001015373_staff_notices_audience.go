package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffNoticesAudienceVersion     = "1.15.373"
	staffNoticesAudienceDescription = "Tagesinformationen: Zielgruppe je Hinweis und Leserecht staff_notices:read für die Lehrkraft-Rolle (#2208)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffNoticesAudienceVersion,
		Description: staffNoticesAudienceDescription,
		DependsOn:   []string{staffNoticesVersion, lehrkraftRoleVersion},
	})

	Migrations.MustRegister(staffNoticesAudienceUp, staffNoticesAudienceDown)
}

// staffNoticesAudienceUp gibt jeder Tagesinformation eine Zielgruppe und der
// Lehrkraft-Rolle ein eigenes Leserecht (#2208).
//
// Zielgruppe: drei feste Werte, kein Freitext-Verteiler. 'all' erreicht die
// ganze Einrichtung, 'staff' nur die Betreuung im OGS-Portal, 'lehrkraft' nur
// die Lehrkräfte in moto schule. Bestehende Hinweise waren bisher schulweit,
// also bleibt 'all' der Standard und der Backfill ist der Spaltendefault.
//
// Leserecht: Lehrkräfte halten bewusst kein users:read (das Kinderverzeichnis
// bliebe sonst nicht geschlossen), deshalb hängt die Schul-Route der Hinweise
// an staff_notices:read. Admins bekommen das Recht auch, damit dieselbe
// Frage in beiden Portalen denselben Namen trägt; das OGS-Portal liest
// weiterhin über users:read, das jede Betreuungskraft hält.
func staffNoticesAudienceUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.373: Zielgruppe für Tagesinformationen und staff_notices:read...")

	if _, err := db.NewRaw(`
		ALTER TABLE users.staff_notices
		ADD COLUMN IF NOT EXISTS audience VARCHAR(20) NOT NULL DEFAULT 'all';

		ALTER TABLE users.staff_notices
		DROP CONSTRAINT IF EXISTS chk_staff_notices_audience;

		ALTER TABLE users.staff_notices
		ADD CONSTRAINT chk_staff_notices_audience
		CHECK (audience IN ('all', 'staff', 'lehrkraft'));
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding audience column to users.staff_notices: %w", err)
	}

	return grantPermissionToRoles(ctx, db, permissionSpec{
		Name:        "staff_notices:read",
		Description: "Tagesinformationen der Leitung lesen und zur Kenntnis nehmen",
		Resource:    "staff_notices",
		Action:      "read",
	}, "admin", "lehrkraft")
}

func staffNoticesAudienceDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.373...")

	if err := dropPermission(ctx, db, "staff_notices:read"); err != nil {
		return err
	}

	if _, err := db.NewRaw(`
		ALTER TABLE users.staff_notices
		DROP CONSTRAINT IF EXISTS chk_staff_notices_audience;

		ALTER TABLE users.staff_notices
		DROP COLUMN IF EXISTS audience;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping audience column from users.staff_notices: %w", err)
	}

	return nil
}
