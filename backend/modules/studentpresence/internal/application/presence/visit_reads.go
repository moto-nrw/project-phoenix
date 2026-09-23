package presence

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

func (s *service) FindVisitsByStudentID(ctx context.Context, studentID int64) ([]studentpresence.Visit, error) {
	visits, err := s.SchoolPresence.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{studentID}, OpenOnly: true})
	if err != nil {
		return nil, &ActiveError{Op: "FindVisitsByStudentID", Err: ErrDatabaseOperation}
	}
	return visits, nil
}

func (s *service) GetStudentCurrentVisit(ctx context.Context, studentID int64) (*studentpresence.Visit, error) {
	visits, err := s.SchoolPresence.ListVisits(ctx, studentpresence.VisitFilter{
		StudentIDs: []int64{studentID}, OpenOnly: true, NewestFirst: true, Limit: 1,
	})
	if err != nil {
		return nil, &ActiveError{Op: "GetStudentCurrentVisit", Err: ErrDatabaseOperation}
	}
	if len(visits) == 0 {
		return nil, &ActiveError{Op: "GetStudentCurrentVisit", Err: ErrVisitNotFound}
	}
	return &visits[0], nil
}

func (s *service) GetStudentCurrentVisitWithRoom(ctx context.Context, studentID int64) (*VisitWithRoom, error) {
	locations, err := s.SchoolPresence.ListVisitLocations(ctx, studentpresence.VisitLocationFilter{VisitFilter: studentpresence.VisitFilter{
		StudentIDs: []int64{studentID}, OpenOnly: true, NewestFirst: true, Limit: 1,
	}})
	if err != nil {
		return nil, &ActiveError{Op: "GetStudentCurrentVisitWithRoom", Err: ErrDatabaseOperation}
	}
	if len(locations) == 0 {
		return nil, &ActiveError{Op: "GetStudentCurrentVisitWithRoom", Err: ErrVisitNotFound}
	}
	visit := &VisitWithRoom{Visit: locations[0].Visit}
	group := locations[0].Group
	if group == nil {
		return visit, nil
	}
	visit.ActiveGroup = &VisitRoomGroup{
		ID: group.ID, CreatedAt: group.CreatedAt, UpdatedAt: group.UpdatedAt,
		RoomID: group.RoomID, StartTime: group.StartTime, EndTime: group.EndTime,
		LastActivity: group.LastActivity, GroupID: group.TemplateID, DeviceID: group.DeviceID, TimeoutMinutes: group.TimeoutMinutes,
	}
	rooms, err := s.RoomRepo.FindByIDs(ctx, []int64{group.RoomID})
	if err != nil {
		return nil, &ActiveError{Op: "GetStudentCurrentVisitWithRoom", Err: ErrDatabaseOperation}
	}
	for _, room := range rooms {
		if room.ID == group.RoomID {
			visit.ActiveGroup.Room = &VisitRoom{ID: room.ID, CreatedAt: room.CreatedAt, UpdatedAt: room.UpdatedAt, Name: room.Name}
			break
		}
	}
	return visit, nil
}

func (s *service) GetStudentsCurrentVisits(ctx context.Context, studentIDs []int64) (map[int64]*studentpresence.Visit, error) {
	visits, err := s.currentPresenceVisits(ctx, studentIDs, false)
	if err != nil {
		return nil, &ActiveError{Op: "GetStudentsCurrentVisits", Err: ErrDatabaseOperation}
	}
	return visits, nil
}

func (s *service) CountActiveVisitsByActiveGroupID(ctx context.Context, activeGroupID int64) (int, error) {
	count, err := s.SchoolPresence.CountOpenVisitsInGroup(ctx, activeGroupID)
	if err != nil {
		return 0, &ActiveError{Op: "CountActiveVisitsByActiveGroupID", Err: ErrDatabaseOperation}
	}
	return count, nil
}

// ListStudentsPresentInRoom returns the IDs of students currently checked-in
// to any active group in the given room. The student list handler feeds
// these IDs through the standard ListWithOptions pipeline, which applies
// GDPR redaction and pagination.
func (s *service) ListStudentsPresentInRoom(ctx context.Context, roomID int64) ([]int64, error) {
	if roomID <= 0 {
		return []int64{}, nil
	}
	rows, err := s.SchoolPresence.ListOpenVisitRooms(ctx, roomID)
	if err != nil {
		return nil, &ActiveError{Op: "ListStudentsPresentInRoom", Err: fmt.Errorf("list active student IDs: %w", err)}
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.StudentID)
	}
	return ids, nil
}

// ListOpenVisitStudentIDsByRoom returns the current tenant's room presence in
// one read for consumers rendering several rooms at once.
func (s *service) ListOpenVisitStudentIDsByRoom(ctx context.Context) (map[int64][]int64, error) {
	rows, err := s.SchoolPresence.ListOpenVisitRooms(ctx, 0)
	if err != nil {
		return nil, &ActiveError{Op: "ListOpenVisitStudentIDsByRoom", Err: fmt.Errorf("list open visits by room: %w", err)}
	}
	rooms := make(map[int64][]int64)
	for _, row := range rows {
		rooms[row.RoomID] = append(rooms[row.RoomID], row.StudentID)
	}
	return rooms, nil
}

// GetTrackingIndicators returns per-student match results for the given labels.
// For each student, it checks today's visits and matches activity group name + room name
// against each label using case-insensitive substring matching.
func (s *service) GetTrackingIndicators(ctx context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error) {
	result := make(map[int64][]bool, len(studentIDs))
	if len(studentIDs) == 0 || len(labels) == 0 {
		return result, nil
	}

	visitNames, err := s.todayVisitNames(ctx, studentIDs)
	if err != nil {
		return nil, &ActiveError{Op: "GetTrackingIndicators", Err: ErrDatabaseOperation}
	}

	// Build a map of student ID → concatenated visit texts for matching.
	studentVisitTexts := make(map[int64][]string, len(studentIDs))
	for _, vn := range visitNames {
		text := strings.ToLower(strings.TrimSpace(vn.ActivityGroupName + " " + vn.RoomName))
		studentVisitTexts[vn.StudentID] = append(studentVisitTexts[vn.StudentID], text)
	}

	// Lowercase the labels once.
	lowerLabels := make([]string, len(labels))
	for i, l := range labels {
		lowerLabels[i] = strings.ToLower(strings.TrimSpace(l))
	}

	// For each student, check each label against their visit texts.
	for _, sid := range studentIDs {
		matches := make([]bool, len(labels))
		texts := studentVisitTexts[sid]
		for li, ll := range lowerLabels {
			for _, t := range texts {
				if strings.Contains(t, ll) {
					matches[li] = true
					break
				}
			}
		}
		result[sid] = matches
	}

	return result, nil
}
