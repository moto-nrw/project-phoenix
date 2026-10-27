package classdayhttp

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// Absence reports through "moto schule" (#4791): the Schulverwaltung enters
// sick or excused for a child the Lehrkraft marked absent without a report,
// and takes such a report back. Every write passes the school scope, the
// class_day:absence_report_write permission and the caller's class reach
// here; the projection then checks today, the school day, the roster and
// that the child misses without a report.

// errAbsenceReportsNotConfigured answers when the projection was composed
// without the absence reports.
var errAbsenceReportsNotConfigured = errors.New("absence reports are not configured")

// AbsenceReportRequest is the body of
// PUT /school/class-day/absence-reports/{schoolClass}/{date}/students/{studentId}.
type AbsenceReportRequest struct {
	// Status is "sick" or "excused".
	Status string `json:"status"`
}

// Bind implements render.Binder.
func (r *AbsenceReportRequest) Bind(_ *http.Request) error {
	if r.Status != classday.AbsenceReportSick && r.Status != classday.AbsenceReportExcused {
		return errors.New("status must be sick or excused")
	}
	return nil
}

var absenceReportErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: classday.ErrAbsenceReportNotToday, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeClassdayAbsenceReportNotToday)
	}},
	{Target: classday.ErrAbsenceReportNoSchoolDay, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeClassdayAbsenceReportNoSchoolDay)
	}},
	{Target: classday.ErrAbsenceReportUnknownChild, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeClassdayAbsenceReportUnknownChild)
	}},
	{Target: classday.ErrAbsenceReportNotMissing, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeClassdayAbsenceReportNotMissing)
	}},
	{Target: classday.ErrAbsenceReportConflict, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeClassdayAbsenceReportConflict)
	}},
	{Target: classday.ErrAbsenceReportNotFound, Render: func(err error) render.Renderer {
		return common.ErrorNotFoundWithCode(err, common.CodeClassdayAbsenceReportNotFound)
	}},
	{Target: classday.ErrAbsenceReportInvalid, Render: common.ErrorInvalidRequest},
}, common.ErrorInternalServer)

// absenceReportTarget is what both routes read from the path and the token.
type absenceReportTarget struct {
	class     string
	date      classday.Date
	studentID int64
	actor     classday.AbsenceReportActor
}

// putAbsenceReport handles
// PUT /school/class-day/absence-reports/{schoolClass}/{date}/students/{studentId}.
func (rs *Resource) putAbsenceReport(w http.ResponseWriter, r *http.Request) {
	target, ok := rs.absenceReportTarget(w, r)
	if !ok {
		return
	}
	req := &AbsenceReportRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := rs.AbsenceReports.ReportAbsence(r.Context(), classday.AbsenceReportWrite{
		SchoolClass: target.class, Date: target.date, StudentID: target.studentID, Status: req.Status, Actor: target.actor,
	}); err != nil {
		common.RenderError(w, r, absenceReportErrorRenderer(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// deleteAbsenceReport handles
// DELETE /school/class-day/absence-reports/{schoolClass}/{date}/students/{studentId}.
func (rs *Resource) deleteAbsenceReport(w http.ResponseWriter, r *http.Request) {
	target, ok := rs.absenceReportTarget(w, r)
	if !ok {
		return
	}
	if err := rs.AbsenceReports.WithdrawAbsenceReport(r.Context(), classday.AbsenceReportWithdrawal{
		SchoolClass: target.class, Date: target.date, StudentID: target.studentID, Actor: target.actor,
	}); err != nil {
		common.RenderError(w, r, absenceReportErrorRenderer(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// absenceReportTarget resolves the class within the caller's reach, the date,
// the child and the acting account; it renders the refusal itself.
func (rs *Resource) absenceReportTarget(w http.ResponseWriter, r *http.Request) (absenceReportTarget, bool) {
	if rs.AbsenceReports == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errAbsenceReportsNotConfigured))
		return absenceReportTarget{}, false
	}
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("invalid token")))
		return absenceReportTarget{}, false
	}
	class, ok := rs.requireAssignedClass(w, r, chi.URLParam(r, "schoolClass"))
	if !ok {
		return absenceReportTarget{}, false
	}
	date, ok := parseArrivalExceptionDate(w, r, chi.URLParam(r, "date"))
	if !ok {
		return absenceReportTarget{}, false
	}
	studentID, err := strconv.ParseInt(strings.TrimSpace(chi.URLParam(r, "studentId")), 10, 64)
	if err != nil || studentID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid student id")))
		return absenceReportTarget{}, false
	}
	// The change history names the actor; a token without a name falls back
	// to the account's login.
	name := claims.DisplayName()
	if name == "" {
		name = claims.Sub
	}
	return absenceReportTarget{
		class: class, date: classday.Date(date.String()), studentID: studentID,
		actor: classday.AbsenceReportActor{AccountID: int64(claims.ID), Name: name},
	}, true
}
