package httpadapter

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/collation"
	roomsHTTP "github.com/moto-nrw/project-phoenix/modules/facilities/http/rooms"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

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

func exportRoomSnapshot(ctx context.Context, dependencies Dependencies, request roomsHTTP.SnapshotRequest) (roomsHTTP.ExportFile, error) {
	locations, err := loadRoomSnapshotLocations(ctx, dependencies, request)
	if err != nil {
		return roomsHTTP.ExportFile{}, err
	}
	students, err := loadRoomSnapshotStudents(ctx, dependencies, collectRoomSnapshotStudentIDs(locations))
	if err != nil {
		return roomsHTTP.ExportFile{}, err
	}
	document := listexport.Document{
		Title:       roomSnapshotTitle(request.Title),
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
		Rows: buildRoomSnapshotRows(locations, students),
	}
	file, err := dependencies.ListExport.Render(document, listexport.Format(request.Format), document.Title)
	if err != nil {
		return roomsHTTP.ExportFile{}, &roomsHTTP.InvalidExportError{Err: err}
	}
	return roomsHTTP.ExportFile{Data: file.Data, ContentType: file.ContentType, Filename: file.Filename}, nil
}

func loadRoomSnapshotLocations(ctx context.Context, dependencies Dependencies, request roomsHTTP.SnapshotRequest) ([]roomSnapshotLocation, error) {
	rooms, err := dependencies.Facilities.ListRooms(ctx)
	if err != nil {
		return nil, err
	}
	selected := selectedRoomIDSet(request.RoomIDs)
	locations := make([]roomSnapshotLocation, 0, len(rooms)+1)
	for _, room := range rooms {
		if selected != nil {
			if _, found := selected[room.ID]; !found {
				continue
			}
		}
		studentIDs, err := dependencies.Active.ListStudentsPresentInRoom(ctx, room.ID)
		if err != nil {
			return nil, err
		}
		locations = append(locations, roomSnapshotLocation{
			Name: room.Name, Status: occupiedLabel(room.IsOccupied), Building: room.Building,
			Floor: formatSnapshotFloor(room.Floor), Activity: dereference(room.GroupName),
			Supervision: dereference(room.SupervisorNames), ChildCount: len(studentIDs), StudentIDs: studentIDs,
		})
	}
	if request.IncludeTransit {
		studentIDs, err := dependencies.Active.ListStudentsInTransit(ctx)
		if err != nil {
			return nil, err
		}
		locations = append(locations, roomSnapshotLocation{
			Name: "Unterwegs", Status: "Unterwegs", ChildCount: len(studentIDs), StudentIDs: studentIDs, IsTransit: true,
		})
	}
	return locations, nil
}

func loadRoomSnapshotStudents(ctx context.Context, dependencies Dependencies, studentIDs []int64) (map[int64]roomSnapshotStudent, error) {
	result := make(map[int64]roomSnapshotStudent, len(studentIDs))
	if len(studentIDs) == 0 {
		return result, nil
	}
	students, err := dependencies.Users.GetStudentsByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	personIDs, groupIDs := make([]int64, 0, len(students)), make([]int64, 0, len(students))
	for _, student := range students {
		personIDs = appendUniqueInt64(personIDs, student.PersonID)
		if student.GroupID != nil {
			groupIDs = appendUniqueInt64(groupIDs, *student.GroupID)
		}
	}
	persons, err := dependencies.Users.GetByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	groups, err := dependencies.Education.GetGroupsByIDs(ctx, groupIDs)
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
		name := fmt.Sprintf("Kind %d", student.ID)
		if person := persons[student.PersonID]; person != nil {
			name = strings.TrimSpace(person.FirstName + " " + person.LastName)
		}
		result[id] = roomSnapshotStudent{ID: id, Name: name, SchoolClass: student.SchoolClass, GroupName: groupName}
	}
	return result, nil
}

func dereference(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func selectedRoomIDSet(ids *[]int64) map[int64]struct{} {
	if ids == nil {
		return nil
	}
	result := make(map[int64]struct{}, len(*ids))
	for _, id := range *ids {
		result[id] = struct{}{}
	}
	return result
}

func collectRoomSnapshotStudentIDs(locations []roomSnapshotLocation) []int64 {
	var result []int64
	for _, location := range locations {
		for _, id := range location.StudentIDs {
			result = appendUniqueInt64(result, id)
		}
	}
	return result
}

func buildRoomSnapshotRows(locations []roomSnapshotLocation, studentsByID map[int64]roomSnapshotStudent) []listexport.Row {
	rows := make([]listexport.Row, 0, len(locations))
	for _, location := range locations {
		students := make([]roomSnapshotStudent, 0, len(location.StudentIDs))
		for _, id := range location.StudentIDs {
			if student, found := studentsByID[id]; found {
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
		listexport.ColumnRoomName: location.Name, listexport.ColumnRoomStatus: location.Status,
		listexport.ColumnRoomBuilding: location.Building, listexport.ColumnRoomFloor: location.Floor,
		listexport.ColumnRoomActivity: location.Activity, listexport.ColumnRoomSupervision: location.Supervision,
		listexport.ColumnRoomChildCount: strconv.Itoa(location.ChildCount),
	}}
}

func roomSnapshotStudentRow(location roomSnapshotLocation, student roomSnapshotStudent) listexport.Row {
	row := roomSnapshotLocationRow(location)
	row.Values[listexport.ColumnChecklist] = ""
	row.Values[listexport.ColumnStudentName] = student.Name
	row.Values[listexport.ColumnStudentClass] = student.SchoolClass
	row.Values[listexport.ColumnStudentGroup] = student.GroupName
	return row
}

func roomSnapshotTitle(title string) string {
	if title = strings.TrimSpace(title); title != "" {
		return title
	}
	return "Wer ist wo"
}

func roomSnapshotSubtitle(locations []roomSnapshotLocation) string {
	rooms, children := 0, 0
	transit := false
	for _, location := range locations {
		children += location.ChildCount
		if location.IsTransit {
			transit = true
		} else {
			rooms++
		}
	}
	suffix := ""
	if transit {
		suffix = " plus Unterwegs"
	}
	return fmt.Sprintf("%d Räume%s - %d Kinder", rooms, suffix, children)
}

func occupiedLabel(occupied bool) string {
	if occupied {
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

func appendUniqueInt64(values []int64, next int64) []int64 {
	for _, value := range values {
		if value == next {
			return values
		}
	}
	return append(values, next)
}
