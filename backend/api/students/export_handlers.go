package students

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
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/moto-nrw/project-phoenix/tenant"
)

const studentExportPageSize = 5000

type studentExportRequest struct {
	Format  listexport.Format     `json:"format"`
	Preset  listexport.Preset     `json:"preset"`
	Title   string                `json:"title"`
	Filters studentExportFilters  `json:"filters"`
	Columns []listexport.ColumnID `json:"columns"`
}

type studentExportFilters struct {
	Search      string `json:"search"`
	GroupID     string `json:"group_id"`
	RoomID      string `json:"room_id"`
	Year        string `json:"year"`
	Status      string `json:"status"`
	PickupTime  string `json:"pickup_time"`
	ArrivalTime string `json:"arrival_time"`
	Sort        string `json:"sort"`
}

type weeklySchedule struct {
	ArrivalByWeekday map[int]string
	PickupByWeekday  map[int]string
}

func (rs *Resource) exportStudents(w http.ResponseWriter, r *http.Request) {
	if rs.ListExportService == nil {
		renderError(w, r, ErrorInternalServer(errors.New("list export service is not configured")))
		return
	}

	req, err := decodeStudentExportRequest(r)
	if err != nil {
		renderError(w, r, ErrorInvalidRequest(err))
		return
	}

	params := exportRequestToListParams(req)
	students, _, err := rs.fetchStudentsForList(r, params)
	if err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	studentIDs, personIDs, groupIDs := collectIDsFromStudents(students)
	dataSnapshot, err := common.LoadStudentDataSnapshot(r.Context(), rs.PersonService, rs.EducationService, rs.ActiveService, studentIDs, personIDs, groupIDs)
	if err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	accessCtx := rs.determineStudentAccess(r)
	responses := rs.buildStudentResponses(r.Context(), students, params, accessCtx, dataSnapshot, false)
	for i := range responses {
		if responses[i].HasFullAccess {
			applyActualTimesFromSnapshot(&responses[i], dataSnapshot)
		}
	}

	fullAccessIDs := collectFullAccessStudentIDs(responses)
	today := time.Now()
	rs.enrichWithPickupTimes(r.Context(), responses, fullAccessIDs, today)
	rs.enrichWithArrivalTimes(r.Context(), responses, fullAccessIDs, today)

	responses = applyExportFilters(responses, req.Filters)
	sortExportResponses(responses, req.Filters.Sort)

	weekly, err := rs.loadWeeklySchedules(r, collectResponseIDs(responses))
	if err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	doc := listexport.Document{
		Title:       exportTitle(req),
		Subtitle:    rs.exportSubtitle(r, len(responses)),
		GeneratedAt: time.Now(),
		Filters:     exportFilterLabels(req.Filters),
		Columns:     listexport.ResolveColumns(req.Columns, req.Preset),
		Rows:        buildExportRows(responses, weekly),
	}

	file, err := rs.ListExportService.Render(doc, req.Format, doc.Title)
	if err != nil {
		renderError(w, r, ErrorInvalidRequest(err))
		return
	}

	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

func decodeStudentExportRequest(r *http.Request) (studentExportRequest, error) {
	var req studentExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, err
	}
	if req.Format == "" {
		req.Format = listexport.FormatPDF
	}
	if req.Preset == "" {
		req.Preset = listexport.PresetOGSWeekly
	}
	switch req.Format {
	case listexport.FormatPDF, listexport.FormatDOCX, listexport.FormatXLSX:
		return req, nil
	default:
		return req, fmt.Errorf("unsupported export format %q", req.Format)
	}
}

func exportRequestToListParams(req studentExportRequest) *studentListParams {
	params := &studentListParams{
		search:              strings.TrimSpace(req.Filters.Search),
		page:                1,
		pageSize:            studentExportPageSize,
		includePickupTimes:  true,
		includeArrivalTimes: true,
	}
	if req.Filters.GroupID != "" {
		if groupID, err := strconv.ParseInt(req.Filters.GroupID, 10, 64); err == nil {
			params.groupID = groupID
		}
	}
	if req.Filters.RoomID != "" {
		if roomID, err := strconv.ParseInt(req.Filters.RoomID, 10, 64); err == nil {
			params.roomID = roomID
		}
	}
	return params
}

