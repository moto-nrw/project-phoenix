package supervisiondashboard

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type openRoomInputs struct {
	rooms        []ReleasedRoom
	sessions     []RunningSession
	sessionByID  map[int64]RunningSession
	visitsByRoom map[int64][]VisitRecord
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

func (s *service) loadOpenRoomInputs(ctx context.Context) (openRoomInputs, error) {
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
	return openRoomInputs{
		rooms: rooms, sessions: sessions, sessionByID: sessionByID,
		visitsByRoom: groupOpenRoomVisits(sessionByID, visits),
	}, nil
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

func (s *service) assembleOpenRooms(in openRoomInputs, attendance map[int64]Attendance, fullAccess, photosEnabled bool, staffID *int64) []OpenRoom {
	sessionsByRoom := make(map[int64][]RunningSession, len(in.rooms))
	for _, session := range in.sessions {
		sessionsByRoom[session.RoomID] = append(sessionsByRoom[session.RoomID], session)
	}
	result := make([]OpenRoom, 0, len(in.rooms))
	for _, room := range in.rooms {
		sessions := sessionsByRoom[room.ID]
		sort.Slice(sessions, func(i, j int) bool { return sessions[i].StartTime.Before(sessions[j].StartTime) })
		ids := make([]string, 0, len(sessions))
		supervising := false
		for _, session := range sessions {
			ids = append(ids, strconv.FormatInt(session.ActiveGroupID, 10))
			for _, supervisorID := range session.SupervisorStaffIDs {
				if staffID != nil && supervisorID == *staffID {
					supervising = true
				}
			}
		}
		rows := in.visitsByRoom[room.ID]
		students := make([]OpenRoomStudent, 0, len(rows))
		for _, row := range rows {
			students = append(students, OpenRoomStudent{
				Visit:        s.buildVisit(row, attendance, fullAccess, photosEnabled),
				ActivityName: in.sessionByID[row.ActiveGroupID].ActivityName,
			})
		}
		result = append(result, OpenRoom{
			RoomID: room.ID, Name: room.Name, IsUserSupervising: supervising,
			ActiveGroupIDs: ids, StudentCount: len(students), Students: students,
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
