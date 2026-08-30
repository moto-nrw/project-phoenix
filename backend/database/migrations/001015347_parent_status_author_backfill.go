package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
)

const (
	parentStatusAuthorBackfillVersion     = "1.15.347"
	parentStatusAuthorBackfillDescription = "Backfill the guardian author of parent-reported status days from approved absence requests (#2267)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentStatusAuthorBackfillVersion,
		Description: parentStatusAuthorBackfillDescription,
		DependsOn:   []string{parentStatusAuthorVersion, excusedAbsenceRequestsVersion},
	})
	Migrations.MustRegister(parentStatusAuthorBackfillUp, parentStatusAuthorBackfillDown)
}

// parentStatusAuthorBackfillUp stamps the submitting guardian on status days
// that an approved absence request wrote before 1.15.345 added the column.
//
// Only approved requests are evidence of authorship: a rejected or withdrawn
// request never wrote a day. Days a parent reported directly (sick note without
// the approval gate) carry no author source anywhere, so they stay NULL — the
// reader treats NULL as "unknown author" and hides the note rather than showing
// it to the wrong co-guardian.
func parentStatusAuthorBackfillUp(ctx context.Context, db *bun.DB) error {
	slog.Info("migration starting", slog.String("migration", parentStatusAuthorBackfillVersion))

	result, err := db.ExecContext(ctx, `
		WITH candidates AS (
			SELECT day.id AS day_id,
			       (
			           SELECT request.submitted_by
			           FROM active.excused_absence_requests AS request
			           WHERE request.tenant_id = day.tenant_id
			             AND request.student_id = day.student_id
			             AND request.status = 'approved'
			             AND request.absence_status = day.status
			             AND day.date::text IN (
			                 SELECT jsonb_array_elements_text(request.dates)
			             )
			           ORDER BY request.reviewed_at DESC NULLS LAST, request.id DESC
			           LIMIT 1
			       ) AS author_account_id
			FROM active.student_status_days AS day
			WHERE day.guardian_account_id IS NULL
			  AND day.source = 'parent'
		)
		UPDATE active.student_status_days AS day
		SET guardian_account_id = candidate.author_account_id
		FROM candidates AS candidate
		WHERE candidate.day_id = day.id
		  AND candidate.author_account_id IS NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("backfill parent status day authors: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count backfilled parent status day authors: %w", err)
	}
	slog.Info("migration finished",
		slog.String("migration", parentStatusAuthorBackfillVersion),
		slog.Int64("status_days_stamped", rows))
	return nil
}

// A backfilled author is indistinguishable from one the live parent flow wrote,
// so clearing the column on rollback would destroy real authorship. Rolling back
// 1.15.345 drops the column outright, which is the only safe reversal.
func parentStatusAuthorBackfillDown(_ context.Context, _ *bun.DB) error {
	slog.Info("migration rollback is a no-op",
		slog.String("migration", parentStatusAuthorBackfillVersion))
	return nil
}
