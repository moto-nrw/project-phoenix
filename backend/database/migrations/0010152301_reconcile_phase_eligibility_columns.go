package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	phaseEligibilityReconciliationVersion     = "1.15.230.1"
	phaseEligibilityReconciliationDescription = "Reconcile enrollment.phases eligibility columns when the former push migration used 001015230"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     phaseEligibilityReconciliationVersion,
		Description: phaseEligibilityReconciliationDescription,
		DependsOn:   []string{phaseEligibilityColumnsVersion},
	})

	Migrations.MustRegister(
		phaseEligibilityReconciliationUp,
		phaseEligibilityReconciliationDown,
	)
}

// phaseEligibilityReconciliationUp repairs databases where the former push
// migration occupied Bun name 001015230. Bun then considered the published
// phase migration applied although its columns were never created. This name
// sorts immediately after 001015230 and before 001015231, so the latter can
// safely install its audience constraint afterwards.
func phaseEligibilityReconciliationUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.230.1: Reconciling enrollment.phases eligibility columns...")

	_, err := db.NewRaw(`
		ALTER TABLE enrollment.phases
		ADD COLUMN IF NOT EXISTS audience TEXT NOT NULL DEFAULT 'open',
		ADD COLUMN IF NOT EXISTS eligible_school_classes JSONB NOT NULL DEFAULT '[]'::jsonb;

		COMMENT ON COLUMN enrollment.phases.audience IS 'Who may apply to this phase: open (everyone incl. anonymous), new_students (anonymous allowed, children already enrolled at the school are rejected), linked_parents (only parent accounts with an active guardian link at the school; hidden from the public listing)';
		COMMENT ON COLUMN enrollment.phases.eligible_school_classes IS 'When non-empty, every child in a submission must declare a school class contained in this list (e.g. ["1a","2b"]); empty means no class restriction';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed reconciling enrollment phase eligibility columns: %w", err)
	}
	return nil
}

// This is intentionally forward-only. The columns may be owned by the
// published 1.15.230 migration, whose ledger entry must remain authoritative.
func phaseEligibilityReconciliationDown(context.Context, *bun.DB) error {
	return nil
}
