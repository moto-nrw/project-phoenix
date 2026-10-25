package announcement

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	announcementService "github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// SurveyResource is the staff-portal HTTP surface of the surveys (#4781),
// mounted at /api/surveys. Authoring and evaluation need surveys:manage;
// answering a staff survey needs only a place in its audience, which the
// service checks.
type SurveyResource struct {
	Service announcementService.SurveyCapability
	Reports ReportRenderer
}

// NewSurveyResource wires the survey routes.
func NewSurveyResource(service announcementService.SurveyCapability, reports ReportRenderer) *SurveyResource {
	return &SurveyResource{Service: service, Reports: reports}
}

// Router returns the chi router scoped to /surveys.
func (rs *SurveyResource) Router(protectedTenantRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	protectedTenantRoutes(r, func(r chi.Router, withTx common.Middleware) {
		manage := common.RequiresPermission(permissions.SurveysManage)
		r.With(withTx).Get("/to-answer", rs.toAnswer)
		r.With(withTx).Get("/{surveyId}/form", rs.form)
		r.With(withTx).Post("/{surveyId}/answers", rs.answer)

		r.With(manage, withTx).Get("/", rs.list)
		r.With(manage, withTx).Post("/", rs.create)
		r.With(manage, withTx).Get("/{surveyId}", rs.get)
		r.With(manage, withTx).Put("/{surveyId}", rs.update)
		r.With(manage, withTx).Delete("/{surveyId}", rs.delete)
		r.With(manage, withTx).Post("/{surveyId}/publish", rs.publish)
		r.With(manage, withTx).Post("/{surveyId}/close", rs.close)
		r.With(manage, withTx).Post("/{surveyId}/remind", rs.remind)
		r.With(manage, withTx).Get("/{surveyId}/results", rs.results)
		r.With(manage, withTx).Get("/{surveyId}/results/export", rs.export)
		r.With(manage, withTx).Get("/{surveyId}/respondents", rs.respondents)
	})
	return r
}

var surveyErrorRules = []common.ErrorRule{
	{Target: announcementService.ErrSurveyNotFound, Render: common.ErrorNotFound},
	{Target: announcementService.ErrSurveyInvalid, Render: common.ErrorInvalidRequest},
	{Target: announcementService.ErrSurveyNotDraft, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeCommunicationSurveyNotDraft)
	}},
	{Target: announcementService.ErrSurveyNotOpen, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeCommunicationSurveyNotOpen)
	}},
	{Target: announcementService.ErrSurveyAlreadyAnswered, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeCommunicationSurveyAlreadyAnswered)
	}},
	{Target: announcementService.ErrSurveyAnonymous, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeCommunicationSurveyAnonymous)
	}},
}

var renderSurveyError = common.RulesRenderer(surveyErrorRules, func(err error) render.Renderer {
	return common.ErrorInternalServerWrap("survey request failed", err)
})

func surveyFailure(w http.ResponseWriter, r *http.Request, err error) {
	common.RenderError(w, r, renderSurveyError(err))
}

func parseSurveyID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return common.ParsePositiveInt64IDWithError(w, r, "surveyId", "invalid survey ID")
}

func callerAccountID(r *http.Request) int64 {
	return int64(sessiontoken.ClaimsFromCtx(r.Context()).ID)
}

func (rs *SurveyResource) list(w http.ResponseWriter, r *http.Request) {
	surveys, err := rs.Service.ListSurveys(r.Context())
	if err != nil {
		surveyFailure(w, r, err)
		return
	}
	out := make([]surveyResponse, 0, len(surveys))
	for i := range surveys {
		out = append(out, toSurveyResponse(&surveys[i]))
	}
	common.Respond(w, r, http.StatusOK, out, "Surveys retrieved")
}

func (rs *SurveyResource) get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseSurveyID(w, r)
	if !ok {
		return
	}
	survey, err := rs.Service.GetSurvey(r.Context(), id)
	rs.respondSurvey(w, r, survey, err, http.StatusOK, "Survey retrieved")
}

func (rs *SurveyResource) create(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeSurveyRequest(w, r)
	if !ok {
		return
	}
	survey, err := rs.Service.CreateSurvey(r.Context(), callerAccountID(r), in)
	rs.respondSurvey(w, r, survey, err, http.StatusCreated, "Survey created")
}

func (rs *SurveyResource) update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseSurveyID(w, r)
	if !ok {
		return
	}
	in, ok := decodeSurveyRequest(w, r)
	if !ok {
		return
	}
	survey, err := rs.Service.UpdateSurvey(r.Context(), id, in)
	rs.respondSurvey(w, r, survey, err, http.StatusOK, "Survey updated")
}

