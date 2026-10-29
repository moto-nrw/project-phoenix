package statisticshttp

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Soll und Ist je Kind (#4774): the comparison of one child on the child
// page and the overrun list of the school management.

var renderCareTimeError = common.RulesRenderer([]common.ErrorRule{
	{Target: studentpresence.ErrInvalidStatisticsRange, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodePresenceStatisticsRangeInvalid)
	}},
	{Target: studentpresence.ErrCareTimeStudentNotFound, Render: common.ErrorNotFound},
}, func(err error) render.Renderer {
	return common.ErrorInternalServerWrap("care time comparison failed", err)
})

func (rs *Resource) getStudentCareTimes(w http.ResponseWriter, r *http.Request) {
	studentID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid student id")
	if !ok {
		return
	}
	filters, err := parseCareTimeFilters(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	result, err := rs.Service.StudentCareTimes(r.Context(), studentID, filters.From, filters.To)
	if err != nil {
		rs.logCareTimeFailure("student care times failed", err)
		common.RenderError(w, r, renderCareTimeError(err))
		return
	}
	common.Respond(w, r, http.StatusOK, toStudentCareTimesResponse(result), "")
}

func (rs *Resource) getCareTimeOverruns(w http.ResponseWriter, r *http.Request) {
	filters, err := parseCareTimeFilters(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	result, err := rs.Service.CareTimeOverruns(r.Context(), filters, rs.Runtime.Actor(r))
	if err != nil {
		rs.logCareTimeFailure("care time overruns failed", err)
		common.RenderError(w, r, renderCareTimeError(err))
		return
	}
	common.Respond(w, r, http.StatusOK, toCareTimeOverrunsResponse(result), "")
}

func (rs *Resource) exportCareTimeOverruns(w http.ResponseWriter, r *http.Request) {
	if rs.Runtime.Export == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("statistics export service is not configured")))
		return
	}
	filters, err := parseCareTimeFilters(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	format, err := parseFormat(r)
	if err != nil || format == formatDOCX {
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("unsupported export format %q", format)))
		return
	}
	result, err := rs.Service.CareTimeOverrunsForExport(r.Context(), filters, rs.Runtime.Actor(r), string(format))
	if err != nil {
		rs.logCareTimeFailure("care time overrun export failed", err)
		common.RenderError(w, r, renderCareTimeError(err))
		return
	}
	filename := "soll-und-ist-" + result.From.String() + "-" + result.To.String()
	file, err := rs.Runtime.Export(buildCareTimeExportDocument(result), format, filename)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	writeExportFile(w, file)
}

