package compose

import "context"

func (e engine) ListGuardianSchoolIDs(ctx context.Context, accountID int64) ([]int64, error) {
	ids, err := e.guardianSchools.ListGuardianSchoolIDs(ctx, accountID)
	return ids, mapError(err)
}
