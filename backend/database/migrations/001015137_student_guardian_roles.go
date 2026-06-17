package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	studentGuardianRolesVersion     = "1.15.137"
	studentGuardianRolesDescription = "Add role presets and parent portal permissions to student guardians"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentGuardianRolesVersion,
		Description: studentGuardianRolesDescription,
		DependsOn:   []string{parentAuthoredExceptionsVersion, guardianInvitationProfileOriginVersion, passkeyCredentialsVersion, guardianFKRestrictVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return studentGuardianRolesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return studentGuardianRolesDown(ctx, db)
		},
	)
}

func studentGuardianRolesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.137: Adding guardian role presets to users.students_guardians...")

	if _, err := db.NewRaw(`
		ALTER TABLE users.students_guardians
		ADD COLUMN IF NOT EXISTS guardian_role TEXT NOT NULL DEFAULT 'custom';
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding guardian_role to users.students_guardians: %w", err)
	}

	if _, err := db.NewRaw(`
		UPDATE users.students_guardians
		SET guardian_role = CASE
			WHEN is_primary = TRUE THEN 'primary_guardian'
			WHEN relationship_type IN ('parent', 'guardian') THEN 'legal_guardian'
			WHEN can_pickup = TRUE AND relationship_type IN ('relative', 'other') THEN 'pickup_only'
			WHEN is_emergency_contact = TRUE AND relationship_type IN ('relative', 'other') THEN 'emergency_contact'
			ELSE 'custom'
		END
		WHERE guardian_role IS NULL OR guardian_role = 'custom';
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed backfilling guardian_role on users.students_guardians: %w", err)
	}

	if _, err := db.NewRaw(`
		UPDATE users.students_guardians
		SET permissions = COALESCE(permissions, '{}'::jsonb) || jsonb_build_object(
			'parent_portal.access', true,
			'parent_portal.sick_note.submit', true,
			'parent_portal.notes.write', true,
			'parent_portal.enrollments.view', true,
			'parent_portal.enrollment.submit', true
		)
		WHERE guardian_role IN ('primary_guardian', 'legal_guardian', 'co_guardian');
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed backfilling guardian portal permissions: %w", err)
	}

	return nil
}

func studentGuardianRolesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.137: Removing guardian role presets from users.students_guardians...")
	if _, err := db.NewRaw(`
		ALTER TABLE users.students_guardians
		DROP COLUMN IF EXISTS guardian_role;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping guardian_role from users.students_guardians: %w", err)
	}
	return nil
}
