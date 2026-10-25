package enrollmenthttp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The participant lists of the hosting school (#4665, ADR 0116), mounted at
// /api/holiday-weeks in the school portal: the weeks the school hosts, the
// confirmed children of one week with the counts per time model and home
// school, and the CSV and Excel export. The school and the person come from
// the session; the tenant transaction of the school reads only what the host
// policies admit.

// HolidayHostResource serves the participant lists to the hosting school.
type HolidayHostResource struct {
	participants capability.HolidayHostParticipants
	schoolNames  capability.HolidayHostSchoolNames
	requireRead  common.Middleware
}

// NewHolidayHostResource builds the routes. requireRead is the school
// permission that opens the participant lists.
func NewHolidayHostResource(participants capability.HolidayHostParticipants, schoolNames capability.HolidayHostSchoolNames, requireRead common.Middleware) *HolidayHostResource {
	if participants == nil || schoolNames == nil || requireRead == nil {
		panic("enrollment holiday host resource: participants, school names and the permission are required")
	}
	return &HolidayHostResource{participants: participants, schoolNames: schoolNames, requireRead: requireRead}
}

// Router mounts the routes behind the school session.
func (rs *HolidayHostResource) Router(protectedTenantRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTenantRoutes(r, func(r chi.Router, withTx common.Middleware) {
		r.With(rs.requireRead, withTx).Get("/", rs.listWeeks)
		read := r.With(rs.requireRead, rs.withSchoolNames, withTx)
		read.Get("/{weekId}/participants", rs.getParticipants)
		read.Get("/{weekId}/participants/export", rs.exportParticipants)
	})
	return r
}

// hostOf returns the school and the account of the session, or renders 401.
func hostOf(w http.ResponseWriter, r *http.Request) (capability.HolidayHost, bool) {
	principal, err := common.CurrentPrincipal(r.Context())
	if err != nil || principal.TenantID() <= 0 || principal.AccountID() <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return capability.HolidayHost{}, false
	}
	return capability.HolidayHost{SchoolID: principal.TenantID(), AccountID: principal.AccountID(), Role: actorRoleFromClaims(principal.Roles())}, true
}

func (rs *HolidayHostResource) listWeeks(w http.ResponseWriter, r *http.Request) {
	host, ok := hostOf(w, r)
	if !ok {
		return
	}
	weeks, err := rs.participants.HostedHolidayWeeks(r.Context(), host)
	if err != nil {
		renderHolidayHostError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, weeks, "Hosted holiday weeks retrieved successfully")
}

// holidayHomeSchoolResponse is a home school count with the school's name; a
// nil SchoolID counts the holiday guests.
type holidayHomeSchoolResponse struct {
	SchoolID *int64 `json:"school_id"`
	Name     string `json:"name"`
	Count    int    `json:"count"`
}

// holidayParticipantResponse is a participant with the name of its home
// school.
type holidayParticipantResponse struct {
	capability.HolidayParticipant
	HomeSchoolName string `json:"home_school_name"`
}

type holidayParticipantListResponse struct {
	Week         capability.HostedHolidayWeek       `json:"week"`
	TimeModels   []capability.HolidayTimeModelCount `json:"time_models"`
	HomeSchools  []holidayHomeSchoolResponse        `json:"home_schools"`
	Participants []holidayParticipantResponse       `json:"participants"`
}

func (rs *HolidayHostResource) getParticipants(w http.ResponseWriter, r *http.Request) {
	host, weekID, ok := hostedWeekKey(w, r)
	if !ok {
		return
	}
	list, err := rs.participants.HolidayParticipants(r.Context(), host, weekID)
	if err != nil {
		renderHolidayHostError(w, r, err)
		return
	}
	names := hostSchoolNames(r.Context())
	response := holidayParticipantListResponse{
		Week: list.Week, TimeModels: list.TimeModels,
		HomeSchools:  make([]holidayHomeSchoolResponse, len(list.HomeSchools)),
		Participants: make([]holidayParticipantResponse, len(list.Participants)),
	}
	for i, school := range list.HomeSchools {
		response.HomeSchools[i] = holidayHomeSchoolResponse{SchoolID: school.SchoolID, Name: names.homeSchool(school.SchoolID), Count: school.Count}
	}
	for i, participant := range list.Participants {
		response.Participants[i] = holidayParticipantResponse{HolidayParticipant: participant, HomeSchoolName: names.participantSchool(participant)}
	}
	common.Respond(w, r, http.StatusOK, response, "Holiday participants retrieved successfully")
}

