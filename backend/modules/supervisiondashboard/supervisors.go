package supervisiondashboard

import (
	"context"
	"fmt"
	"sort"

	"github.com/moto-nrw/project-phoenix/sharedkernel/collation"
)

// SessionSupervisor is one current supervisor of a session, named for the
// page head (#4354).
type SessionSupervisor struct {
	StaffID int64  `json:"staff_id,string"`
	Name    string `json:"name"`
}

// nameSupervisors names the supervisors of every session tab and of every
// session in a released room. The tabs' supervisors come in one bulk read,
// the open rooms already carry theirs, and one name read serves both.
func (s *service) nameSupervisors(ctx context.Context, projection *Projection, openRooms openRoomInputs) error {
	groupSupervisors, err := s.groupSupervisorIDs(ctx, projection.Groups)
	if err != nil {
		return err
	}
	names, err := s.supervisorNames(ctx, supervisingStaffIDs(groupSupervisors, openRooms.sessions))
	if err != nil {
		return err
	}
	for i := range projection.Groups {
		projection.Groups[i].Supervisors = namedSupervisors(groupSupervisors[projection.Groups[i].ID], names)
	}
	for i := range projection.OpenRooms {
		sessions := projection.OpenRooms[i].Sessions
		for j := range sessions {
			sessions[j].Supervisors = namedSupervisors(openRooms.sessionByID[sessions[j].ActiveGroupID].SupervisorStaffIDs, names)
		}
	}
	return nil
}

// groupSupervisorIDs reads the current supervisors of the session tabs.
func (s *service) groupSupervisorIDs(ctx context.Context, groups []Group) (map[int64][]int64, error) {
	if len(groups) == 0 {
		return map[int64][]int64{}, nil
	}
	ids := make([]int64, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.ID)
	}
	loaded, err := s.deps.Sessions.SupervisorStaffIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load session supervisors: %w", err)
	}
	if loaded == nil {
		return map[int64][]int64{}, nil
	}
	return loaded, nil
}

// supervisorNames names the staff in one read; nobody to name, no read.
func (s *service) supervisorNames(ctx context.Context, staffIDs []int64) (map[int64]string, error) {
	if len(staffIDs) == 0 {
		return map[int64]string{}, nil
	}
	loaded, err := s.deps.Staff.StaffNames(ctx, staffIDs)
	if err != nil {
		return nil, fmt.Errorf("load supervisor names: %w", err)
	}
	if loaded == nil {
		return map[int64]string{}, nil
	}
	return loaded, nil
}

// supervisingStaffIDs is every staff id supervising a tab or an open-room
// session, once each, in ascending order.
func supervisingStaffIDs(groupSupervisors map[int64][]int64, sessions []RunningSession) []int64 {
	seen := map[int64]struct{}{}
	for _, ids := range groupSupervisors {
		for _, id := range ids {
			seen[id] = struct{}{}
		}
	}
	for _, session := range sessions {
		for _, id := range session.SupervisorStaffIDs {
			seen[id] = struct{}{}
		}
	}
	result := make([]int64, 0, len(seen))
	for id := range seen {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

// namedSupervisors keeps the staff with a name, in German dictionary order;
// nil when nobody is left.
func namedSupervisors(staffIDs []int64, names map[int64]string) []SessionSupervisor {
	var result []SessionSupervisor
	for _, id := range staffIDs {
		if name := names[id]; name != "" {
			result = append(result, SessionSupervisor{StaffID: id, Name: name})
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		return collation.CompareGerman(result[i].Name, result[j].Name) < 0
	})
	return result
}
