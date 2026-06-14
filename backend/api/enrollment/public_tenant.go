package enrollment

import (
	"context"
	"errors"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/tenant"
)

func (rs *Resource) resolvePublicTenantID(ctx context.Context, slug string) (int64, error) {
	var schoolID int64
	err := tenant.WithAdminTx(ctx, rs.db, func(adminCtx context.Context, _ bun.Tx) error {
		school, schoolErr := rs.SchoolService.GetSchoolBySlug(adminCtx, slug)
		if schoolErr != nil || school == nil || school.IsDeleted() {
			return errors.New("tenant not found")
		}
		schoolID = school.ID
		return nil
	})
	if err != nil {
		return 0, err
	}
	return schoolID, nil
}
