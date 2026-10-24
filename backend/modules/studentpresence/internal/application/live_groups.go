package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ListLiveGroups returns tenant-owned room sessions for the supplied IDs, including ended sessions.
func (s *Service) ListLiveGroups(ctx context.Context, ids []int64) ([]studentpresence.LiveGroup, error) {
	if ids == nil {
		ids = []int64{}
	}
	return s.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{IDs: ids})
}

func (s *Service) QueryLiveGroups(ctx context.Context, filter studentpresence.LiveGroupFilter) (result []studentpresence.LiveGroup, err error) {
	err = s.runWrite(ctx, "query_live_groups", func(ctx context.Context) (ports.Stats, error) {
		if filter.Limit < 0 || filter.Offset < 0 {
			return ports.Stats{}, errors.New("student presence: invalid pagination")
		}
		if err := validateLiveGroupFilterIDs(filter); err != nil {
			return ports.Stats{}, err
		}
		if filter.From != nil && filter.Until != nil && filter.From.After(*filter.Until) {
			return ports.Stats{}, errors.New("student presence: invalid group time range")
		}
		var stats ports.Stats
		result, stats, err = s.store.QueryLiveGroups(ctx, filter)
		return stats, err
	})
	return listResult(result, err)
}

// ListSupervisedLiveGroups returns the open room sessions the staff member
// supervises today. The owner decides "today" in the school calendar and
// counts a supervision from its start date up to, not including, its end date.
// Room display facts stay with Facilities; the session carries its RoomID.
func (s *Service) ListSupervisedLiveGroups(ctx context.Context, staffID int64) (result []studentpresence.LiveGroup, err error) {
	date := calendar.DateFromTime(s.now())
	err = s.runWrite(ctx, "list_supervised_live_groups", func(ctx context.Context) (ports.Stats, error) {
		if staffID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid staff ID")
		}
		supervisions, stats, err := s.store.QueryGroupSupervisions(ctx, ports.GroupSupervisionFilter{StaffID: &staffID, ActiveOn: &date})
		if err != nil {
			return stats, err
		}
		groupIDs := make([]int64, 0, len(supervisions))
		for _, supervision := range supervisions {
			groupIDs = append(groupIDs, supervision.GroupID)
		}
		var sessionStats ports.Stats
		result, sessionStats, err = s.store.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{IDs: groupIDs, OpenOnly: true})
		stats.Queries += sessionStats.Queries
		stats.Rows += sessionStats.Rows
		stats.StatementDuration += sessionStats.StatementDuration
		return stats, err
	})
	return listResult(result, err)
}

// ListOpenLiveGroupsForActivities returns the open room sessions of the given
// activity groups. Unlike LiveGroupFilter, nil or empty IDs match nothing.
func (s *Service) ListOpenLiveGroupsForActivities(ctx context.Context, ids []int64) ([]studentpresence.LiveGroup, error) {
	if ids == nil {
		ids = []int64{}
	}
	return s.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{ActivityGroupIDs: ids, OpenOnly: true})
}

func (s *Service) OccupiedActivityGroupIDs(ctx context.Context, ids []int64) (result []int64, err error) {
	err = s.runWrite(ctx, "occupied_activity_group_ids", func(ctx context.Context) (ports.Stats, error) {
		for _, id := range ids {
			if id <= 0 {
				return ports.Stats{}, errors.New("student presence: invalid activity group ID")
			}
		}
		var stats ports.Stats
		result, stats, err = s.store.OccupiedActivityGroupIDs(ctx, ids)
		return stats, err
	})
	return result, err
}

func (s *Service) ListGroupSupervisions(ctx context.Context, groupID int64) ([]studentpresence.GroupSupervision, error) {
	return s.QueryGroupSupervisions(ctx, studentpresence.GroupSupervisionFilter{GroupIDs: []int64{groupID}})
}

func parseOptionalDate(value *string) (*ports.Date, error) {
	if value == nil {
		return nil, nil
	}
	date, err := calendar.ParseDate(*value)
	if err != nil {
		return nil, err
	}
	return &date, nil
}