func writeExportFile(w http.ResponseWriter, file ExportFile) {
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

func (rs *Resource) logCareTimeFailure(msg string, err error) {
	if errors.Is(err, studentpresence.ErrCareTimeStudentNotFound) {
		return
	}
	rs.logFailure(msg, err)
}

// parseCareTimeFilters reads from, to (YYYY-MM-DD, required), group_id and
// school_class.
func parseCareTimeFilters(r *http.Request) (studentpresence.CareTimeFilters, error) {
	filters, err := parseFilters(r)
	if err != nil {
		return studentpresence.CareTimeFilters{}, err
	}
	return studentpresence.CareTimeFilters{
		From: filters.From, To: filters.To, GroupIDs: filters.GroupIDs, SchoolClasses: filters.SchoolClasses,
	}, nil
}

type careTimeSummaryResponse struct {
	OverrunCount     int `json:"overrun_count"`
	OverrunDays      int `json:"overrun_days"`
	UnbookedDays     int `json:"unbooked_days"`
	AbsentDays       int `json:"absent_days"`
	ExtraMinutes     int `json:"extra_minutes"`
	PlannedMinutes   int `json:"planned_minutes"`
	ActualMinutes    int `json:"actual_minutes"`
	DeviationMinutes int `json:"deviation_minutes"`
}

type careTimeDayResponse struct {
	Date             string  `json:"date"`
	Kind             string  `json:"kind"`
	Overrun          bool    `json:"overrun"`
	PlannedStart     *string `json:"planned_start"`
	PlannedEnd       *string `json:"planned_end"`
	ActualStart      *string `json:"actual_start"`
	ActualEnd        *string `json:"actual_end"`
	PlannedMinutes   *int    `json:"planned_minutes"`
	ActualMinutes    *int    `json:"actual_minutes"`
	DeviationMinutes *int    `json:"deviation_minutes"`
	EarlyMinutes     int     `json:"early_minutes"`
	LateMinutes      int     `json:"late_minutes"`
}

type studentCareTimesResponse struct {
	StudentID        string                  `json:"student_id"`
	From             string                  `json:"from"`
	To               string                  `json:"to"`
	DataFrom         string                  `json:"data_from"`
	ToleranceMinutes int                     `json:"tolerance_minutes"`
	Summary          careTimeSummaryResponse `json:"summary"`
	Days             []careTimeDayResponse   `json:"days"`
}

type careTimeOverrunRowResponse struct {
	StudentID   string                  `json:"student_id"`
	FirstName   string                  `json:"first_name"`
	LastName    string                  `json:"last_name"`
	SchoolClass string                  `json:"school_class"`
	GroupName   string                  `json:"group_name"`
	Summary     careTimeSummaryResponse `json:"summary"`
}

type careTimeOverrunsResponse struct {
	From             string                       `json:"from"`
	To               string                       `json:"to"`
	DataFrom         string                       `json:"data_from"`
	ToleranceMinutes int                          `json:"tolerance_minutes"`
	Students         []careTimeOverrunRowResponse `json:"students"`
}

func toCareTimeSummaryResponse(summary studentpresence.CareTimeSummary) careTimeSummaryResponse {
	return careTimeSummaryResponse{
		OverrunCount: summary.OverrunCount(), OverrunDays: summary.OverrunDays, UnbookedDays: summary.UnbookedDays,
		AbsentDays: summary.AbsentDays, ExtraMinutes: summary.ExtraMinutes, PlannedMinutes: summary.PlannedMinutes,
		ActualMinutes: summary.ActualMinutes, DeviationMinutes: summary.DeviationMinutes,
	}
}

func formatClock(clock *calendar.WallClock) *string {
	if clock == nil {
		return nil
	}
	value := fmt.Sprintf("%02d:%02d", clock.Hour(), clock.Minute())
	return &value
}

func toStudentCareTimesResponse(result *studentpresence.StudentCareTimes) studentCareTimesResponse {
	days := make([]careTimeDayResponse, 0, len(result.Days))
	for _, day := range result.Days {
		days = append(days, careTimeDayResponse{
			Date: day.Date.String(), Kind: string(day.Kind), Overrun: day.Kind.IsOverrun(),
			PlannedStart: formatClock(day.PlannedStart), PlannedEnd: formatClock(day.PlannedEnd),
			ActualStart: formatClock(day.ActualStart), ActualEnd: formatClock(day.ActualEnd),
			PlannedMinutes: day.PlannedMinutes, ActualMinutes: day.ActualMinutes, DeviationMinutes: day.DeviationMinutes,
			EarlyMinutes: day.EarlyMinutes, LateMinutes: day.LateMinutes,
		})
	}
	return studentCareTimesResponse{
		StudentID: strconv.FormatInt(result.StudentID, 10), From: result.From.String(), To: result.To.String(),
		DataFrom:         result.DataFrom.String(),
		ToleranceMinutes: result.ToleranceMinutes, Summary: toCareTimeSummaryResponse(result.Summary), Days: days,
	}
}

func toCareTimeOverrunsResponse(result *studentpresence.CareTimeOverruns) careTimeOverrunsResponse {
	students := make([]careTimeOverrunRowResponse, 0, len(result.Students))
	for _, row := range result.Students {
		students = append(students, careTimeOverrunRowResponse{
			StudentID: strconv.FormatInt(row.StudentID, 10), FirstName: row.FirstName, LastName: row.LastName,
			SchoolClass: row.SchoolClass, GroupName: row.GroupName, Summary: toCareTimeSummaryResponse(row.Summary),
		})
	}
	return careTimeOverrunsResponse{
		From: result.From.String(), To: result.To.String(), DataFrom: result.DataFrom.String(),
		ToleranceMinutes: result.ToleranceMinutes, Students: students,
	}
}

const (
	columnGroup        = "group"
	columnOverruns     = "overruns"
	columnUnbooked     = "unbooked_days"
	columnExtraMinutes = "extra_minutes"
)

// buildCareTimeExportDocument renders the overrun list in the order and with
// the definitions the screen shows.
func buildCareTimeExportDocument(result *studentpresence.CareTimeOverruns) ExportDocument {
	doc := ExportDocument{
		Title:       "Soll und Ist",
		Subtitle:    fmt.Sprintf("Kinder mit Überschreitungen vom %s bis %s", result.From.Format("02.01.2006"), result.To.Format("02.01.2006")),
		GeneratedAt: time.Now(),
		Columns: []ExportColumn{
			{ID: columnName, Label: "Name"},
			{ID: columnSchoolClass, Label: "Klasse"},
			{ID: columnGroup, Label: "Gruppe"},
			{ID: columnOverruns, Label: "Überschreitungen"},
			{ID: columnUnbooked, Label: "Davon ohne Buchung"},
			{ID: columnExtraMinutes, Label: "Minuten über Buchung"},
		},
		Filters: []string{
			"Soll = hinterlegte Betreuungszeit, Ist = erste Anmeldung bis letzte Abmeldung",
			fmt.Sprintf("Überschreitung = mehr als %d Minuten früher oder später als gebucht oder da ohne Buchung", result.ToleranceMinutes),
		},
		Footer: exportConfidentialityNote,
	}
	if result.DataFrom.After(result.From) {
		doc.Filters = append(doc.Filters, fmt.Sprintf("Verglichen ab %s; ältere Anwesenheiten sind nicht mehr gespeichert.", result.DataFrom.Format("02.01.2006")))
	}
	for _, row := range result.Students {
		doc.Rows = append(doc.Rows, ExportRow{Values: map[string]string{
			columnName:         row.LastName + ", " + row.FirstName,
			columnSchoolClass:  row.SchoolClass,
			columnGroup:        row.GroupName,
			columnOverruns:     strconv.Itoa(row.Summary.OverrunCount()),
			columnUnbooked:     strconv.Itoa(row.Summary.UnbookedDays),
			columnExtraMinutes: strconv.Itoa(row.Summary.ExtraMinutes),
		}})
	}
	return doc
}
