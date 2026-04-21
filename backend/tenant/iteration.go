package tenant

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/uptrace/bun"
)

// ActiveSchoolLister lists active tenants for iteration helpers.
// platform.SchoolRepository satisfies this interface.
type ActiveSchoolLister interface {
	ListActive(ctx context.Context) ([]platform.School, error)
}

// ForEachActive executes fn once per active tenant, each call wrapped in
// WithTenantTx. Per-tenant errors are logged and swallowed so one broken
// tenant does not halt the loop — the scheduler and the CLI both want
// best-effort iteration. The admin listing is wrapped in WithAdminTx.
//
// Returns a non-nil error only when the admin listing itself fails. Use
// opName for structured-log correlation across iteration runs.
func ForEachActive(
	ctx context.Context,
	db *bun.DB,
	lister ActiveSchoolLister,
	logger *slog.Logger,
	opName string,
	fn func(ctx context.Context, tenantID int64) error,
) error {
	if db == nil {
		return fmt.Errorf("tenant.ForEachActive: db is required")
	}
	if lister == nil {
		return fmt.Errorf("tenant.ForEachActive: lister is required")
	}
	if logger == nil {
		logger = slog.Default()
	}

	var schools []platform.School
	if err := WithAdminTx(ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		var e error
		schools, e = lister.ListActive(txCtx)
		return e
	}); err != nil {
		return fmt.Errorf("list active tenants for %s: %w", opName, err)
	}

	for _, school := range schools {
		tenantErr := WithTenantTx(ctx, db, school.ID, func(txCtx context.Context, _ bun.Tx) error {
			return fn(txCtx, school.ID)
		})
		if tenantErr != nil {
			logger.Error("tenant operation failed, continuing to next tenant",
				slog.String("operation", opName),
				slog.Int64("tenant_id", school.ID),
				slog.String("error", tenantErr.Error()),
			)
		}
	}

	return nil
}
