package postgres

import (
	context "context"
)

// assignments reads the template binding from the employment profile and
// keeps only the memberships School Membership still holds as live.
type Assignments struct {
	Store *Store
	Live  func(context.Context, []int64) ([]int64, error)
}

func (a Assignments) AssignedStaffIDs(ctx context.Context, workTimeModelID int64) ([]int64, error) {
	ids, err := a.Store.StaffOnWorkTimeModel(ctx, workTimeModelID)
	if err != nil || len(ids) == 0 {
		return ids, err
	}
	return a.Live(ctx, ids)
}
func (a Assignments) RebaseAnchor(ctx context.Context, staffIDs []int64, anchorDate string) error {
	if len(staffIDs) == 0 {
		return nil
	}
	return a.Store.RebaseStaffRotationAnchor(ctx, staffIDs, anchorDate)
}
