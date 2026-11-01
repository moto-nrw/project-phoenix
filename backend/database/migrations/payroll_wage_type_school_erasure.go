package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

// The installers of the 1.15.725 (#4807) school erasure functions; their SQL
// lives in the payroll_wage_type_school_erasure_*.go files.

// 1.15.725 (#4807): the 1.15.724 (#4808) plan with the wage types and surcharge rules.
func installPayrollWageTypeErasurePlan(ctx context.Context, tx bun.Tx) error {
	_, err := tx.ExecContext(ctx, payrollWageTypeErasurePlanSQL)
	return err
}

// 1.15.725 (#4807): the 1.15.724 (#4808) preflight with the wage types and surcharge rules.
func installPayrollWageTypeErasurePreflight(ctx context.Context, tx bun.Tx) error {
	_, err := tx.ExecContext(ctx, payrollWageTypeErasurePreflightSQL)
	return err
}

// 1.15.725 (#4807): the 1.15.724 (#4808) residual with the wage types and surcharge rules.
func installPayrollWageTypeErasureResidual(ctx context.Context, tx bun.Tx) error {
	_, err := tx.ExecContext(ctx, payrollWageTypeErasureResidualSQL)
	return err
}

// 1.15.725 (#4807): the 1.15.724 (#4808) delete with the wage types and surcharge rules.
func installPayrollWageTypeErasureDelete(ctx context.Context, tx bun.Tx) error {
	_, err := tx.ExecContext(ctx, payrollWageTypeErasureDeleteSQL)
	return err
}

// 1.15.725 (#4807): the 1.15.724 (#4808) preview with the wage types and surcharge rules.
func installPayrollWageTypeErasurePreview(ctx context.Context, tx bun.Tx) error {
	_, err := tx.ExecContext(ctx, payrollWageTypeErasurePreviewSQL)
	return err
}
