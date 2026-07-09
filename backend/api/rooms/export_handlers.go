package rooms

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/collation"
	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

type roomSnapshotExportRequest struct {
	Format         listexport.Format `json:"format"`
	Title          string            `json:"title"`
	RoomIDs        *[]int64          `json:"room_ids"`
	IncludeTransit bool              `json:"include_transit"`
}

type roomSnapshotLocation struct {
	Name        string
	Status      string
	Building    string
	Floor       string
	Activity    string
	Supervision string
	ChildCount  int
	StudentIDs  []int64
	IsTransit   bool
}

type roomSnapshotStudent struct {
	ID          int64
	Name        string
	SchoolClass string
	GroupName   string
}

func (rs *Resource) exportSnapshot(w http.ResponseWriter, r *http.Request) {
	if err := rs.ensureRoomExportDependencies(); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	req, err := decodeRoomSnapshotExportRequest(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	locations, err := rs.loadRoomSnapshotLocations(r, req)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	studentsByID, err := rs.loadRoomSnapshotStudents(r, collectRoomSnapshotStudentIDs(locations))
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	doc := listexport.Document{
		Title:       roomSnapshotTitle(req.Title),
		Subtitle:    roomSnapshotSubtitle(locations),
		GeneratedAt: time.Now(),
		Columns: []listexport.Column{
			{ID: listexport.ColumnRoomName, Label: "Raum"},
			{ID: listexport.ColumnRoomStatus, Label: "Status"},
			{ID: listexport.ColumnRoomBuilding, Label: "Gebäude"},
			{ID: listexport.ColumnRoomFloor, Label: "Etage"},
			{ID: listexport.ColumnRoomActivity, Label: "Aktivität"},
			{ID: listexport.ColumnRoomSupervision, Label: "Aufsicht"},
			{ID: listexport.ColumnRoomChildCount, Label: "Kinder"},
			{ID: listexport.ColumnStudentClass, Label: "Klasse"},
			{ID: listexport.ColumnStudentGroup, Label: "Aufsichtsgruppe"},
			{ID: listexport.ColumnStudentName, Label: "Kind"},
			{ID: listexport.ColumnChecklist, Label: "Abhaken"},
		},
		Rows: buildRoomSnapshotRows(locations, studentsByID),
	}

	file, err := rs.ListExportService.Render(doc, req.Format, doc.Title)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

func (rs *Resource) ensureRoomExportDependencies() error {
	if rs.ActiveService == nil {
		return errors.New("active service is not configured")
	}
	if rs.PersonService == nil {
		return errors.New("person service is not configured")
	}
	if rs.EducationService == nil {
		return errors.New("education service is not configured")
	}
	if rs.PersonService == nil {
		return errors.New("student repository is not configured")
	}
	if rs.ListExportService == nil {
		return errors.New("list export service is not configured")
	}
	return nil
}

func decodeRoomSnapshotExportRequest(r *http.Request) (roomSnapshotExportRequest, error) {
	var req roomSnapshotExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, err
	}
	if req.Format == "" {
		req.Format = listexport.FormatPDF
	}
	switch req.Format {
	case listexport.FormatPDF, listexport.FormatDOCX, listexport.FormatXLSX:
		return req, nil
	default:
		return req, fmt.Errorf("unsupported export format %q", req.Format)
	}
}

func (rs *Resource) loadRoomSnapshotLocations(r *http.Request, req roomSnapshotExportRequest) ([]roomSnapshotLocation, error) {
	rooms, err := rs.FacilityService.ListRooms(r.Context(), nil)
	if err != nil {
		return nil, err
	}

	selected := selectedRoomIDSet(req.RoomIDs)
	locations := make([]roomSnapshotLocation, 0, len(rooms)+1)
	for _, room := range rooms {
		if selected != nil {
			if _, ok := selected[room.ID]; !ok {
				continue
			}
		}
		studentIDs, err := rs.ActiveService.ListStudentsPresentInRoom(r.Context(), room.ID)
		if err != nil {
			return nil, err
		}
		locations = append(locations, roomSnapshotLocationFromRoom(room, studentIDs))
	}

	if req.IncludeTransit {
		studentIDs, err := rs.ActiveService.ListStudentsInTransit(r.Context())
		if err != nil {
			return nil, err
		}
		locations = append(locations, roomSnapshotLocation{
			Name:       "Unterwegs",
			Status:     "Unterwegs",
			ChildCount: len(studentIDs),
			StudentIDs: studentIDs,
			IsTransit:  true,
		})
	}

	return locations, nil
}

func roomSnapshotLocationFromRoom(room facilities.RoomWithOccupancy, studentIDs []int64) roomSnapshotLocation {
	return roomSnapshotLocation{
		Name:        room.Name,
		Status:      occupiedLabel(room.IsOccupied),
		Building:    room.Building,
		Floor:       formatSnapshotFloor(room.Floor),
		Activity:    base.Deref(room.GroupName),
		Supervision: base.Deref(room.SupervisorNames),
		ChildCount:  len(studentIDs),
		StudentIDs:  studentIDs,
	}
}

func selectedRoomIDSet(ids *[]int64) map[int64]struct{} {
	if ids == nil {
		return nil
	}
	set := make(map[int64]struct{}, len(*ids))
	for _, id := range *ids {
		set[id] = struct{}{}
	}
	return set
}

func (rs *Resource) loadRoomSnapshotStudents(r *http.Request, studentIDs []int64) (map[int64]roomSnapshotStudent, error) {
	result := make(map[int64]roomSnapshotStudent, len(studentIDs))
	if len(studentIDs) == 0 {
		return result, nil
	}

	students, err := rs.PersonService.GetStudentsByIDs(r.Context(), studentIDs)
	if err != nil {
		return nil, err
	}

	personIDs := make([]int64, 0, len(students))
	groupIDs := make([]int64, 0, len(students))
	for _, student := range students {
		personIDs = appendUniqueInt64(personIDs, student.PersonID)
		if student.GroupID != nil {
			groupIDs = appendUniqueInt64(groupIDs, *student.GroupID)
		}
	}

	persons, err := rs.PersonService.GetByIDs(r.Context(), personIDs)
	if err != nil {
		return nil, err
	}
	groups, err := rs.EducationService.GetGroupsByIDs(r.Context(), groupIDs)
	if err != nil {
		return nil, err
	}

	for id, student := range students {
		groupName := ""
		if student.GroupID != nil {
			if group := groups[*student.GroupID]; group != nil {
				groupName = group.Name
			}
		}
		result[id] = roomSnapshotStudent{
			ID:          id,
			Name:        snapshotStudentName(student, persons[student.PersonID]),
			SchoolClass: student.SchoolClass,
			GroupName:   groupName,
		}
	}
	return result, nil
}

func collectRoomSnapshotStudentIDs(locations []roomSnapshotLocation) []int64 {
	ids := make([]int64, 0)
	for _, location := range locations {
		for _, id := range location.StudentIDs {
			ids = appendUniqueInt64(ids, id)
		}
	}
	return ids
}

func buildRoomSnapshotRows(locations []roomSnapshotLocation, studentsByID map[int64]roomSnapshotStudent) []listexport.Row {
	rows := make([]listexport.Row, 0, len(locations))
	for _, location := range locations {
		students := make([]roomSnapshotStudent, 0, len(location.StudentIDs))
		for _, id := range location.StudentIDs {
			if student, ok := studentsByID[id]; ok {
				students = append(students, student)
			}
		}
		sort.SliceStable(students, func(i, j int) bool {
			return collation.CompareGerman(students[i].Name, students[j].Name) < 0
		})

		if len(students) == 0 {
			rows = append(rows, roomSnapshotLocationRow(location))
			continue
		}
		for _, student := range students {
			rows = append(rows, roomSnapshotStudentRow(location, student))
		}
	}
	return rows
}

func roomSnapshotLocationRow(location roomSnapshotLocation) listexport.Row {
	return listexport.Row{Values: map[listexport.ColumnID]string{
		listexport.ColumnRoomName:        location.Name,
		listexport.ColumnRoomStatus:      location.Status,
		listexport.ColumnRoomBuilding:    location.Building,
		listexport.ColumnRoomFloor:       location.Floor,
		listexport.ColumnRoomActivity:    location.Activity,
		listexport.ColumnRoomSupervision: location.Supervision,
		listexport.ColumnRoomChildCount:  strconv.Itoa(location.ChildCount),
	}}
}

func roomSnapshotStudentRow(location roomSnapshotLocation, student roomSnapshotStudent) listexport.Row {
	return listexport.Row{Values: map[listexport.ColumnID]string{
		listexport.ColumnRoomName:        location.Name,
		listexport.ColumnRoomStatus:      location.Status,
		listexport.ColumnRoomBuilding:    location.Building,
		listexport.ColumnRoomFloor:       location.Floor,
		listexport.ColumnRoomActivity:    location.Activity,
		listexport.ColumnRoomSupervision: location.Supervision,
		listexport.ColumnRoomChildCount:  strconv.Itoa(location.ChildCount),
		listexport.ColumnChecklist:       "",
		listexport.ColumnStudentName:     student.Name,
		listexport.ColumnStudentClass:    student.SchoolClass,
		listexport.ColumnStudentGroup:    student.GroupName,
	}}
}

func roomSnapshotTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "Wer ist wo"
	}
	return title
}

func roomSnapshotSubtitle(locations []roomSnapshotLocation) string {
	rooms := 0
	transit := false
	children := 0
	for _, location := range locations {
		children += location.ChildCount
		if location.IsTransit {
			transit = true
			continue
		}
		rooms++
	}
	suffix := ""
	if transit {
		suffix = " plus Unterwegs"
	}
	return fmt.Sprintf("%d Räume%s - %d Kinder", rooms, suffix, children)
}

func occupiedLabel(isOccupied bool) string {
	if isOccupied {
		return "Belegt"
	}
	return "Frei"
}

func formatSnapshotFloor(floor *int) string {
	if floor == nil {
		return ""
	}
	if *floor == 0 {
		return "Erdgeschoss"
	}
	return fmt.Sprintf("Etage %d", *floor)
}

func snapshotStudentName(student *userModels.Student, person *userModels.Person) string {
	if person == nil {
		return fmt.Sprintf("Kind %d", student.ID)
	}
	return strings.TrimSpace(person.FirstName + " " + person.LastName)
}

func appendUniqueInt64(values []int64, next int64) []int64 {
	for _, value := range values {
		if value == next {
			return values
		}
	}
	return append(values, next)
}
