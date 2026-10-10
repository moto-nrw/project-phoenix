package enrollmenthttp

import (
	"context"
	"errors"
)

func (rs *Resource) resolvePublicTenantID(ctx context.Context, slug string) (int64, error) {
	var schoolID int64
	err := withinAdmin(ctx, func(adminCtx context.Context) error {
		school, schoolErr := rs.SchoolService.GetSchoolBySlug(adminCtx, slug)
		if schoolErr != nil || school == nil || school.Deleted {
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
