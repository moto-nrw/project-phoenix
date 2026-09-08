package supervisiondashboard

// The shared open-room view (#3065).
//
// A released room is a place the administration opened permanently, so it is
// reachable whether or not anyone supervises it and whether or not anything
// runs there. That is the difference from the navigation this replaces, which
// derived its rooms from running supervisions and therefore could not show an
// empty one at all.
//
// Everything currently recorded in such a room converges into ONE view: all
// running sessions of that room contribute, each child appears once, and the
// offering the child is recorded under travels with them. Seeing a room here
// grants nothing — no supervision, no booking right. The caller's own
// supervision is reported as a flag so the client can keep the two apart.

import (
	"context"
	"fmt"
	"sort"
	"time"

	activeService "github.com/moto-nrw/project-phoenix/services/active"
)

// OpenRoom is one permanently released room and everyone currently in it.
type OpenRoom struct {
	RoomID int64  `json:"room_id,string"`
	Name   string `json:"name"`
	// IsUserSupervising reports whether the caller supervises any session in
	// this room right now. It is a display hint that separates a shared room
	// from the caller's own supervision — never a visibility or permission
	// decision.
	IsUserSupervising bool `json:"is_user_supervising"`
	// ActiveGroupIDs are the running sessions that feed this view, oldest
	// first. Empty for a released room with nothing running in it.
	ActiveGroupIDs []int64           `json:"active_group_ids"`
	StudentCount   int               `json:"student_count"`
	Students       []OpenRoomStudent `json:"students"`
}

// OpenRoomStudent is one child in a released room, listed once no matter how
// many sessions of that room they could be attributed to.
type OpenRoomStudent struct {
	StudentID   int64  `json:"student_id,string"`
	StudentName string `json:"student_name"`
	SchoolClass string `json:"school_class"`
	OGSGroup    string `json:"ogs_group,omitempty"`
	// ActivityName is the offering the child is recorded under. Empty means
	// the child is in the room without one — a state this slice can display
	// but not yet create (#3066 adds that), so an empty value here is never
	// invented: it only appears when a session genuinely has no template.
	ActivityName  string    `json:"activity_name,omitempty"`
	ActiveGroupID int64     `json:"active_group_id,string"`
	CheckInTime   time.Time `json:"check_in_time"`
}

// OpenRoomDirectory answers which rooms are released. Consumer-owned port over
// the Facilities capability: this projection reads room configuration, it does
// not own it.
type OpenRoomDirectory interface {
	ListReleasedRooms(ctx context.Context) ([]ReleasedRoom, error)
}

// ReleasedRoom is the room shape this projection needs — nothing more.
type ReleasedRoom struct {
	ID   int64
	Name string
}

// OpenRoomSessions answers which sessions run in the released rooms and which
// offering each is backed by. Consumer-owned port over Student Presence and
// Timetable & Activities, which keep ownership of sessions and offerings.
type OpenRoomSessions interface {
	ListRunningSessionsInRooms(ctx context.Context, roomIDs []int64) ([]RunningSession, error)
}

// OpenRoomVisits reads the open visits of several sessions at once.
//
// Declared here rather than added to activeService.Service on purpose: that
// interface is wide and hand-mocked in a dozen packages, so growing it makes
// every unrelated test double stop compiling. A consumer-owned port names
// exactly what this projection needs, and the concrete presence service
// already satisfies it.
type OpenRoomVisits interface {
	GetActiveGroupVisitsWithDisplayForGroups(ctx context.Context, activeGroupIDs []int64) ([]*activeService.VisitWithStudentDisplay, error)
}

// RunningSession is one open session located in a room, with the offering it
// belongs to. ActivityName is empty when the session runs without a template.
type RunningSession struct {
	ActiveGroupID int64
	RoomID        int64
	ActivityName  string
	StartTime     time.Time
	// SupervisorStaffIDs are the staff currently supervising this session.
	SupervisorStaffIDs []int64
}

