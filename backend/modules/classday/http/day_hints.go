package classdayhttp

import (
	"context"
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

// The Lehrkraft's day hint for the OGS (#4795): a short message for today,
// for an assigned class or for one of its children. Every write passes the
// gates of the arrival exceptions in the same order — school scope, the
// class_day:day_hint_write permission, the school's setting
// operations.school_portal_write_scope, and the caller's
// education.class_teachers assignment — before the projection refuses any
// day but today and any child outside the class. The class assignment is
// checked here; the setting, the day and the child by the projection.

// DayHintRequest is the body of PUT /school/class-day/day-hints/{schoolClass}/{date}
// and of its per-child twin.
type DayHintRequest struct {
	Body string `json:"body"`
}

// Bind implements render.Binder. The length is checked by the projection, so
// the limit has one home.
func (r *DayHintRequest) Bind(_ *http.Request) error {
	if strings.TrimSpace(r.Body) == "" {
		return errors.New("body is required")
	}
	return nil
}

var dayHintErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: classday.ErrDayHintsClosed, Render: func(err error) render.Renderer {
		return common.ErrorForbiddenWithCode(err, common.CodeClassdaySchoolWriteDisabled)
	}},
	{Target: classday.ErrDayHintNotToday, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeClassdayDayHintNotToday)
	}},
	{Target: classday.ErrDayHintInvalid, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeClassdayDayHintInvalid)
	}},
	{Target: classday.ErrDayHintStudentNotInClass, Render: func(err error) render.Renderer {
		return common.ErrorNotFoundWithCode(err, common.CodeClassdayDayHintStudentNotInClass)
	}},
	{Target: classday.ErrDayHintClassEmpty, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeClassdayDayHintClassEmpty)
	}},
	{Target: classday.ErrDayHintNotFound, Render: common.ErrorNotFound},
}, common.ErrorInternalServer)

// dayHints is the write facade; without it every write answers "not
// configured" through the shared error renderer.
func (rs *Resource) dayHints() classday.DayHints {
	if rs.DayHints == nil {
		return unconfiguredDayHints{}
	}
	return rs.DayHints
}

type unconfiguredDayHints struct{}

func (unconfiguredDayHints) SetDayHint(context.Context, classday.DayHintWrite) error {
	return classday.ErrDayHintsNotConfigured
}

func (unconfiguredDayHints) ClearDayHint(context.Context, classday.DayHintTarget) error {
	return classday.ErrDayHintsNotConfigured
}

// requireDayHintWrite resolves the class against the caller's assignments.
// The school's setting, the day and the child are checked by the projection.
func (rs *Resource) requireDayHintWrite(w http.ResponseWriter, r *http.Request) (string, bool) {
	return rs.requireAssignedClass(w, r, chi.URLParam(r, "schoolClass"))
}

// dayHintStudentID reads the optional {studentId}; 0 is the class hint.
func dayHintStudentID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := chi.URLParam(r, "studentId")
	if raw == "" {
		return 0, true
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("ungültige Kind-ID")))
		return 0, false
	}
	return id, true
}

// putDayHint handles PUT /school/class-day/day-hints/{schoolClass}/{date}
// (the whole class) and .../students/{studentId} (one child).
func (rs *Resource) putDayHint(w http.ResponseWriter, r *http.Request) {
	class, ok := rs.requireDayHintWrite(w, r)
	if !ok {
		return
	}
	date, ok := parseArrivalExceptionDate(w, r, chi.URLParam(r, "date"))
	if !ok {
		return
	}
	studentID, ok := dayHintStudentID(w, r)
	if !ok {
		return
	}
	req := &DayHintRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeClassdayDayHintInvalid))
		return
	}
	err := rs.dayHints().SetDayHint(r.Context(), classday.DayHintWrite{
		SchoolClass: class, Date: classday.Date(date.String()), StudentID: studentID, Body: req.Body,
		AuthorAccountID: int64(sessiontoken.ClaimsFromCtx(r.Context()).ID),
	})
	if err != nil {
		common.RenderError(w, r, dayHintErrorRenderer(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// deleteDayHint handles DELETE on the same paths.
func (rs *Resource) deleteDayHint(w http.ResponseWriter, r *http.Request) {
	class, ok := rs.requireDayHintWrite(w, r)
	if !ok {
		return
	}
	date, ok := parseArrivalExceptionDate(w, r, chi.URLParam(r, "date"))
	if !ok {
		return
	}
	studentID, ok := dayHintStudentID(w, r)
	if !ok {
		return
	}
	err := rs.dayHints().ClearDayHint(r.Context(), classday.DayHintTarget{
		SchoolClass: class, Date: classday.Date(date.String()), StudentID: studentID,
		AuthorAccountID: int64(sessiontoken.ClaimsFromCtx(r.Context()).ID),
	})
	if err != nil {
		common.RenderError(w, r, dayHintErrorRenderer(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
