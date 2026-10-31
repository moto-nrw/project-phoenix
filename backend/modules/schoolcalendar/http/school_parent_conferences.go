package staffcalendarhttp

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	calendarService "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
)

// mountSchoolParentConferences mounts the Elternsprechtag of moto schule
// (#4801) under /school/calendar/parent-conferences. The Lehrkraft offers a
// day for a class she teaches, sees its occupancy and prints it.
func (rs *Resource) mountSchoolParentConferences(r chi.Router, school, withTx common.Middleware) {
	r.With(school, withTx).Get("/parent-conferences", rs.listSchoolParentConferences)
	r.With(school, withTx).Post("/parent-conferences", rs.createSchoolParentConference)
	r.With(school, withTx).Get("/parent-conferences/{conferenceId}", rs.getSchoolParentConference)
	r.With(school, withTx).Put("/parent-conferences/{conferenceId}", rs.updateSchoolParentConference)
	r.With(school, withTx).Delete("/parent-conferences/{conferenceId}", rs.deleteSchoolParentConference)
}

func (rs *Resource) listSchoolParentConferences(w http.ResponseWriter, r *http.Request) {
	list, err := rs.service.ListSchoolParentConferences(r.Context())
	if err != nil {
		renderCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, list, "Parent conferences retrieved")
}

func (rs *Resource) getSchoolParentConference(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "conferenceId", "invalid parent conference ID")
	if !ok {
		return
	}
	view, err := rs.service.GetSchoolParentConference(r.Context(), id)
	if err != nil {
		renderCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, view, "Parent conference retrieved")
}

func (rs *Resource) createSchoolParentConference(w http.ResponseWriter, r *http.Request) {
	var req calendarService.ParentConferenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	view, err := rs.service.CreateSchoolParentConference(r.Context(), req)
	if err != nil {
		renderCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, view, "Parent conference created")
}

func (rs *Resource) updateSchoolParentConference(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "conferenceId", "invalid parent conference ID")
	if !ok {
		return
	}
	var req calendarService.ParentConferenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	view, err := rs.service.UpdateSchoolParentConference(r.Context(), id, req)
	if err != nil {
		renderCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, view, "Parent conference updated")
}

func (rs *Resource) deleteSchoolParentConference(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "conferenceId", "invalid parent conference ID")
	if !ok {
		return
	}
	if err := rs.service.DeleteSchoolParentConference(r.Context(), id); err != nil {
		renderCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{"status": "deleted"}, "Parent conference deleted")
}
