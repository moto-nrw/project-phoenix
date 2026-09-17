package supervisiondashboard

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
)

type openRoomInputs struct {
	rooms        []ReleasedRoom
	sessions     []RunningSession
	sessionByID  map[int64]RunningSession
	visitsByRoom map[int64][]VisitRecord
	// blocks holds the running timetable block behind a session, keyed by
	// session id (#3281).
	blocks map[int64]SessionBlock
}

func (in openRoomInputs) studentIDs() []int64 {
	seen := map[int64]struct{}{}
	ids := make([]int64, 0)
	for _, rows := range in.visitsByRoom {
		for _, row := range rows {
			if _, ok := seen[row.StudentID]; !ok {
				seen[row.StudentID] = struct{}{}
				ids = append(ids, row.StudentID)
			}
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func (s *service) loadOpenRoomInputs(ctx context.Context, caller Caller, businessDay Date) (openRoomInputs, error) {
	if s.deps.Rooms == nil {
		return openRoomInputs{}, nil
	}
	rooms, err := s.deps.Rooms.Released(ctx)
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
	sessions, err := s.deps.Sessions.InRooms(ctx, roomIDs)
	if err != nil {
		return openRoomInputs{}, fmt.Errorf("load open-room sessions: %w", err)
	}
	sessionByID := make(map[int64]RunningSession, len(sessions))
	sessionIDs := make([]int64, 0, len(sessions))
	for _, session := range sessions {
		sessionByID[session.ActiveGroupID] = session
		sessionIDs = append(sessionIDs, session.ActiveGroupID)
	}
	visits := make([]VisitRecord, 0)
	if len(sessionIDs) > 0 {
		visits, err = s.deps.Presence.OpenVisitsOfSessions(ctx, sessionIDs)
		if err != nil {
			return openRoomInputs{}, fmt.Errorf("load open-room visits: %w", err)
		}
	}
	blocks, err := s.loadSessionBlocks(ctx, caller, businessDay, sessions)
	if err != nil {
		return openRoomInputs{}, err
	}
	return openRoomInputs{
		rooms: rooms, sessions: sessions, sessionByID: sessionByID,
		visitsByRoom: groupOpenRoomVisits(sessionByID, visits),
		blocks:       blocks,
	}, nil
}

// loadSessionBlocks asks the Timetable owner once for the blocks behind every
// session of the released rooms (#3281). The room's own session of
// independent stays is never a block, even when a web start put a
// spontaneous instance behind it, so it is not asked about. Like the other
// schedule sections, no block is named without schedules:read.
func (s *service) loadSessionBlocks(ctx context.Context, caller Caller, businessDay Date, sessions []RunningSession) (map[int64]SessionBlock, error) {
	blocks := map[int64]SessionBlock{}
	if !caller.CanReadSchedules {
		return blocks, nil
	}
	supervisors := make(map[int64][]int64, len(sessions))
	for _, session := range sessions {
		if !session.IndependentStays {
			supervisors[session.ActiveGroupID] = session.SupervisorStaffIDs
		}
	}
	if len(supervisors) == 0 {
		return blocks, nil
	}
	rows, err := s.deps.Schedule.SessionBlocks(ctx, SessionBlocksQuery{
		AccountID:   caller.AccountID,
		TokenAdmin:  caller.TokenAdmin,
		Date:        businessDay,
		Supervisors: supervisors,
	})
	if err != nil {
		return nil, fmt.Errorf("load open-room blocks: %w", err)
	}
	for _, row := range rows {
		if _, asked := supervisors[row.ActiveGroupID]; asked {
			blocks[row.ActiveGroupID] = row
		}
	}
	return blocks, nil
}

func groupOpenRoomVisits(sessionByID map[int64]RunningSession, visits []VisitRecord) map[int64][]VisitRecord {
	type roomStudentKey struct{ roomID, studentID int64 }
	chosen := make(map[roomStudentKey]VisitRecord, len(visits))
	for _, visit := range visits {
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
	byRoom := make(map[int64][]VisitRecord, len(chosen))
	for key, visit := range chosen {
		byRoom[key.roomID] = append(byRoom[key.roomID], visit)
	}
	for roomID := range byRoom {
		sort.Slice(byRoom[roomID], func(i, j int) bool {
			left := strings.TrimSpace(byRoom[roomID][i].FirstName + " " + byRoom[roomID][i].LastName)
			right := strings.TrimSpace(byRoom[roomID][j].FirstName + " " + byRoom[roomID][j].LastName)
			if left != right {
				return left < right
			}
			return byRoom[roomID][i].StudentID < byRoom[roomID][j].StudentID
		})
	}
	return byRoom
}

func (s *service) assembleOpenRooms(in openRoomInputs, attendance map[int64]Attendance, fullAccess, photosEnabled bool, staffID *int64, adminScope bool) []OpenRoom {
	sessionsByRoom := make(map[int64][]RunningSession, len(in.rooms))
	for _, session := range in.sessions {
		sessionsByRoom[session.RoomID] = append(sessionsByRoom[session.RoomID], session)
	}
	result := make([]OpenRoom, 0, len(in.rooms))
	for _, room := range in.rooms {
		sessions := sessionsByRoom[room.ID]
		sort.Slice(sessions, func(i, j int) bool { return sessions[i].StartTime.Before(sessions[j].StartTime) })
		rows := in.visitsByRoom[room.ID]
		students := make([]OpenRoomStudent, 0, len(rows))
		childrenBySession := make(map[int64]int, len(sessions))
		for _, row := range rows {
			session := in.sessionByID[row.ActiveGroupID]
			student := OpenRoomStudent{
				Visit:       s.buildVisit(row, attendance, fullAccess, photosEnabled),
				Independent: session.IndependentStays,
			}
			// A device-less system session is the room's own stay; naming it
			// would present an independent stay as activity participation.
			if !session.IndependentStays {
				student.ActivityName = session.ActivityName
			}
			students = append(students, student)
			childrenBySession[row.ActiveGroupID]++
		}
		ids := make([]string, 0, len(sessions))
		entries := make([]OpenRoomSession, 0, len(sessions))
		supervising := false
		occupying := false
		for _, session := range sessions {
			ids = append(ids, strconv.FormatInt(session.ActiveGroupID, 10))
			if !session.IndependentStays {
				occupying = true
			}
			entry := openRoomSession(session, in.blocks, staffID, adminScope)
			entry.StudentCount = childrenBySession[session.ActiveGroupID]
			supervising = supervising || entry.IsUserSupervising
			entries = append(entries, entry)
		}
		result = append(result, OpenRoom{
			RoomID: room.ID, Name: room.Name, IsUserSupervising: supervising,
			ActiveGroupIDs: ids, HasOccupyingSession: occupying,
			StudentCount: len(students), Students: students,
			Sessions: entries,
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

// openRoomSession projects one running session for its section on the room
// page: its name, the caller's relation to it, and the block behind it.
func openRoomSession(session RunningSession, blocks map[int64]SessionBlock, staffID *int64, adminScope bool) OpenRoomSession {
	supervising := staffID != nil && slices.Contains(session.SupervisorStaffIDs, *staffID)
	entry := OpenRoomSession{
		ActiveGroupID:     session.ActiveGroupID,
		Independent:       session.IndependentStays,
		IsUserSupervising: supervising,
		CanAssign:         adminScope || supervising,
	}
	if !session.IndependentStays {
		entry.Title = session.ActivityName
	}
	if block, found := blocks[session.ActiveGroupID]; found {
		entry.Title = block.Title
		entry.Block = &OpenRoomBlock{
			InstanceID:     block.InstanceID,
			StartTime:      block.StartTime,
			EndTime:        block.EndTime,
			IsUserAssigned: block.IsAssigned,
			CanOperate:     block.CanOperate,
		}
	}
	return entry
}