// loadOpenRooms assembles the shared view with a constant number of queries:
// one for the released rooms, one for their running sessions, one for the open
// visits of those sessions. It deliberately does not loop per room or per
// session — that would grow with the timetable instead of the configuration.
func (s *service) loadOpenRooms(ctx context.Context, projection *Projection, staffID *int64) error {
	if s.deps.OpenRoomDirectory == nil || s.deps.OpenRoomSessions == nil || s.deps.OpenRoomVisits == nil {
		return nil
	}

	rooms, err := s.deps.OpenRoomDirectory.ListReleasedRooms(ctx)
	if err != nil {
		return fmt.Errorf("load released rooms: %w", err)
	}
	if len(rooms) == 0 {
		return nil
	}

	roomIDs := make([]int64, 0, len(rooms))
	for _, room := range rooms {
		roomIDs = append(roomIDs, room.ID)
	}

	sessions, err := s.deps.OpenRoomSessions.ListRunningSessionsInRooms(ctx, roomIDs)
	if err != nil {
		return fmt.Errorf("load open-room sessions: %w", err)
	}

	sessionIDs := make([]int64, 0, len(sessions))
	sessionByID := make(map[int64]RunningSession, len(sessions))
	for _, session := range sessions {
		sessionIDs = append(sessionIDs, session.ActiveGroupID)
		sessionByID[session.ActiveGroupID] = session
	}

	var visits []*activeService.VisitWithStudentDisplay
	if len(sessionIDs) > 0 {
		visits, err = s.deps.OpenRoomVisits.GetActiveGroupVisitsWithDisplayForGroups(ctx, sessionIDs)
		if err != nil {
			return fmt.Errorf("load open-room visits: %w", err)
		}
	}

	projection.OpenRooms = assembleOpenRooms(rooms, sessions, sessionByID, visits, staffID)
	return nil
}

// assembleOpenRooms is the pure part: it turns the three loaded lists into one
// entry per released room. Kept separate from the loading so the aggregation
// rules — deduplication, ordering, the supervision flag — are testable without
// a database.
func assembleOpenRooms(
	rooms []ReleasedRoom,
	sessions []RunningSession,
	sessionByID map[int64]RunningSession,
	visits []*activeService.VisitWithStudentDisplay,
	staffID *int64,
) []OpenRoom {
	sessionsByRoom := make(map[int64][]RunningSession, len(rooms))
	for _, session := range sessions {
		sessionsByRoom[session.RoomID] = append(sessionsByRoom[session.RoomID], session)
	}

	// One child per room, even when several sessions of that room could claim
	// them. The earliest check-in wins: it is the arrival the room actually
	// saw, and picking "the newest session" is exactly the rule #3062 rejects.
	type roomStudentKey struct{ roomID, studentID int64 }
	chosen := make(map[roomStudentKey]OpenRoomStudent, len(visits))
	for _, visit := range visits {
		if visit == nil {
			continue
		}
		session, found := sessionByID[visit.ActiveGroupID]
		if !found {
			continue
		}
		key := roomStudentKey{roomID: session.RoomID, studentID: visit.StudentID}
		candidate := OpenRoomStudent{
			StudentID:     visit.StudentID,
			StudentName:   visit.FirstName + " " + visit.LastName,
			SchoolClass:   visit.SchoolClass,
			OGSGroup:      visit.OGSGroupName,
			ActivityName:  session.ActivityName,
			ActiveGroupID: visit.ActiveGroupID,
			CheckInTime:   visit.EntryTime,
		}
		existing, seen := chosen[key]
		if !seen || candidate.CheckInTime.Before(existing.CheckInTime) {
			chosen[key] = candidate
		}
	}

	studentsByRoom := make(map[int64][]OpenRoomStudent, len(rooms))
	for key, student := range chosen {
		studentsByRoom[key.roomID] = append(studentsByRoom[key.roomID], student)
	}

	result := make([]OpenRoom, 0, len(rooms))
	for _, room := range rooms {
		roomSessions := sessionsByRoom[room.ID]
		sort.Slice(roomSessions, func(i, j int) bool {
			return roomSessions[i].StartTime.Before(roomSessions[j].StartTime)
		})
		activeGroupIDs := make([]int64, 0, len(roomSessions))
		supervising := false
		for _, session := range roomSessions {
			activeGroupIDs = append(activeGroupIDs, session.ActiveGroupID)
			if staffID != nil {
				for _, supervisor := range session.SupervisorStaffIDs {
					if supervisor == *staffID {
						supervising = true
						break
					}
				}
			}
		}

		students := studentsByRoom[room.ID]
		sort.Slice(students, func(i, j int) bool {
			if students[i].StudentName != students[j].StudentName {
				return students[i].StudentName < students[j].StudentName
			}
			return students[i].StudentID < students[j].StudentID
		})
		if students == nil {
			students = []OpenRoomStudent{}
		}

		result = append(result, OpenRoom{
			RoomID:            room.ID,
			Name:              room.Name,
			IsUserSupervising: supervising,
			ActiveGroupIDs:    activeGroupIDs,
			StudentCount:      len(students),
			Students:          students,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		return result[i].RoomID < result[j].RoomID
	})
	return result
}
