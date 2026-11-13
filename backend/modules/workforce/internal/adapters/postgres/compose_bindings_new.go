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
func (a Assignments) AssignedStaffByModel(ctx context.Context) (map[int64][]int64, error) {
	byModel, err := a.Store.StaffByWorkTimeModel(ctx)
	if err != nil || len(byModel) == 0 {
		return byModel, err
	}
	bound := make([]int64, 0, len(byModel))
	for _, ids := range byModel {
		bound = append(bound, ids...)
	}
	live, err := a.Live(ctx, bound)
	if err != nil {
		return nil, err
	}
	isLive := make(map[int64]bool, len(live))
	for _, id := range live {
		isLive[id] = true
	}
	result := make(map[int64][]int64, len(byModel))
	for modelID, ids := range byModel {
		for _, id := range ids {
			if isLive[id] {
				result[modelID] = append(result[modelID], id)
			}
		}
	}
	return result, nil
}

func (a Assignments) RebaseAnchor(ctx context.Context, staffIDs []int64, anchorDate string) error {
	if len(staffIDs) == 0 {
		return nil
	}
	return a.Store.RebaseStaffRotationAnchor(ctx, staffIDs, anchorDate)
}
