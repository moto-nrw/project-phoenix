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
	"strconv"
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
	// first. Empty for a released room with nothing running in it. Decimal
	// strings, like every other id on this wire: encoding/json has no
	// `,string` for slice elements, and a bare number would be the one id the
	// frontend has to treat differently.
	ActiveGroupIDs []string          `json:"active_group_ids"`
	StudentCount   int               `json:"student_count"`
	Students       []OpenRoomStudent `json:"students"`
}

// OpenRoomStudent is one child in a released room, listed once no matter how
// many sessions of that room they could be attributed to.
//
// It embeds the same Visit shape a session's roster uses, and under the same
// access and photo gates, so the page renders a released room with the roster
// it already has instead of a second, poorer student shape.
type OpenRoomStudent struct {
	Visit
	// ActivityName is the offering the child is recorded under. Empty means
	// the child is in the room without one — a state this slice can display
	// but not yet create (#3066 adds that), so an empty value here is never
	// invented: it only appears when a session genuinely has no template.
	ActivityName string `json:"activity_name,omitempty"`
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

// openRoomInputs is what the three port reads produced, already joined and
// deduplicated but not yet rendered.
//
// It is kept as an intermediate on purpose: the students of every released
// room take part in the same attendance, tracking and planning-time loads as
// the selected session's roster, so those must run once over the union rather
// than once per room.
type openRoomInputs struct {
	rooms       []ReleasedRoom
	sessions    []RunningSession
	sessionByID map[int64]RunningSession
	// visitsByRoom holds one row per child per room, ordered for display.
	visitsByRoom map[int64][]*activeService.VisitWithStudentDisplay
}

// studentIDs are all children currently recorded in any released room, each
// one once across every room.
func (in openRoomInputs) studentIDs() []int64 {
	seen := map[int64]struct{}{}
	ids := make([]int64, 0)
	for _, rows := range in.visitsByRoom {
		for _, row := range rows {
			if _, ok := seen[row.StudentID]; ok {
				continue
			}
			seen[row.StudentID] = struct{}{}
			ids = append(ids, row.StudentID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// loadOpenRoomInputs reads the shared view's sources with a constant number of
// queries: one for the released rooms, one for their running sessions, one for
// the open visits of those sessions. It deliberately does not loop per room or
// per session — that would grow with the timetable instead of the configuration.
func (s *service) loadOpenRoomInputs(ctx context.Context) (openRoomInputs, error) {
	if s.deps.OpenRoomDirectory == nil || s.deps.OpenRoomSessions == nil || s.deps.OpenRoomVisits == nil {
		return openRoomInputs{}, nil
	}

	rooms, err := s.deps.OpenRoomDirectory.ListReleasedRooms(ctx)
	if err != nil {
		return openRoomInputs{}, fmt.Errorf("load released rooms: %w", err)
	}
	if len(rooms) == 0 {
		return openRoomInputs{}, nil
	}

	roomIDs := make([]int64, 0, len(rooms))
	for _, room := range rooms {
		roomIDs = append(roomIDs, room.ID)
	}

	sessions, err := s.deps.OpenRoomSessions.ListRunningSessionsInRooms(ctx, roomIDs)
	if err != nil {
		return openRoomInputs{}, fmt.Errorf("load open-room sessions: %w", err)
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
			return openRoomInputs{}, fmt.Errorf("load open-room visits: %w", err)
		}
	}

	return openRoomInputs{
		rooms:        rooms,
		sessions:     sessions,
		sessionByID:  sessionByID,
		visitsByRoom: groupOpenRoomVisits(sessionByID, visits),
	}, nil
}

// groupOpenRoomVisits joins the open visits onto their room and drops the
// duplicates. It is the aggregation promise of #3062 in one place, so it can
// be read and tested without a database.
//
// Today `uniq_active_visits_open_per_student` already keeps a child to one
// open visit, so the deduplication below cannot trigger through that path.
// It stays because the promise belongs to this view, not to an index in
// another owner's schema: the rule is "one row per child per room, chosen by
// the arrival the room saw", and it must hold whatever the caller hands over.
func groupOpenRoomVisits(
	sessionByID map[int64]RunningSession,
	visits []*activeService.VisitWithStudentDisplay,
) map[int64][]*activeService.VisitWithStudentDisplay {
	// One child per room, even when several sessions of that room could claim
	// them. The earliest check-in wins: it is the arrival the room actually
	// saw, and picking "the newest session" is exactly the rule #3062 rejects.
	type roomStudentKey struct{ roomID, studentID int64 }
	chosen := make(map[roomStudentKey]*activeService.VisitWithStudentDisplay, len(visits))
	for _, visit := range visits {
		if visit == nil {
			continue
		}
		session, found := sessionByID[visit.ActiveGroupID]
		if !found {
			continue
		}
		key := roomStudentKey{roomID: session.RoomID, studentID: visit.StudentID}
		existing, seen := chosen[key]
		if !seen || visit.EntryTime.Before(existing.EntryTime) {
			chosen[key] = visit
		}
	}

	byRoom := make(map[int64][]*activeService.VisitWithStudentDisplay, len(chosen))
	for key, visit := range chosen {
		byRoom[key.roomID] = append(byRoom[key.roomID], visit)
	}
	for roomID, rows := range byRoom {
		sort.Slice(rows, func(i, j int) bool {
			left, right := displayName(rows[i]), displayName(rows[j])
			if left != right {
				return left < right
			}
			return rows[i].StudentID < rows[j].StudentID
		})
		byRoom[roomID] = rows
	}
	return byRoom
}

// assembleOpenRooms turns the joined inputs into one entry per released room.
// Pure, so the rules it encodes — which rooms appear, the ordering, and what
// the supervision flag means — are testable without a database.
func assembleOpenRooms(in openRoomInputs, display visitDisplay, staffID *int64) []OpenRoom {
	sessionsByRoom := make(map[int64][]RunningSession, len(in.rooms))
	for _, session := range in.sessions {
		sessionsByRoom[session.RoomID] = append(sessionsByRoom[session.RoomID], session)
	}

	result := make([]OpenRoom, 0, len(in.rooms))
	for _, room := range in.rooms {
		roomSessions := sessionsByRoom[room.ID]
		sort.Slice(roomSessions, func(i, j int) bool {
			return roomSessions[i].StartTime.Before(roomSessions[j].StartTime)
		})
		activeGroupIDs := make([]string, 0, len(roomSessions))
		supervising := false
		for _, session := range roomSessions {
			activeGroupIDs = append(activeGroupIDs, strconv.FormatInt(session.ActiveGroupID, 10))
			if staffID != nil {
				for _, supervisor := range session.SupervisorStaffIDs {
					if supervisor == *staffID {
						supervising = true
						break
					}
				}
			}
		}

		rows := in.visitsByRoom[room.ID]
		students := make([]OpenRoomStudent, 0, len(rows))
		for _, row := range rows {
			students = append(students, OpenRoomStudent{
				Visit:        buildVisit(row, display),
				ActivityName: in.sessionByID[row.ActiveGroupID].ActivityName,
			})
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
