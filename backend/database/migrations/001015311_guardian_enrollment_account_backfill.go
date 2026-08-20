package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	guardianEnrollmentAccountBackfillVersion     = "1.15.311"
	guardianEnrollmentAccountBackfillDescription = "Link guardian-less enrollment requests to existing parent accounts (#2422)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     guardianEnrollmentAccountBackfillVersion,
		Description: guardianEnrollmentAccountBackfillDescription,
		DependsOn:   []string{absenceRequestStatusVersion},
	})
	Migrations.MustRegister(guardianEnrollmentAccountBackfillUp, guardianEnrollmentAccountBackfillDown)
}

func guardianEnrollmentAccountBackfillUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.311: Linking guardian-less enrollment requests to parent accounts...")

	result, err := db.ExecContext(ctx, `
		WITH unique_accounts AS (
			SELECT LOWER(TRIM(email)) AS normalized_email,
			       MIN(id) AS account_id
			FROM auth.accounts
			GROUP BY LOWER(TRIM(email))
			HAVING COUNT(*) = 1
		)
		UPDATE enrollment.requests AS request
		SET guardian_account_id = unique_account.account_id
		FROM unique_accounts AS unique_account
		WHERE request.guardian_account_id IS NULL
		  AND LOWER(TRIM(request.guardian_email)) = unique_account.normalized_email;
	`)
	if err != nil {
		return fmt.Errorf("backfill enrollment guardian account ids: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count backfilled enrollment guardian account ids: %w", err)
	}
	fmt.Printf("Migration 1.15.311: Linked %d enrollment request(s) to parent accounts.\n", rows)
	return nil
}

// The updated rows cannot be distinguished from links written by the live
// invitation-accept flow, so clearing them on rollback would destroy valid
// ownership data.
func guardianEnrollmentAccountBackfillDown(_ context.Context, _ *bun.DB) error {
	fmt.Println("Rolling back 1.15.311: no-op — enrollment ownership links are not safely reversible.")
	return nil
}
