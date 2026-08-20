package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	offeringAdjustmentSourceBackfillVersion     = "1.15.309"
	offeringAdjustmentSourceBackfillDescription = "Classify pre-1.15.308 offering adjustments so historical direct corrections reach the central history (#2413)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     offeringAdjustmentSourceBackfillVersion,
		Description: offeringAdjustmentSourceBackfillDescription,
		DependsOn:   []string{offeringAdjustmentSourceVersion},
	})
	Migrations.MustRegister(offeringAdjustmentSourceBackfillUp, offeringAdjustmentSourceBackfillDown)
}

// 1.15.308 introduced the source discriminator and left every pre-existing row
// as 'unknown', which the central history and the child's Änderungsprotokoll
// both skip. That hides exactly the corrections the Anfragen module was built
// to explain: the reference incident (OGS am Berg, 17.–19.08.2026) predates
// the column, so the school still cannot see what it changed back then.
//
// This backfill classifies those rows from evidence, never from a guess. Two
// writers produced request-applied rows before 1.15.308, and each left a
// machine-checkable trace:
//
//  1. Approving an Angebots-Anfrage wrote a fully generated reason
//     ("Elternanfrage #12 freigegeben (gültig ab 01.09.2026)"), optionally
//     followed by ": " and the reviewer's own words.
//  2. Approving an Anmeldungsänderung wrote the reviewer's decision note
//     verbatim — the same string the change request stores in
//     admin_decision_note, in the same transaction, by the same account, on
//     the same enrollment request.
//
// Everything the two rules do not claim is the office correcting a booking
// itself. Misjudging rule 2 would leave a real correction hidden, i.e. exactly
// today's behaviour, so the backfill can only improve on the status quo — but
// the rule is narrow enough (four columns must agree, including a
// character-identical free-text note) that an accidental match is not
// realistic.
func offeringAdjustmentSourceBackfillUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.309: Classifying historical offering adjustments...")

	// Rule 1: the reason is machine-generated, so the shape identifies it.
	generated, err := db.ExecContext(ctx, `
		UPDATE audit.enrollment_offering_adjustments
		   SET source = 'request'
		 WHERE source = 'unknown'
		   AND reason ~ '^Elternanfrage #[0-9]+ freigegeben \(gültig ab [0-9]{2}\.[0-9]{2}\.[0-9]{4}\)';
	`)
	if err != nil {
		return fmt.Errorf("label generated offering-request adjustments: %w", err)
	}

	// Rule 2: the adjustment and the approved Anmeldungsänderung were written
	// by one transaction. changed_at defaults to the transaction start and
	// reviewed_at is stamped inside it, so a minute of slack is already far
	// more than the real gap.
	correlated, err := db.ExecContext(ctx, `
		UPDATE audit.enrollment_offering_adjustments a
		   SET source = 'request'
		 WHERE a.source = 'unknown'
		   AND EXISTS (
		       SELECT 1
		         FROM enrollment.change_requests cr
		        WHERE cr.tenant_id = a.tenant_id
		          AND cr.request_id = a.request_id
		          AND cr.status = 'approved'
		          AND cr.reviewed_at IS NOT NULL
		          AND cr.reviewed_by_account_id = a.actor_account_id
		          AND cr.admin_decision_note = a.reason
		          AND a.changed_at BETWEEN cr.reviewed_at - INTERVAL '1 minute'
		                               AND cr.reviewed_at + INTERVAL '1 minute'
		   );
	`)
	if err != nil {
		return fmt.Errorf("label request-applied offering adjustments: %w", err)
	}

	// Everything left is the office's own correction.
	direct, err := db.ExecContext(ctx, `
		UPDATE audit.enrollment_offering_adjustments
		   SET source = 'direct'
		 WHERE source = 'unknown';
	`)
	if err != nil {
		return fmt.Errorf("label direct offering adjustments: %w", err)
	}

	generatedRows, _ := generated.RowsAffected()
	correlatedRows, _ := correlated.RowsAffected()
	directRows, _ := direct.RowsAffected()
	fmt.Printf("Migration 1.15.309: %d generated + %d correlated request row(s), %d direct correction(s)\n",
		generatedRows, correlatedRows, directRows)
	return nil
}

// The classification cannot be un-derived: after this migration a 'direct' row
// may be a backfilled legacy row or one the running code wrote, and nothing
// tells them apart. Resetting them all to 'unknown' would blank out live data,
// so the rollback deliberately does nothing. Rolling back past 1.15.308 drops
// the column entirely, which covers the real "undo" case.
func offeringAdjustmentSourceBackfillDown(_ context.Context, _ *bun.DB) error {
	fmt.Println("Rolling back 1.15.309: no-op — the source classification is not reversible (1.15.308 drops the column).")
	return nil
}
