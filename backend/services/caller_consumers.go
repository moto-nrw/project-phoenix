package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/grouplive"
	grouplivelegacy "github.com/moto-nrw/project-phoenix/modules/grouplive/legacy"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	supervisiondashboardlegacy "github.com/moto-nrw/project-phoenix/modules/supervisiondashboard/legacy"
)

// groupLiveCaller serves the caller context to the live-group adapters in
// their own group records.
type groupLiveCaller struct{ *repositories.CallerRows }

var _ grouplivelegacy.CallerContext = groupLiveCaller{}

func (c groupLiveCaller) MyGroupRecords(ctx context.Context) ([]grouplive.GroupRecord, error) {
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

// supervisionCaller serves the caller context to the supervision adapters
// in their own group records.
type supervisionCaller struct{ *repositories.CallerRows }

var _ supervisiondashboardlegacy.CallerContext = supervisionCaller{}

// CurrentStaffID resolves the caller's staff record; nil, without an error,
// for a caller who is no staff member.
func (c supervisionCaller) CurrentStaffID(ctx context.Context) (*int64, error) {
	staffID, found, err := c.CallerRows.CurrentStaffID(ctx)
	if err != nil || !found {
		return nil, err
	}
	return &staffID, nil
}

// GetMySupervisedGroups returns the room sessions the caller supervises with
// their display relations.
func (c supervisionCaller) GetMySupervisedGroups(ctx context.Context) ([]*studentpresence.SessionDetail, error) {
	groups, err := c.CallerRows.GetMySupervisedGroups(ctx)
	if err != nil {
		return nil, err
	}
	return presenceservice.SessionDetails(groups), nil
}

// FullStudentAccess reports whether the caller sees unredacted student data.
func (c supervisionCaller) FullStudentAccess(ctx context.Context) (bool, error) {
	return c.HasFullStudentAccess(ctx), nil
}

func (c supervisionCaller) MyGroups(ctx context.Context) ([]supervisiondashboardlegacy.CallerGroup, error) {
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
