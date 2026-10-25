package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// restoreSchoolErasureBeforeSurveys reinstalls the frozen pre-1.15.553
// definitions, including the previous plan and FK fingerprint, in the same
// transaction that drops the survey tables.
func restoreSchoolErasureBeforeSurveys(ctx context.Context, tx bun.Tx) error {
	for _, step := range []struct {
		name string
		sql  string
	}{
		{"plan", schoolErasurePlanWithoutSurveysSQL},
		{"preflight", schoolErasurePreflightWithoutSurveysSQL},
		{"residual", schoolErasureResidualWithoutSurveysSQL},
		{"delete", schoolErasureDeleteWithoutSurveysSQL},
		{"preview", schoolErasurePreviewWithoutSurveysSQL},
	} {
		if _, err := tx.ExecContext(ctx, step.sql); err != nil {
			return fmt.Errorf("restore school erasure %s: %w", step.name, err)
		}
	}
	return nil
}
