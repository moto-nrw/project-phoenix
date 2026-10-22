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
	groupSupervisors := map[int64][]int64{}
	if len(projection.Groups) > 0 {
		ids := make([]int64, 0, len(projection.Groups))
		for _, group := range projection.Groups {
			ids = append(ids, group.ID)
		}
		loaded, err := s.deps.Sessions.SupervisorStaffIDs(ctx, ids)
		if err != nil {
			return fmt.Errorf("load session supervisors: %w", err)
		}
		if loaded != nil {
			groupSupervisors = loaded
		}
	}

	staffIDs := make([]int64, 0)
	seen := map[int64]struct{}{}
	collect := func(ids []int64) {
		for _, id := range ids {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				staffIDs = append(staffIDs, id)
			}
		}
	}
	for _, ids := range groupSupervisors {
		collect(ids)
	}
	for _, session := range openRooms.sessions {
		collect(session.SupervisorStaffIDs)
	}

	names := map[int64]string{}
	if len(staffIDs) > 0 {
		sort.Slice(staffIDs, func(i, j int) bool { return staffIDs[i] < staffIDs[j] })
		loaded, err := s.deps.Staff.StaffNames(ctx, staffIDs)
		if err != nil {
			return fmt.Errorf("load supervisor names: %w", err)
		}
		if loaded != nil {
			names = loaded
		}
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