func applyExportFilters(students []StudentResponse, filters studentExportFilters) []StudentResponse {
	filtered := make([]StudentResponse, 0, len(students))
	for _, student := range students {
		if filters.Year != "" && filters.Year != "all" && schoolYear(student.SchoolClass) != filters.Year {
			continue
		}
		if filters.Status != "" && filters.Status != "all" && exportStatus(student) != filters.Status {
			continue
		}
		if filters.PickupTime != "" && filters.PickupTime != "all" {
			if filters.PickupTime == "none" {
				if student.PickupTime != nil || student.PickupIsException {
					continue
				}
			} else if student.PickupTime == nil || *student.PickupTime != filters.PickupTime {
				continue
			}
		}
		if filters.ArrivalTime != "" && filters.ArrivalTime != "all" {
			if filters.ArrivalTime == "none" {
				if student.ArrivalTime != nil || student.ArrivalIsException {
					continue
				}
			} else if student.ArrivalTime == nil || *student.ArrivalTime != filters.ArrivalTime {
				continue
			}
		}
		filtered = append(filtered, student)
	}
	return filtered
}

func sortExportResponses(students []StudentResponse, sortMode string) {
	sort.SliceStable(students, func(i, j int) bool {
		a := students[i]
		b := students[j]
		if sortMode == "pickup" {
			return timeValue(a.PickupTime) < timeValue(b.PickupTime)
		}
		if sortMode == "arrival" {
			return timeValue(a.ArrivalTime) < timeValue(b.ArrivalTime)
		}
		if strings.EqualFold(a.LastName, b.LastName) {
			return strings.ToLower(a.FirstName) < strings.ToLower(b.FirstName)
		}
		return strings.ToLower(a.LastName) < strings.ToLower(b.LastName)
	})
}

func (rs *Resource) loadWeeklySchedules(r *http.Request, studentIDs []int64) (map[int64]weeklySchedule, error) {
	result := make(map[int64]weeklySchedule, len(studentIDs))
	if len(studentIDs) == 0 {
		return result, nil
	}
	if rs.ArrivalScheduleRepo == nil || rs.PickupScheduleRepo == nil {
		return nil, errors.New("student schedule repositories are not configured")
	}
	for _, studentID := range studentIDs {
		result[studentID] = weeklySchedule{
			ArrivalByWeekday: make(map[int]string),
			PickupByWeekday:  make(map[int]string),
		}
	}
	for weekday := schedule.WeekdayMonday; weekday <= schedule.WeekdayFriday; weekday++ {
		arrivals, err := rs.ArrivalScheduleRepo.FindByStudentIDsAndWeekday(r.Context(), studentIDs, weekday)
		if err != nil {
			return nil, err
		}
		for _, arrival := range arrivals {
			weekly := result[arrival.StudentID]
			weekly.ArrivalByWeekday[weekday] = formatWallClock(arrival.ExpectedArrival)
			result[arrival.StudentID] = weekly
		}
		pickups, err := rs.PickupScheduleRepo.FindByStudentIDsAndWeekday(r.Context(), studentIDs, weekday)
		if err != nil {
			return nil, err
		}
		for _, pickup := range pickups {
			weekly := result[pickup.StudentID]
			weekly.PickupByWeekday[weekday] = formatWallClock(pickup.PickupTime)
			result[pickup.StudentID] = weekly
		}
	}
	return result, nil
}

func buildExportRows(students []StudentResponse, weekly map[int64]weeklySchedule) []listexport.Row {
	rows := make([]listexport.Row, 0, len(students))
	for _, student := range students {
		plan := weekly[student.ID]
		rows = append(rows, listexport.Row{Values: map[listexport.ColumnID]string{
			listexport.ColumnName:            strings.TrimSpace(student.FirstName + " " + student.LastName),
			listexport.ColumnSchoolClass:     student.SchoolClass,
			listexport.ColumnGroup:           student.GroupName,
			listexport.ColumnCareDays:        careDays(plan),
			listexport.ColumnWeeklyMonday:    weeklyCell(plan, schedule.WeekdayMonday),
			listexport.ColumnWeeklyTuesday:   weeklyCell(plan, schedule.WeekdayTuesday),
			listexport.ColumnWeeklyWednesday: weeklyCell(plan, schedule.WeekdayWednesday),
			listexport.ColumnWeeklyThursday:  weeklyCell(plan, schedule.WeekdayThursday),
			listexport.ColumnWeeklyFriday:    weeklyCell(plan, schedule.WeekdayFriday),
			listexport.ColumnPlannedArrival:  ptrValue(student.ArrivalTime),
			listexport.ColumnPlannedPickup:   ptrValue(student.PickupTime),
			listexport.ColumnDailyNotes:      dailyNotes(student),
			listexport.ColumnCurrentLocation: student.Location,
		}})
	}
	return rows
}