// supervisionRecordFilter types the calendar dates of a public supervision
// filter.
func supervisionRecordFilter(value studentpresence.GroupSupervisionFilter) (ports.GroupSupervisionFilter, error) {
	endedBy, err := parseOptionalDate(value.EndedBy)
	if err != nil {
		return ports.GroupSupervisionFilter{}, err
	}
	startedBefore, err := parseOptionalDate(value.StartedBefore)
	if err != nil {
		return ports.GroupSupervisionFilter{}, err
	}
	activeOn, err := parseOptionalDate(value.ActiveOn)
	if err != nil {
		return ports.GroupSupervisionFilter{}, err
	}
	return ports.GroupSupervisionFilter{StaffIDs: value.StaffIDs, EndedBy: endedBy, Limit: value.Limit, Offset: value.Offset, IDs: value.IDs, GroupIDs: value.GroupIDs, StaffID: value.StaffID, ForUpdate: value.ForUpdate, ActiveOn: activeOn, OpenOnly: value.OpenOnly, StartedBefore: startedBefore}, nil
}

func (s *Service) QueryGroupSupervisions(ctx context.Context, value studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error) {
	filter, err := supervisionRecordFilter(value)
	if err != nil {
		return nil, err
	}
	var rows []ports.GroupSupervision
	err = s.runQuery(ctx, filter.ForUpdate, "query_group_supervisions", func(ctx context.Context) (ports.Stats, error) {
		if filter.Limit < 0 || filter.Offset < 0 {
			return ports.Stats{}, errors.New("student presence: invalid supervision pagination")
		}
		if err := validateSupervisionRowFilter(filter); err != nil {
			return ports.Stats{}, err
		}
		if filter.ForUpdate {
			if err := s.tx.Require(ctx); err != nil {
				return ports.Stats{}, err
			}
		}
		if err := validateSupervisionGroupFilter(filter); err != nil {
			return ports.Stats{}, err
		}
		var stats ports.Stats
		var err error
		rows, stats, err = s.store.QueryGroupSupervisions(ctx, filter)
		return stats, err
	})
	if err != nil {
		return nil, err
	}
	return supervisionsToPublic(rows), nil
}

// StaffIDsWithSupervisionOn includes supervision starting or ending on the date.
func (s *Service) StaffIDsWithSupervisionOn(ctx context.Context, value string) (result []int64, err error) {
	date, err := calendar.ParseDate(value)
	if err != nil {
		return nil, err
	}
	err = s.runWrite(ctx, "staff_ids_with_supervision", func(ctx context.Context) (ports.Stats, error) {
		var stats ports.Stats
		result, stats, err = s.store.StaffIDsWithSupervisionOn(ctx, date)
		return stats, err
	})
	return result, err
}

// SupervisedRoomsOn returns distinct staff/room pairs for date-active supervision in open sessions.
func (s *Service) SupervisedRoomsOn(ctx context.Context, value string) (result []studentpresence.StaffRoomSupervision, err error) {
	date, err := calendar.ParseDate(value)
	if err != nil {
		return nil, err
	}
	err = s.runWrite(ctx, "supervised_rooms_on", func(ctx context.Context) (ports.Stats, error) {
		var stats ports.Stats
		result, stats, err = s.store.SupervisedRoomsOn(ctx, date)
		return stats, err
	})
	return listResult(result, err)
}

// validateLiveGroupFilterIDs rejects non-positive session, activity group,
// room and device IDs in a live group filter.
func validateLiveGroupFilterIDs(filter studentpresence.LiveGroupFilter) error {
	for _, ids := range [][]int64{filter.IDs, filter.ActivityGroupIDs} {
		for _, id := range ids {
			if id <= 0 {
				return errors.New("student presence: invalid group filter ID")
			}
		}
	}
	for _, id := range []*int64{filter.RoomID, filter.DeviceID} {
		if id != nil && *id <= 0 {
			return errors.New("student presence: invalid group filter ID")
		}
	}
	return nil
}

// validateSupervisionRowFilter rejects non-positive staff and supervision IDs
// in a supervision filter.
func validateSupervisionRowFilter(filter ports.GroupSupervisionFilter) error {
	for _, id := range filter.StaffIDs {
		if id <= 0 {
			return errors.New("student presence: invalid staff ID")
		}
	}
	for _, id := range filter.IDs {
		if id <= 0 {
			return errors.New("student presence: invalid supervision ID")
		}
	}
	return nil
}

// validateSupervisionGroupFilter rejects non-positive group IDs and a
// non-positive single staff ID in a supervision filter.
func validateSupervisionGroupFilter(filter ports.GroupSupervisionFilter) error {
	for _, id := range filter.GroupIDs {
		if id <= 0 {
			return errors.New("student presence: invalid group ID")
		}
	}
	if filter.StaffID != nil && *filter.StaffID <= 0 {
		return errors.New("student presence: invalid staff ID")
	}
	return nil
}
