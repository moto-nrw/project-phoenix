package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	guardianInvitationsParentInitiatedVersion     = "1.15.134"
	guardianInvitationsParentInitiatedDescription = "Add parent-initiated invite + staff-approval columns to auth.guardian_invitations"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     guardianInvitationsParentInitiatedVersion,
		Description: guardianInvitationsParentInitiatedDescription,
		DependsOn: []string{
			AuthGuardianInvitationsVersion, // 1.6.16 — base table
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return guardianInvitationsParentInitiatedUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return guardianInvitationsParentInitiatedDown(ctx, db)
		},
	)
}

func guardianInvitationsParentInitiatedUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.134: Extending auth.guardian_invitations for parent-initiated invites...")

	// New columns:
	//   student_id              — which child the invite grants access to. Lets
	//                             the accept flow link an existing account to an
	//                             additional child (sibling case).
	//   requested_by_account_id — set when a parent (not staff) initiated the
	//                             invite; NULL for staff-initiated invites.
	//   approval_status         — 'not_required' (staff invites + parent direct
	//                             mode) | 'pending' | 'approved' | 'rejected'.
	//   approved_by / approved_at — staff member who resolved a pending request.
	if _, err := db.NewRaw(`
		ALTER TABLE auth.guardian_invitations
			ADD COLUMN IF NOT EXISTS student_id BIGINT,
			ADD COLUMN IF NOT EXISTS requested_by_account_id BIGINT,
			ADD COLUMN IF NOT EXISTS approval_status TEXT NOT NULL DEFAULT 'not_required',
			ADD COLUMN IF NOT EXISTS approved_by BIGINT,
			ADD COLUMN IF NOT EXISTS approved_at TIMESTAMPTZ;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("error adding parent-initiated columns: %w", err)
	}

	// student_id uses a tenant-safe COMPOSITE FK (tenant_id, student_id) →
	// users.students(tenant_id, id) so a write bug can never pair tenant A's
	// invitation with tenant B's student — the DB rejects the mismatch. NULL
	// student_id (staff invites with no specific child) is unenforced under
	// MATCH SIMPLE, which is the intended behaviour.
	if _, err := db.NewRaw(`
		ALTER TABLE auth.guardian_invitations
			ADD CONSTRAINT fk_guardian_invitation_student
				FOREIGN KEY (tenant_id, student_id) REFERENCES users.students(tenant_id, id) ON DELETE CASCADE,
			ADD CONSTRAINT fk_guardian_invitation_requested_by
				FOREIGN KEY (requested_by_account_id) REFERENCES auth.accounts(id) ON DELETE SET NULL,
			ADD CONSTRAINT fk_guardian_invitation_approved_by
				FOREIGN KEY (approved_by) REFERENCES auth.accounts(id) ON DELETE SET NULL,
			ADD CONSTRAINT chk_guardian_invitation_approval_status
				CHECK (approval_status IN ('not_required', 'pending', 'approved', 'rejected'));
	`).Exec(ctx); err != nil {
		return fmt.Errorf("error adding parent-initiated constraints: %w", err)
	}

	// Partial index to serve the staff approval queue efficiently.
	if _, err := db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_guardian_invitations_pending_approval
			ON auth.guardian_invitations(approval_status)
			WHERE approval_status = 'pending';
		CREATE INDEX IF NOT EXISTS idx_guardian_invitations_student_id
			ON auth.guardian_invitations(student_id);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("error creating parent-initiated indexes: %w", err)
	}

	return nil
}

func guardianInvitationsParentInitiatedDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.134: dropping parent-initiated columns...")

	if _, err := db.NewRaw(`
		DROP INDEX IF EXISTS auth.idx_guardian_invitations_pending_approval;
		DROP INDEX IF EXISTS auth.idx_guardian_invitations_student_id;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("error dropping parent-initiated indexes: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE auth.guardian_invitations
			DROP CONSTRAINT IF EXISTS fk_guardian_invitation_student,
			DROP CONSTRAINT IF EXISTS fk_guardian_invitation_requested_by,
			DROP CONSTRAINT IF EXISTS fk_guardian_invitation_approved_by,
			DROP CONSTRAINT IF EXISTS chk_guardian_invitation_approval_status,
			DROP COLUMN IF EXISTS student_id,
			DROP COLUMN IF EXISTS requested_by_account_id,
			DROP COLUMN IF EXISTS approval_status,
			DROP COLUMN IF EXISTS approved_by,
			DROP COLUMN IF EXISTS approved_at;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("error dropping parent-initiated columns: %w", err)
	}

	return nil
}
