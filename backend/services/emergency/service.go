package emergency

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/collation"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

const presenceModeBinary = "binary"

// healthInfoMissing is what a child WITHOUT a stored health note prints as.
// An empty cell reads as "no allergies" to whoever grabs the sheet, which is
// the one reading that could get a child hurt — so the absence of data says
// so in words (#2609).
const healthInfoMissing = "Nicht hinterlegt"

var healthInfoStyleMarkers = strings.NewReplacer("\x01", "", "\x02", "", "\x03", "")

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

// settingsReader is the narrow slice of the settings service this package
// needs: one boolean, resolved for the request's tenant.
type settingsReader interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
}

type Dependencies struct {
	AttendanceRepo      attendanceReader
	StudentRepo         studentReader
	PersonRepo          personReader
	VisitRepo           visitLocationReader
	StudentGuardianRepo guardianContactReader
	ActiveService       activePresenceReader
	ListExport          *listexport.RendererService
	// Settings decides whether the health column is printed
	// (operations.emergency_list_health_info). Optional: a nil service means
	// no school-level opinion is available, and the column is left off — see
	// healthInfoEnabled.
	Settings settingsReader
	Logger   *slog.Logger
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
	HealthInfo      string
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

	withHealth := s.healthInfoEnabled(ctx)

	columns := []listexport.Column{
		{ID: listexport.ColumnName, Label: "Name"},
		{ID: listexport.ColumnSchoolClass, Label: "Klasse"},
		{ID: listexport.ColumnCurrentLocation, Label: "Ort / Raum"},
		{ID: listexport.ColumnContactPhone, Label: "Telefonnummer"},
		{ID: listexport.ColumnContactName, Label: "Kontakt"},
	}
	if withHealth {
		columns = append(columns, listexport.Column{ID: listexport.ColumnHealthInfo, Label: "Gesundheit / Allergien"})
	}

	return listexport.Document{
		Title:       "Notfallliste",
		Subtitle:    fmt.Sprintf("%d anwesende Kinder", len(rows)),
		GeneratedAt: generatedAt,
		Columns:     columns,
		Rows:        buildDocumentRows(rows, withHealth),
	}, nil
}

// healthInfoEnabled reports whether the school prints health notes on the
// Notfallliste. It is NOT a read gate — the note is already visible to every
// account with users:read in the child's record, and this export requires the
// same permission; the switch only decides what lands on the paper.
//
// It errs towards leaving the column OFF when the setting cannot be read: a
// school that switched it off did so for a data-protection reason, and a
// column that appears because a lookup failed would break that silently. The
// rest of the list (names, location, phone numbers) is unaffected, so the
// sheet still does its job.
func (s *Service) healthInfoEnabled(ctx context.Context) bool {
	if s.Settings == nil {
		return false
	}
	enabled, err := s.Settings.ResolveBool(ctx, configModel.KeyEmergencyListHealthInfo)
	if err != nil {
		s.logger().WarnContext(ctx, "emergency list: health info setting could not be resolved, printing list without health column",
			slog.String("key", configModel.KeyEmergencyListHealthInfo),
			slog.String("error", err.Error()),
		)
		return false
	}
	return enabled
}

func (s *Service) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
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
			HealthInfo:      strings.TrimSpace(base.Deref(student.HealthInfo)),
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

func buildDocumentRows(rows []snapshotRow, withHealth bool) []listexport.Row {
	result := make([]listexport.Row, 0, len(rows))
	for _, row := range rows {
		values := map[listexport.ColumnID]string{
			listexport.ColumnName:            row.Name,
			listexport.ColumnSchoolClass:     row.SchoolClass,
			listexport.ColumnCurrentLocation: row.Location,
			listexport.ColumnContactPhone:    row.ContactPhone,
			listexport.ColumnContactName:     row.ContactName,
		}
		if withHealth {
			values[listexport.ColumnHealthInfo] = healthInfoCell(row.HealthInfo)
		}
		result = append(result, listexport.Row{Values: values})
	}
	return result
}

// healthInfoCell renders one child's health note. Whitespace-only notes count
// as no note: "   " on screen and "" on paper mean the same thing to a reader,
// and both must say "Nicht hinterlegt" rather than leave a blank that reads as
// an all-clear.
func healthInfoCell(note string) string {
	if strings.TrimSpace(note) == "" {
		return healthInfoMissing
	}

	// listexport stores line styling inside cell text using these C0 control
	// markers. Health notes are user-controlled free text, so passing a marker
	// through would make the renderer interpret part of the note as styling.
	return healthInfoStyleMarkers.Replace(note)
}