func (rs *SurveyResource) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseSurveyID(w, r)
	if !ok {
		return
	}
	if err := rs.Service.DeleteSurvey(r.Context(), id); err != nil {
		surveyFailure(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]bool{"deleted": true}, "Survey deleted")
}

func (rs *SurveyResource) publish(w http.ResponseWriter, r *http.Request) {
	id, ok := parseSurveyID(w, r)
	if !ok {
		return
	}
	survey, err := rs.Service.PublishSurvey(r.Context(), callerAccountID(r), id)
	rs.respondSurvey(w, r, survey, err, http.StatusOK, "Survey published")
}

func (rs *SurveyResource) close(w http.ResponseWriter, r *http.Request) {
	id, ok := parseSurveyID(w, r)
	if !ok {
		return
	}
	survey, err := rs.Service.CloseSurvey(r.Context(), id)
	rs.respondSurvey(w, r, survey, err, http.StatusOK, "Survey closed")
}

func (rs *SurveyResource) respondSurvey(w http.ResponseWriter, r *http.Request, survey *announcementService.Survey, err error, status int, msg string) {
	if err != nil {
		surveyFailure(w, r, err)
		return
	}
	common.Respond(w, r, status, toSurveyResponse(survey), msg)
}

func (rs *SurveyResource) remind(w http.ResponseWriter, r *http.Request) {
	id, ok := parseSurveyID(w, r)
	if !ok {
		return
	}
	count, err := rs.Service.RemindSurvey(r.Context(), id)
	if err != nil {
		surveyFailure(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]int{"reminded": count}, "Survey reminder sent")
}

func (rs *SurveyResource) results(w http.ResponseWriter, r *http.Request) {
	id, ok := parseSurveyID(w, r)
	if !ok {
		return
	}
	results, err := rs.Service.SurveyResults(r.Context(), id)
	if err != nil {
		surveyFailure(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toSurveyResultsResponse(results), "Survey results retrieved")
}

// export renders the evaluation as a PDF: the same sums and distributions as
// the screen, below the minimum only the count.
func (rs *SurveyResource) export(w http.ResponseWriter, r *http.Request) {
	id, ok := parseSurveyID(w, r)
	if !ok {
		return
	}
	results, err := rs.Service.SurveyResults(r.Context(), id)
	if err != nil {
		surveyFailure(w, r, err)
		return
	}
	if rs.Reports == nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("render survey export", errors.New("survey report renderer is not bound")))
		return
	}
	file, err := rs.Reports.RenderReport(surveyReport(results, time.Now()), "umfrage-auswertung")
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("render survey export", err))
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

func (rs *SurveyResource) respondents(w http.ResponseWriter, r *http.Request) {
	id, ok := parseSurveyID(w, r)
	if !ok {
		return
	}
	rows, err := rs.Service.SurveyRespondents(r.Context(), id)
	if err != nil {
		surveyFailure(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toRespondentResponses(rows), "Survey respondents retrieved")
}

func (rs *SurveyResource) toAnswer(w http.ResponseWriter, r *http.Request) {
	rows, err := rs.Service.StaffSurveysToAnswer(r.Context(), callerAccountID(r))
	if err != nil {
		surveyFailure(w, r, err)
		return
	}
	out := make([]surveyToAnswerResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, surveyToAnswerResponse{
			ID: strconv.FormatInt(row.ID, 10), Title: row.Title, Intro: row.Intro,
			Anonymous: row.Anonymous, ClosesAt: row.ClosesAt, Participated: row.Participated,
		})
	}
	common.Respond(w, r, http.StatusOK, out, "Open surveys retrieved")
}

func (rs *SurveyResource) form(w http.ResponseWriter, r *http.Request) {
	id, ok := parseSurveyID(w, r)
	if !ok {
		return
	}
	form, err := rs.Service.StaffSurveyForm(r.Context(), callerAccountID(r), id)
	if err != nil {
		surveyFailure(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toSurveyFormResponse(form), "Survey retrieved")
}

func (rs *SurveyResource) answer(w http.ResponseWriter, r *http.Request) {
	id, ok := parseSurveyID(w, r)
	if !ok {
		return
	}
	answers, err := decodeSurveyAnswers(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := rs.Service.SubmitStaffSurvey(r.Context(), callerAccountID(r), id, answers); err != nil {
		surveyFailure(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]bool{"submitted": true}, "Survey answered")
}

func decodeSurveyRequest(w http.ResponseWriter, r *http.Request) (announcementService.SurveyInput, bool) {
	var req surveyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return announcementService.SurveyInput{}, false
	}
	in, err := req.input()
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return announcementService.SurveyInput{}, false
	}
	return in, true
}
