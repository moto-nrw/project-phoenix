package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

// restoreSchoolErasureWithoutSignatures restores the exact five function
// bodies installed before this migration added audit.signatures.
func restoreSchoolErasureWithoutSignatures(ctx context.Context, tx bun.Tx) error {
	if err := restoreSchoolErasurePlanWithoutSignatures(ctx, tx); err != nil {
		return err
	}
	if err := restoreSchoolErasurePreflightWithoutSignatures(ctx, tx); err != nil {
		return err
	}
	if err := restoreSchoolErasureDeleteWithoutSignatures(ctx, tx); err != nil {
		return err
	}
	if err := restoreSchoolErasurePreviewWithoutSignatures(ctx, tx); err != nil {
		return err
	}
	if err := restoreSchoolErasureResidualWithoutSignatures(ctx, tx); err != nil {
		return err
	}
	return nil
}
