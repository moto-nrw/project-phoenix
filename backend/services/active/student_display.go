package active

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

type StudentDisplayFacts struct {
	FirstName, LastName, OGSGroupName string
	ID, PersonID                      int64
	SchoolClass                       string
	GroupID                           *int64
	Sick, Excused                     *bool
	SickSince, ExcusedSince           *time.Time
	PhotoPath                         *string
}

type StudentDisplayReader interface {
	ListStudentDisplayFacts(context.Context, []int64) ([]StudentDisplayFacts, error)
}

// GetActiveGroupVisitsWithDisplay keeps visit ordering and drops students
// absent from the tenant-scoped directory, matching the original inner join.
func (s *service) GetActiveGroupVisitsWithDisplay(ctx context.Context, groupID int64) ([]*VisitWithStudentDisplay, error) {
	return s.GetActiveGroupVisitsWithDisplayForGroups(ctx, []int64{groupID})
}

// GetActiveGroupVisitsWithDisplayForGroups is the batch form: one visit query
// and one directory query regardless of how many sessions are asked for. A
// caller aggregating several rooms (the shared open-room view, #3065) would
// otherwise issue a pair of queries per session, which grows with the school's
// timetable rather than with its configuration.
func (s *service) GetActiveGroupVisitsWithDisplayForGroups(ctx context.Context, groupIDs []int64) ([]*VisitWithStudentDisplay, error) {
	if s.StudentDisplay == nil {
		return nil, errors.New("student display directory is required")
	}
	if len(groupIDs) == 0 {
		return nil, nil
	}
	visits, err := s.SchoolPresence.ListVisits(ctx, studentpresence.VisitFilter{ActiveGroupIDs: groupIDs, OpenOnly: true, NewestFirst: true})
	if err != nil {
		return nil, err
	}
	if len(visits) == 0 {
		return nil, nil
	}
	ids := make([]int64, 0, len(visits))
	for _, visit := range visits {
		ids = append(ids, visit.StudentID)
	}
	students, err := s.StudentDisplay.ListStudentDisplayFacts(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]StudentDisplayFacts, len(students))
	for _, student := range students {
		byID[student.ID] = student
	}
	result := make([]*VisitWithStudentDisplay, 0, len(visits))
	for _, visit := range visits {
		student, found := byID[visit.StudentID]
		if !found {
			continue
		}
		result = append(result, &VisitWithStudentDisplay{
			FirstName: student.FirstName, LastName: student.LastName, OGSGroupName: student.OGSGroupName,
			VisitID: visit.ID, StudentID: visit.StudentID, ActiveGroupID: visit.ActiveGroupID,
			EntryTime: visit.EntryTime, ExitTime: visit.ExitTime, CreatedAt: visit.CreatedAt, UpdatedAt: visit.UpdatedAt,
			PersonID: student.PersonID, SchoolClass: student.SchoolClass, GroupID: student.GroupID,
			Sick: student.Sick, SickSince: student.SickSince, Excused: student.Excused, ExcusedSince: student.ExcusedSince, PhotoPath: student.PhotoPath,
		})
	}
	return result, nil
}

// VisitWithStudentDisplay is the read model behind the active-group visit
// list: one open visit joined with the student's display data. Produced by
// the Student Presence facade and student display directory.
type VisitWithStudentDisplay struct {
	VisitID       int64
	StudentID     int64
	PersonID      int64
	ActiveGroupID int64
	EntryTime     time.Time
	ExitTime      *time.Time
	FirstName     string
	LastName      string
	SchoolClass   string
	GroupID       *int64 // student's education group_id (nullable)
	OGSGroupName  string
	Sick          *bool
	SickSince     *time.Time
	Excused       *bool
	ExcusedSince  *time.Time
	PhotoPath     *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
