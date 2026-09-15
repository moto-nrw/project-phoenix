package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// Presence cleanup owns one transaction per school. Output is published only
// after commit; a failed school is reported while the remaining schools run.
func forEachPresenceTenant(cc *cleanupContext, operation string, action func(context.Context, int64) (func(), error)) error {
	ctx := tenant.WithUnitOfWork(context.Background(), cc.TenantRuntime)
	ids, err := listActiveTenantIDsForCLI(ctx, cc)
	if err != nil {
		return fmt.Errorf("%s: list active schools: %w", operation, err)
	}
	var failures []error
	for _, rawID := range ids {
		id, err := tenant.NewTenantID(rawID)
		if err != nil {
			return fmt.Errorf("%s: %w", operation, err)
		}
		var publish func()
		err = tenant.WithinTenant(ctx, id, func(txCtx context.Context) error {
			var actionErr error
			publish, actionErr = action(txCtx, rawID)
			return actionErr
		})
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: school %d: %w", operation, rawID, err))
			continue
		}
		mustFprintf(cc.output(), "\nSchool %d\n", rawID)
		publish()
	}
	return errors.Join(failures...)
}
