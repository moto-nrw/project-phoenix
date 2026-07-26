package emergency

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/collation"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

const presenceModeBinary = "binary"

type attendanceReader interface {
	ListOpenStudentIDsForDate(ctx context.Context, date timezone.Date) ([]int64, error)
}

type studentReader interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*userModel.Student, error)
}

type personReader interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*userModel.Person, error)
}

type activePresenceReader interface {
	GetPresenceMode(ctx context.Context) string
	GetStudentsAttendanceStatuses(ctx context.Context, studentIDs []int64) (map[int64]*activeService.AttendanceStatus, error)
}

type visitLocationReader interface {
	GetCurrentRoomNamesForStudents(ctx context.Context, studentIDs []int64) (map[int64]string, error)
}

type guardianContactReader interface {
	ListEmergencyContactRows(ctx context.Context, studentIDs []int64) ([]userModel.GuardianEmergencyContactRow, error)
}

type Dependencies struct {
	AttendanceRepo      attendanceReader
	StudentRepo         studentReader
	PersonRepo          personReader
	VisitRepo           visitLocationReader
	StudentGuardianRepo guardianContactReader
	ActiveService       activePresenceReader
	ListExport          *listexport.RendererService
}

// Service renders the emergency list ("Notfallliste") snapshot: every
// currently checked-in student with location and emergency contacts.
type Service struct {
	Dependencies
}

type snapshotRow struct {
	StudentID       int64
	Name            string
	SchoolClass     string
	Location        string
	ContactName     string
	ContactPhone    string
	GuardianName    string
	GuardianContact string
	GuardianPhone   string
}

func NewService(deps Dependencies) *Service {
	return &Service{Dependencies: deps}
}

func (s *Service) RenderSnapshot(ctx context.Context) (listexport.File, error) {
	doc, err := s.BuildSnapshotDocument(ctx, time.Now())
	if err != nil {
		return listexport.File{}, err
	}
	return s.ListExport.Render(doc, listexport.FormatPDF, "notfallliste")
}

func (s *Service) BuildSnapshotDocument(ctx context.Context, generatedAt time.Time) (listexport.Document, error) {
	if s.AttendanceRepo == nil || s.StudentRepo == nil || s.PersonRepo == nil || s.ListExport == nil || s.VisitRepo == nil || s.StudentGuardianRepo == nil {
		return listexport.Document{}, fmt.Errorf("emergency snapshot service is not configured")
	}

	studentIDs, err := s.AttendanceRepo.ListOpenStudentIDsForDate(ctx, timezone.TodayDate())
	if err != nil {
		return listexport.Document{}, err
	}

	rows, err := s.loadSnapshotRows(ctx, studentIDs)
	if err != nil {
		return listexport.Document{}, err
	}

	return listexport.Document{
		Title:       "Notfallliste",
		Subtitle:    fmt.Sprintf("%d anwesende Kinder", len(rows)),
		GeneratedAt: generatedAt,
		Columns: []listexport.Column{
			{ID: listexport.ColumnName, Label: "Name"},
			{ID: listexport.ColumnSchoolClass, Label: "Klasse"},
			{ID: listexport.ColumnCurrentLocation, Label: "Ort / Raum"},
			{ID: listexport.ColumnContactPhone, Label: "Telefonnummer"},
			{ID: listexport.ColumnContactName, Label: "Kontakt"},
		},
		Rows: buildDocumentRows(rows),
	}, nil
}

func (s *Service) loadSnapshotRows(ctx context.Context, studentIDs []int64) ([]snapshotRow, error) {
	if len(studentIDs) == 0 {
		return []snapshotRow{}, nil
	}

	students, err := s.StudentRepo.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}

	personIDs := make([]int64, 0, len(students))
	for _, student := range students {
		personIDs = append(personIDs, student.PersonID)
	}

	persons, err := s.PersonRepo.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}

	locations, err := s.loadCurrentLocations(ctx, studentIDs)
	if err != nil {
		return nil, err
	}

	contacts, err := s.loadGuardianContacts(ctx, studentIDs)
	if err != nil {
		return nil, err
	}

	rows := buildSnapshotRows(studentIDs, students, persons, locations, contacts)
	sortSnapshotRows(rows)
	return rows, nil
}

