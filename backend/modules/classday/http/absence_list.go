package classdayhttp

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// Absence list of moto schule (#4793): per child of a class the absence days
// of a period, excused and unexcused, for the Klassenbuch and the Zeugnis.
// Both routes need class_day:read and resolve the class within the caller's
// reach: the Lehrkraft's assigned classes, every class for the holders of
// class_day:all_classes. The projection logs every read and export.

// absenceListClientErrors are the refusals of a bad request: they answer 400
// and are not logged as failures.
var absenceListClientErrors = []error{classday.ErrAbsenceListInvalidPeriod, classday.ErrInvalidReportFilter}

var absenceListErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: absenceListClientErrors[0], Render: common.ErrorInvalidRequest},
	{Target: absenceListClientErrors[1], Render: common.ErrorInvalidRequest},
}, common.ErrorInternalServer)

// getAbsenceList handles
// GET /school/class-day/absence-list?class=…&from=YYYY-MM-DD&to=YYYY-MM-DD.
// Without from and to the period is the current school half-year.
func (rs *Resource) getAbsenceList(w http.ResponseWriter, r *http.Request) {
	query, ok := rs.absenceListQuery(w, r)
	if !ok {
		return
	}
	report, err := rs.AbsenceList.AbsenceListReport(r.Context(), query)
	if err != nil {
		rs.renderAbsenceListError(w, r, query, err)
		return
	}
	common.Respond(w, r, http.StatusOK, report, "Absence list retrieved successfully")
}

// exportAbsenceList handles
// GET /school/class-day/absence-list/export?class=…&from=…&to=…&format=pdf|xlsx.
// A missing format renders the PDF.
func (rs *Resource) exportAbsenceList(w http.ResponseWriter, r *http.Request) {
	format := classday.Format(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = classday.FormatPDF
	}
	if format != classday.FormatPDF && format != classday.FormatXLSX {
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("unsupported export format %q", format)))
		return
	}
	query, ok := rs.absenceListQuery(w, r)
	if !ok {
		return
	}
	file, err := rs.AbsenceList.RenderAbsenceList(r.Context(), query, format)
	if err != nil {
		rs.renderAbsenceListError(w, r, query, err)
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

// absenceListQuery resolves the class within the caller's reach and reads the
// period; it renders the refusal itself.
func (rs *Resource) absenceListQuery(w http.ResponseWriter, r *http.Request) (classday.AbsenceListQuery, bool) {
	if rs.AbsenceList == nil {
		common.RenderError(w, r, common.ErrorInternalServer(classday.ErrAbsenceListNotConfigured))
		return classday.AbsenceListQuery{}, false
	}
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("invalid token")))
		return classday.AbsenceListQuery{}, false
	}
	class, ok := rs.requireAssignedClass(w, r, r.URL.Query().Get("class"))
	if !ok {
		return classday.AbsenceListQuery{}, false
	}
	return classday.AbsenceListQuery{
		SchoolClass: class,
		From:        classday.Date(strings.TrimSpace(r.URL.Query().Get("from"))),
		To:          classday.Date(strings.TrimSpace(r.URL.Query().Get("to"))),
		Actor:       classday.Actor{AccountID: int64(claims.ID), Roles: strings.Join(claims.Roles, ",")},
	}, true
}

func (rs *Resource) renderAbsenceListError(w http.ResponseWriter, r *http.Request, query classday.AbsenceListQuery, err error) {
	if !slices.ContainsFunc(absenceListClientErrors, func(target error) bool { return errors.Is(err, target) }) {
		rs.logger.Error("class day: absence list failed",
			"school_class", query.SchoolClass,
			"error", err.Error(),
		)
	}
	common.RenderError(w, r, absenceListErrorRenderer(err))
}
