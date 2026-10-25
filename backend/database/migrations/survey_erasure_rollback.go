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
	if err := restoreSchoolErasurePlanBeforeSurveys(ctx, tx); err != nil {
		return fmt.Errorf("restore school erasure plan: %w", err)
	}
	if err := restoreSchoolErasurePreflightBeforeSurveys(ctx, tx); err != nil {
		return fmt.Errorf("restore school erasure preflight: %w", err)
	}
	if err := restoreSchoolErasureResidualBeforeSurveys(ctx, tx); err != nil {
		return fmt.Errorf("restore school erasure residual: %w", err)
	}
	if err := restoreSchoolErasureDeleteBeforeSurveys(ctx, tx); err != nil {
		return fmt.Errorf("restore school erasure delete: %w", err)
	}
	if err := restoreSchoolErasurePreviewBeforeSurveys(ctx, tx); err != nil {
		return fmt.Errorf("restore school erasure preview: %w", err)
	}
	return nil
}
