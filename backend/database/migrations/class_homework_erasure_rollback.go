package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// restoreSchoolErasureBeforeHomework reinstalls the frozen pre-1.15.510
// definitions, including the previous plan and FK fingerprint, in the same
// transaction that drops the homework tables.
func restoreSchoolErasureBeforeHomework(ctx context.Context, tx bun.Tx) error {
	if err := restoreSchoolErasurePlanBeforeHomework(ctx, tx); err != nil {
		return fmt.Errorf("restore school erasure plan: %w", err)
	}
	if err := restoreSchoolErasurePreflightBeforeHomework(ctx, tx); err != nil {
		return fmt.Errorf("restore school erasure preflight: %w", err)
	}
	if err := restoreSchoolErasureResidualBeforeHomework(ctx, tx); err != nil {
		return fmt.Errorf("restore school erasure residual: %w", err)
	}
	if err := restoreSchoolErasureDeleteBeforeHomework(ctx, tx); err != nil {
		return fmt.Errorf("restore school erasure delete: %w", err)
	}
	if err := restoreSchoolErasurePreviewBeforeHomework(ctx, tx); err != nil {
		return fmt.Errorf("restore school erasure preview: %w", err)
	}
	return nil
}