// exportParticipants answers GET .../participants/export?format=csv|xlsx:
// one row per child; the Excel file names the counts above the table.
func (rs *HolidayHostResource) exportParticipants(w http.ResponseWriter, r *http.Request) {
	host, weekID, ok := hostedWeekKey(w, r)
	if !ok {
		return
	}
	format := lists.Format(strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format"))))
	if format != lists.FormatCSV && format != lists.FormatXLSX {
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("unsupported export format %q (use csv or xlsx)", format)))
		return
	}
	list, err := rs.participants.ExportHolidayParticipants(r.Context(), host, weekID, string(format))
	if err != nil {
		renderHolidayHostError(w, r, err)
		return
	}
	file, err := lists.NewRenderer().Render(holidayParticipantDocument(list, hostSchoolNames(r.Context())), format,
		"teilnehmerliste-"+list.Week.StartsOn.String())
	if err != nil {
		renderHolidayHostError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

func hostedWeekKey(w http.ResponseWriter, r *http.Request) (capability.HolidayHost, int64, bool) {
	host, ok := hostOf(w, r)
	if !ok {
		return capability.HolidayHost{}, 0, false
	}
	weekID, err := common.ParseIDParam(r, "weekId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return capability.HolidayHost{}, 0, false
	}
	return host, weekID, true
}

// renderHolidayHostError answers a refused read. The read wrote nothing but
// its access log entry, which goes with the rollback.
func renderHolidayHostError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, capability.ErrHostedHolidayWeekNotFound) {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentHostedHolidayWeekNotFound))
		return
	}
	common.RenderError(w, r, common.ErrorInternalServer(err))
}

type holidayHostSchoolNamesKey struct{}

// withSchoolNames reads the names of the schools of the hosting school's
// Träger before the tenant transaction opens: Organisation & Tenancy reads
// them in its own unit.
func (rs *HolidayHostResource) withSchoolNames(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, ok := hostOf(w, r)
		if !ok {
			return
		}
		names, err := rs.schoolNames.HolidayHostSchoolNames(r.Context(), host.SchoolID)
		if err != nil {
			common.RenderError(w, r, common.ErrorInternalServer(err))
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), holidayHostSchoolNamesKey{}, holidaySchoolNameMap(names))))
	})
}

// holidaySchoolNameMap names the home schools of a participant list.
type holidaySchoolNameMap map[int64]string

func hostSchoolNames(ctx context.Context) holidaySchoolNameMap {
	names, _ := ctx.Value(holidayHostSchoolNamesKey{}).(holidaySchoolNameMap)
	return names
}

// homeSchool names a school of the Träger; nil is the holiday guests.
func (n holidaySchoolNameMap) homeSchool(id *int64) string {
	if id == nil {
		return "Feriengäste"
	}
	if name, ok := n[*id]; ok {
		return name
	}
	return "Schule nicht mehr vorhanden"
}

// participantSchool names the home school of a child; a holiday guest shows
// the school its parents named.
func (n holidaySchoolNameMap) participantSchool(participant capability.HolidayParticipant) string {
	if participant.HomeSchoolID != nil {
		return n.homeSchool(participant.HomeSchoolID)
	}
	if participant.GuestSchoolName != "" {
		return "Feriengast (" + participant.GuestSchoolName + ")"
	}
	return "Feriengast"
}

var holidayParticipantColumns = []lists.Column{
	{ID: "last_name", Label: "Nachname"},
	{ID: "first_name", Label: "Vorname"},
	{ID: "birth_date", Label: "Geburtsdatum"},
	{ID: "home_school", Label: "Heimatschule"},
	{ID: "time_model", Label: "Zeitmodell"},
}

// holidayParticipantDocument is the export of a participant list: one row
// per child, the counts per time model and home school as header lines.
func holidayParticipantDocument(list capability.HolidayParticipantList, names holidaySchoolNameMap) lists.Document {
	models := make(map[int64]string, len(list.TimeModels))
	header := []string{"Woche: " + list.Week.StartsOn.Format("02.01.") + " bis " + list.Week.EndsOn.Format("02.01.2006"),
		"Kinder: " + strconv.Itoa(len(list.Participants))}
	for _, model := range list.TimeModels {
		models[model.ID] = model.Name
		header = append(header, "Zeitmodell "+lists.SanitizeUserText(model.Name)+": "+strconv.Itoa(model.Count))
	}
	for _, school := range list.HomeSchools {
		header = append(header, "Heimatschule "+lists.SanitizeUserText(names.homeSchool(school.SchoolID))+": "+strconv.Itoa(school.Count))
	}
	rows := make([]lists.Row, len(list.Participants))
	for i, participant := range list.Participants {
		values := map[lists.ColumnID]string{
			"last_name": participant.LastName, "first_name": participant.FirstName,
			"birth_date": participant.BirthDate.Format("02.01.2006"), "home_school": names.participantSchool(participant),
			"time_model": models[participant.TimeModelID],
		}
		for id, value := range values {
			values[id] = lists.SanitizeUserText(value)
		}
		rows[i] = lists.Row{Values: values}
	}
	return lists.Document{
		Title: "Teilnehmerliste", Subtitle: list.Week.PeriodName, GeneratedAt: time.Now(),
		Filters: header, Columns: holidayParticipantColumns, Rows: rows,
		Footer: "Vertraulich. Enthält personenbezogene Daten.",
	}
}
