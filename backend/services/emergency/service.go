package emergency

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	repoBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const presenceModeBinary = "binary"

type Service interface {
	RenderSnapshot(ctx context.Context) (listexport.File, error)
	BuildSnapshotDocument(ctx context.Context, generatedAt time.Time) (listexport.Document, error)
}

type attendanceReader interface {
	ListOpenStudentIDsForDate(ctx context.Context, date time.Time) ([]int64, error)
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

type Dependencies struct {
	AttendanceRepo attendanceReader
	StudentRepo    studentReader
	PersonRepo     personReader
	ActiveService  activePresenceReader
	ListExport     listexport.Service
	DB             *bun.DB
}

type service struct {
	attendanceRepo attendanceReader
	studentRepo    studentReader
	personRepo     personReader
	activeService  activePresenceReader
	listExport     listexport.Service
	db             *bun.DB
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

type guardianContactRow struct {
	StudentID   int64          `bun:"student_id"`
	FirstName   sql.NullString `bun:"first_name"`
	LastName    sql.NullString `bun:"last_name"`
	PhoneNumber sql.NullString `bun:"phone_number"`
}

func NewService(deps Dependencies) Service {
	return &service{
		attendanceRepo: deps.AttendanceRepo,
		studentRepo:    deps.StudentRepo,
		personRepo:     deps.PersonRepo,
		activeService:  deps.ActiveService,
		listExport:     deps.ListExport,
		db:             deps.DB,
	}
}

func (s *service) RenderSnapshot(ctx context.Context) (listexport.File, error) {
	doc, err := s.BuildSnapshotDocument(ctx, time.Now())
	if err != nil {
		return listexport.File{}, err
	}
	return s.listExport.Render(doc, listexport.FormatPDF, "notfallliste")
}

func (s *service) BuildSnapshotDocument(ctx context.Context, generatedAt time.Time) (listexport.Document, error) {
	if s.attendanceRepo == nil || s.studentRepo == nil || s.personRepo == nil || s.listExport == nil || s.db == nil {
		return listexport.Document{}, fmt.Errorf("emergency snapshot service is not configured")
	}

	studentIDs, err := s.attendanceRepo.ListOpenStudentIDsForDate(ctx, timezone.TodayUTC())
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

func (s *service) loadSnapshotRows(ctx context.Context, studentIDs []int64) ([]snapshotRow, error) {
	if len(studentIDs) == 0 {
		return []snapshotRow{}, nil
	}

	students, err := s.studentRepo.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}

	personIDs := make([]int64, 0, len(students))
	for _, student := range students {
		personIDs = append(personIDs, student.PersonID)
	}

	persons, err := s.personRepo.FindByIDs(ctx, personIDs)
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
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Location != rows[j].Location {
			return rows[i].Location < rows[j].Location
		}
		return rows[i].Name < rows[j].Name
	})
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
			GuardianName:    ptrString(student.GuardianName),
			GuardianContact: ptrString(student.GuardianContact),
			GuardianPhone:   ptrString(student.GuardianPhone),
		}
		if row.Location == "" {
			row.Location = "Unterwegs"
		}
		if contact, ok := contacts[id]; ok {
			row.ContactName = contact.Name
			row.ContactPhone = contact.Phone
		}
		row.ContactName = joinUnique(row.ContactName, row.GuardianName)
		row.ContactPhone = joinUnique(row.ContactPhone, row.GuardianPhone, row.GuardianContact)
		rows = append(rows, row)
	}
	return rows
}

func (s *service) loadCurrentLocations(ctx context.Context, studentIDs []int64) (map[int64]string, error) {
	if s.activeService != nil && s.activeService.GetPresenceMode(ctx) == presenceModeBinary {
		return s.loadBinaryLocations(ctx, studentIDs)
	}

	type currentLocationRow struct {
		StudentID int64          `bun:"student_id"`
		RoomName  sql.NullString `bun:"room_name"`
	}

	var rows []currentLocationRow
	query := repoBase.GetDB(ctx, s.db).NewSelect().
		TableExpr(`active.visits AS "visit"`).
		ColumnExpr(`DISTINCT ON ("visit".student_id) "visit".student_id`).
		ColumnExpr(`"room".name AS "room_name"`).
		Join(`JOIN active.groups AS "group" ON "group".id = "visit".active_group_id AND "group".end_time IS NULL`).
		Join(`JOIN facilities.rooms AS "room" ON "room".id = "group".room_id`).
		Where(`"visit".student_id IN (?)`, bun.List(studentIDs)).
		Where(`"visit".exit_time IS NULL`).
		OrderExpr(`"visit".student_id ASC`).
		OrderExpr(`"visit".entry_time DESC`)

	if where, val, ok := repoBase.TenantWhere(ctx, "visit"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, err
	}

	locations := make(map[int64]string, len(rows))
	for _, row := range rows {
		if row.RoomName.Valid {
			locations[row.StudentID] = row.RoomName.String
		}
	}
	return locations, nil
}

func (s *service) loadBinaryLocations(ctx context.Context, studentIDs []int64) (map[int64]string, error) {
	statuses, err := s.activeService.GetStudentsAttendanceStatuses(ctx, studentIDs)
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

func (s *service) loadGuardianContacts(ctx context.Context, studentIDs []int64) (map[int64]guardianContact, error) {
	var rows []guardianContactRow
	query := repoBase.GetDB(ctx, s.db).NewSelect().
		TableExpr(`users.students_guardians AS "student_guardian"`).
		ColumnExpr(`"student_guardian".student_id`).
		ColumnExpr(`"guardian".first_name`).
		ColumnExpr(`"guardian".last_name`).
		ColumnExpr(`"phone".phone_number`).
		Join(`JOIN users.guardian_profiles AS "guardian" ON "guardian".id = "student_guardian".guardian_profile_id`).
		Join(`LEFT JOIN users.guardian_phone_numbers AS "phone" ON "phone".guardian_profile_id = "guardian".id`).
		Where(`"student_guardian".student_id IN (?)`, bun.List(studentIDs)).
		OrderExpr(`"student_guardian".is_emergency_contact DESC`).
		OrderExpr(`"student_guardian".is_primary DESC`).
		OrderExpr(`"student_guardian".emergency_priority ASC`).
		OrderExpr(`"phone".is_primary DESC NULLS LAST`).
		OrderExpr(`"phone".priority ASC NULLS LAST`).
		OrderExpr(`"student_guardian".student_id ASC`).
		OrderExpr(`"guardian".id ASC`)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where(`"student_guardian".tenant_id = ?`, tenantID)
		query = query.Where(`"guardian".tenant_id = ?`, tenantID)
		query = query.Where(`("phone".tenant_id = ? OR "phone".tenant_id IS NULL)`, tenantID)
	}

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, err
	}

	contacts := make(map[int64]guardianContact, len(rows))
	for _, row := range rows {
		current := contacts[row.StudentID]
		current.Name = joinUnique(current.Name, strings.TrimSpace(row.FirstName.String+" "+row.LastName.String))
		current.Phone = joinUnique(current.Phone, row.PhoneNumber.String)
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

func joinUnique(values ...string) string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ";") {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			key := strings.ToLower(trimmed)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, trimmed)
		}
	}
	return strings.Join(result, "; ")
}

func ptrString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
