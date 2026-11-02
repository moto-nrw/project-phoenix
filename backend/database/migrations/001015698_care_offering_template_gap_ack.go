package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	careOfferingTemplateGapAckVersion     = "1.15.698"
	careOfferingTemplateGapAckDescription = "Angebot ohne passenden Regeltermin: Entscheidung der Schule „Kein eigener Regeltermin nötig“ am Angebot (#5140)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     careOfferingTemplateGapAckVersion,
		Description: careOfferingTemplateGapAckDescription,
		DependsOn: []string{
			manualRosterConfirmedVersion, // the last migration before this one
		},
	})
	Migrations.MustRegister(careOfferingTemplateGapAckUp, careOfferingTemplateGapAckDown)
}

// careOfferingTemplateGapAckUpSQL records the school's decision that the
// booked children of a care offering need no Regeltermin of their own
// (#5140). The column maps a planning period id to the school classes the
// decision covers in that period; NULL means no decision anywhere. A child of
// another class without a Regeltermin brings the hint back, and a decision
// for one period never hides the hint of another. The value names periods and
// classes, no person, and goes with its offering; a copied offering of the
// next school year starts without it.
const careOfferingTemplateGapAckUpSQL = `
		ALTER TABLE enrollment.care_offerings
			ADD COLUMN IF NOT EXISTS template_gap_acks JSONB;

		COMMENT ON COLUMN enrollment.care_offerings.template_gap_acks IS
			'Planning period id -> school classes whose booked children need no Regeltermin of this offering in that period, as decided by the school; NULL = no decision (#5140).';
`

func careOfferingTemplateGapAckUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, careOfferingTemplateGapAckUpSQL); err != nil {
		return fmt.Errorf("add enrollment.care_offerings.template_gap_acks: %w", err)
	}
	return nil
}

func careOfferingTemplateGapAckDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `ALTER TABLE enrollment.care_offerings DROP COLUMN IF EXISTS template_gap_acks;`); err != nil {
		return fmt.Errorf("drop enrollment.care_offerings.template_gap_acks: %w", err)
	}
	return nil
}