func buildSnapshotRows(
	studentIDs []int64,
	students map[int64]*userModel.Student,
	persons map[int64]*userModel.Person,
	locations map[int64]string,
	contacts map[int64]guardianContact,
) []snapshotRow {
	rows := make([]snapshotRow, 0, len(studentIDs))
	for _, id := range studentIDs {
		student := students[id]
		if student == nil {
			continue
		}
		person := persons[student.PersonID]
		if person == nil {
			continue
		}
		row := snapshotRow{
			StudentID:       id,
			Name:            person.GetFullName(),
			SchoolClass:     student.SchoolClass,
			Location:        locations[id],
			GuardianName:    base.Deref(student.GuardianName),
			GuardianContact: base.Deref(student.GuardianContact),
			GuardianPhone:   base.Deref(student.GuardianPhone),
		}
		if row.Location == "" {
			row.Location = "Unterwegs"
		}
		if contact, ok := contacts[id]; ok {
			row.ContactName = contact.Name
			row.ContactPhone = contact.Phone
		}
		row.ContactName = strutil.JoinUnique(row.ContactName, row.GuardianName)
		row.ContactPhone = strutil.JoinUnique(row.ContactPhone, row.GuardianPhone, row.GuardianContact)
		rows = append(rows, row)
	}
	return rows
}

func sortSnapshotRows(rows []snapshotRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Location != rows[j].Location {
			return rows[i].Location < rows[j].Location
		}
		return collation.CompareGerman(rows[i].Name, rows[j].Name) < 0
	})
}

func (s *Service) loadCurrentLocations(ctx context.Context, studentIDs []int64) (map[int64]string, error) {
	if s.ActiveService != nil && s.ActiveService.GetPresenceMode(ctx) == presenceModeBinary {
		return s.loadBinaryLocations(ctx, studentIDs)
	}

	return s.VisitRepo.GetCurrentRoomNamesForStudents(ctx, studentIDs)
}

func (s *Service) loadBinaryLocations(ctx context.Context, studentIDs []int64) (map[int64]string, error) {
	statuses, err := s.ActiveService.GetStudentsAttendanceStatuses(ctx, studentIDs)
	if err != nil {
		return nil, err
	}

	locations := make(map[int64]string, len(studentIDs))
	for _, id := range studentIDs {
		locations[id] = binaryLocationLabel(statuses[id])
	}
	return locations, nil
}

func binaryLocationLabel(status *activeService.AttendanceStatus) string {
	if status == nil {
		return "Abwesend"
	}
	switch status.Status {
	case "checked_in":
		return "Anwesend"
	case "on_yard":
		return "Schulhof"
	default:
		return "Abwesend"
	}
}

type guardianContact struct {
	Name  string
	Phone string
}

func (s *Service) loadGuardianContacts(ctx context.Context, studentIDs []int64) (map[int64]guardianContact, error) {
	rows, err := s.StudentGuardianRepo.ListEmergencyContactRows(ctx, studentIDs)
	if err != nil {
		return nil, err
	}

	contacts := make(map[int64]guardianContact, len(rows))
	for _, row := range rows {
		current := contacts[row.StudentID]
		current.Name = strutil.JoinUnique(current.Name, strings.TrimSpace(row.FirstName.String+" "+row.LastName.String))
		current.Phone = strutil.JoinUnique(current.Phone, row.PhoneNumber.String)
		contacts[row.StudentID] = current
	}
	return contacts, nil
}

func buildDocumentRows(rows []snapshotRow) []listexport.Row {
	result := make([]listexport.Row, 0, len(rows))
	for _, row := range rows {
		result = append(result, listexport.Row{Values: map[listexport.ColumnID]string{
			listexport.ColumnName:            row.Name,
			listexport.ColumnSchoolClass:     row.SchoolClass,
			listexport.ColumnCurrentLocation: row.Location,
			listexport.ColumnContactPhone:    row.ContactPhone,
			listexport.ColumnContactName:     row.ContactName,
		}})
	}
	return result
}
