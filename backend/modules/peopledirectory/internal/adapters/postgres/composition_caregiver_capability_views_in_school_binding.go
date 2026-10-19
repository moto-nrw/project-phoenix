package postgres

import (
	"context"
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

func CaregiverCapabilityViewsInSchoolBinding(ctx context.Context, schoolID int64, fn func(context.Context) (json.RawMessage, error), execute0 func(ctx context.Context, _ *bun.DB, fn func(context.Context, bun.Tx) error) error, execute1 *bun.DB) (json.RawMessage, error) {
	var view json.RawMessage
	err := execute0(ctx, execute1, func(adminCtx context.Context, _ bun.Tx) error {
		var viewErr error
		view, viewErr = fn(tenant.WithTenantID(adminCtx, schoolID))
		return viewErr
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}
