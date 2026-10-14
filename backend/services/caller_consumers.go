package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/grouplive"
	grouplivelegacy "github.com/moto-nrw/project-phoenix/modules/grouplive/legacy"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	supervisiondashboardlegacy "github.com/moto-nrw/project-phoenix/modules/supervisiondashboard/legacy"
)

// GroupLiveCaller serves the caller context to the live-group adapters in
// their own group records.
type GroupLiveCaller struct{ *CallerRows }

var _ grouplivelegacy.CallerContext = GroupLiveCaller{}

func (c GroupLiveCaller) MyGroupRecords(ctx context.Context) ([]grouplive.GroupRecord, error) {
	groups, err := c.GetMyGroups(ctx)
	if err != nil {
		return nil, err
	}
	records := make([]grouplive.GroupRecord, 0, len(groups))
	for _, group := range groups {
		if group != nil {
			records = append(records, grouplive.GroupRecord{ID: group.ID, Name: group.Name, RoomID: group.RoomID})
		}
	}
	return records, nil
}

// SupervisionCaller serves the caller context to the supervision adapters
// in their own group records.
type SupervisionCaller struct{ *CallerRows }

var _ supervisiondashboardlegacy.CallerContext = SupervisionCaller{}

// CurrentStaffID resolves the caller's staff record; nil, without an error,
// for a caller who is no staff member.
func (c SupervisionCaller) CurrentStaffID(ctx context.Context) (*int64, error) {
	staffID, found, err := c.CallerRows.CurrentStaffID(ctx)
	if err != nil || !found {
		return nil, err
	}
	return &staffID, nil
}

// GetMySupervisedGroups returns the room sessions the caller supervises with
// their display relations.
func (c SupervisionCaller) GetMySupervisedGroups(ctx context.Context) ([]*studentpresence.SessionDetail, error) {
	return c.CallerRows.GetMySupervisedGroups(ctx)
}

// FullStudentAccess reports whether the caller sees unredacted student data.
func (c SupervisionCaller) FullStudentAccess(ctx context.Context) (bool, error) {
	return c.HasFullStudentAccess(ctx), nil
}

func (c SupervisionCaller) MyGroups(ctx context.Context) ([]supervisiondashboardlegacy.CallerGroup, error) {
	groups, err := c.GetMyGroups(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]supervisiondashboardlegacy.CallerGroup, 0, len(groups))
	for _, group := range groups {
		result = append(result, supervisiondashboardlegacy.CallerGroup{ID: group.ID, Name: group.Name, RoomID: group.RoomID})
	}
	return result, nil
}
