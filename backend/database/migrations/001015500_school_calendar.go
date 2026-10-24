package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	schoolCalendarVersion     = "1.15.500"
	schoolCalendarDescription = "Gemeinsamer Kalender: Schultermine, Ziel Lehrkräfte und calendar:school für moto schule (#4798)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     schoolCalendarVersion,
		Description: schoolCalendarDescription,
		DependsOn:   []string{calendarTargetAllSchoolParentsVersion, schoolPortalRolesVersion},
	})

	Migrations.MustRegister(schoolCalendarUp, schoolCalendarDown)
}

// schoolCalendarUp bringt den Kalender nach moto schule (#4798).
//
//   - calendar.appointments.school_event_type markiert einen Schultermin, den
//     eine Lehrkraft oder die Schulverwaltung eingetragen hat. NULL ist ein
//     Termin der OGS. Die Art ist ein fester Wert, kein Freitext.
//   - Das Ziel all_teachers erreicht die Lehrkräfte in moto schule. Die OGS
//     wählt es, wenn Lehrkräfte einen Termin sehen sollen.
//   - calendar:school öffnet den Kalender in moto schule: Termine sehen,
//     eigene Schultermine eintragen, den Abo-Link holen. Die Lehrkraft-Rolle
//     und die Schulrollen aus ihrer Vorlage (#4574) bekommen das Recht, Admins
//     auch, damit dieselbe Frage in beiden Portalen denselben Namen trägt.
func schoolCalendarUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		ALTER TABLE calendar.appointments
			ADD COLUMN IF NOT EXISTS school_event_type VARCHAR(30);

		ALTER TABLE calendar.appointments
			DROP CONSTRAINT IF EXISTS chk_calendar_appointments_school_event_type,
			ADD CONSTRAINT chk_calendar_appointments_school_event_type CHECK (
				school_event_type IS NULL OR school_event_type IN (
					'ausflug',
					'paedagogischer_tag',
					'unterrichtsende',
					'sonstiges'
				)
			);

		ALTER TABLE calendar.appointment_targets
			DROP CONSTRAINT IF EXISTS chk_calendar_targets_type,
			ADD CONSTRAINT chk_calendar_targets_type CHECK (
				target_type IN (
					'staff',
					'guardian_profile',
					'all_staff',
					'all_teachers',
					'all_school_parents',
					'parents_by_class',
					'parents_by_group',
					'parents_by_student'
				)
			);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding school calendar columns: %w", err)
	}

	if err := grantPermissionToRoles(ctx, db, permissionSpec{
		Name:        "calendar:school",
		Description: "Kalender in moto schule nutzen und eigene Schultermine eintragen",
		Resource:    "calendar",
		Action:      "school",
	}, "admin", "lehrkraft"); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `
		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT r.id, p.id
		FROM auth.roles r
		CROSS JOIN auth.permissions p
		WHERE p.name = 'calendar:school'
		  AND r.school_portal
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`); err != nil {
		return fmt.Errorf("failed granting calendar:school to school-portal roles: %w", err)
	}
	return nil
}

func schoolCalendarDown(ctx context.Context, db *bun.DB) error {
	if err := dropPermission(ctx, db, "calendar:school"); err != nil {
		return err
	}

	// Ohne die Spalte und das Ziel haben Schultermine und Ziele für Lehrkräfte
	// keine gültige Form mehr; sie werden vor dem engeren CHECK entfernt.
	if _, err := db.NewRaw(`
		DELETE FROM calendar.appointment_targets
		WHERE target_type = 'all_teachers';

		ALTER TABLE calendar.appointment_targets
			DROP CONSTRAINT IF EXISTS chk_calendar_targets_type,
			ADD CONSTRAINT chk_calendar_targets_type CHECK (
				target_type IN (
					'staff',
					'guardian_profile',
					'all_staff',
					'all_school_parents',
					'parents_by_class',
					'parents_by_group',
					'parents_by_student'
				)
			);

		ALTER TABLE calendar.appointments
			DROP CONSTRAINT IF EXISTS chk_calendar_appointments_school_event_type,
			DROP COLUMN IF EXISTS school_event_type;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping school calendar columns: %w", err)
	}
	return nil
}
