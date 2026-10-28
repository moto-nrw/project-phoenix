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
// (#5140). The column holds the school classes the decision covers; NULL
// means no decision. A child of another class without a Regeltermin brings
// the hint back. The value names classes, no person, and goes with its
// offering; a copied offering of the next school year starts without it.
const careOfferingTemplateGapAckUpSQL = `
		ALTER TABLE enrollment.care_offerings
			ADD COLUMN IF NOT EXISTS template_gap_ack_classes TEXT[];

		COMMENT ON COLUMN enrollment.care_offerings.template_gap_ack_classes IS
			'School classes whose booked children need no Regeltermin of this offering, as decided by the school; NULL = no decision, '''' = children without a class (#5140).';
`

func careOfferingTemplateGapAckUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, careOfferingTemplateGapAckUpSQL); err != nil {
		return fmt.Errorf("add enrollment.care_offerings.template_gap_ack_classes: %w", err)
	}
	return nil
}

func careOfferingTemplateGapAckDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `ALTER TABLE enrollment.care_offerings DROP COLUMN IF EXISTS template_gap_ack_classes;`); err != nil {
		return fmt.Errorf("drop enrollment.care_offerings.template_gap_ack_classes: %w", err)
	}
	return nil
}