func weeklyCell(plan weeklySchedule, weekday int) string {
	arrival := plan.ArrivalByWeekday[weekday]
	pickup := plan.PickupByWeekday[weekday]
	if arrival == "" && pickup == "" {
		return "nein"
	}
	if arrival != "" && pickup != "" {
		return "Ankunft: " + arrival + ", Abholung: " + pickup
	}
	if arrival != "" {
		return "Ankunft: " + arrival
	}
	return "Abholung: " + pickup
}

func careDays(plan weeklySchedule) string {
	labels := []string{}
	for _, day := range []struct {
		weekday int
		label   string
	}{
		{schedule.WeekdayMonday, "Mo"},
		{schedule.WeekdayTuesday, "Di"},
		{schedule.WeekdayWednesday, "Mi"},
		{schedule.WeekdayThursday, "Do"},
		{schedule.WeekdayFriday, "Fr"},
	} {
		if plan.ArrivalByWeekday[day.weekday] != "" || plan.PickupByWeekday[day.weekday] != "" {
			labels = append(labels, day.label)
		}
	}
	if len(labels) == 0 {
		return "keine"
	}
	return strings.Join(labels, ", ")
}

func dailyNotes(student StudentResponse) string {
	notes := []string{}
	if student.ArrivalNotes != "" {
		notes = append(notes, "Ankunft: "+student.ArrivalNotes)
	}
	if student.PickupNotes != "" {
		notes = append(notes, "Abholung: "+student.PickupNotes)
	}
	return strings.Join(notes, "; ")
}

func collectResponseIDs(students []StudentResponse) []int64 {
	ids := make([]int64, 0, len(students))
	for _, student := range students {
		if student.HasFullAccess {
			ids = append(ids, student.ID)
		}
	}
	return ids
}

func (rs *Resource) exportSubtitle(r *http.Request, count int) string {
	name := "Kindersuche"
	if tenantID := tenant.FromContext(r.Context()); tenantID > 0 && rs.SchoolRepo != nil {
		if school, err := rs.SchoolRepo.FindByID(r.Context(), tenantID); err == nil && school != nil && school.Name != "" {
			name = school.Name
		}
	}
	return fmt.Sprintf("%s - %d Kinder", name, count)
}

func exportTitle(req studentExportRequest) string {
	title := strings.TrimSpace(req.Title)
	if title != "" {
		return title
	}
	switch req.Preset {
	case listexport.PresetOGSCompact:
		return "OGS Kompaktliste"
	case listexport.PresetDailyPlanning:
		return "Tagesliste"
	case listexport.PresetAttendanceSnapshot:
		return "Anwesenheitsliste"
	case listexport.PresetPickupList:
		return "Abholliste"
	case listexport.PresetBlankChecklist:
		return "Checkliste"
	default:
		return "OGS Wochenliste"
	}
}

func exportFilterLabels(filters studentExportFilters) []string {
	labels := []string{}
	if filters.Search != "" {
		labels = append(labels, "Suche: "+filters.Search)
	}
	if filters.GroupID != "" {
		labels = append(labels, "Gruppe gefiltert")
	}
	if filters.Year != "" && filters.Year != "all" {
		labels = append(labels, "Stufe: "+filters.Year)
	}
	if filters.Status != "" && filters.Status != "all" {
		labels = append(labels, "Momentaufnahme: "+filters.Status)
	}
	return labels
}

func schoolYear(schoolClass string) string {
	for _, r := range schoolClass {
		if r >= '1' && r <= '9' {
			return string(r)
		}
	}
	return ""
}

func exportStatus(student StudentResponse) string {
	if student.Sick {
		return "krank"
	}
	if student.Excused {
		return "entschuldigt"
	}
	switch student.Location {
	case "Zuhause", "":
		return "abwesend"
	case "Unterwegs":
		return "unterwegs"
	case "Schulhof":
		return "schulhof"
	default:
		return "anwesend"
	}
}

func timeValue(value *string) string {
	if value == nil || *value == "" {
		return "99:99"
	}
	return *value
}

func ptrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func formatWallClock(value time.Time) string {
	return timezone.WallClock(value).Format("15:04")
}
