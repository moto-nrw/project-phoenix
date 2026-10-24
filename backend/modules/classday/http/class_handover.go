package classdayhttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/classday"
)

// Class handover through "moto schule" (#4794): after lessons the Lehrkraft
// hands an assigned class over to the OGS, marks the children with an OGS
// record who are not there, and may take the handover back the same day.
// Every write passes the school scope, the class_day:handover_write
// permission and the caller's education.class_teachers assignment here; the
// projection then checks the write scope, today, the school day and the
// class roster.

// maxHandoverChildren bounds one request; a class is far smaller.
const maxHandoverChildren = 200

// ClassHandoverRequest is the body of
// PUT /school/class-day/handover/{schoolClass}/{date}: the children handed
// over "da" and "nicht da", either list possibly empty.
type ClassHandoverRequest struct {
	PresentStudentIDs []int64 `json:"present_student_ids"`
	AbsentStudentIDs  []int64 `json:"absent_student_ids"`
}

// Bind implements render.Binder.
func (r *ClassHandoverRequest) Bind(_ *http.Request) error {
	if len(r.PresentStudentIDs)+len(r.AbsentStudentIDs) > maxHandoverChildren {
		return errors.New("too many children")
	}
	for _, id := range append(append([]int64{}, r.PresentStudentIDs...), r.AbsentStudentIDs...) {
		if id <= 0 {
			return errors.New("student ids must be positive")
		}
	}
	return nil
}

var classHandoverErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: classday.ErrClassHandoverDisabled, Render: func(err error) render.Renderer {
		return common.ErrorForbiddenWithCode(err, common.CodeClassdaySchoolWriteDisabled)
	}},
	{Target: classday.ErrClassHandoverNotToday, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeClassdayClassHandoverNotToday)
	}},
	{Target: classday.ErrClassHandoverNoSchoolDay, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeClassdayClassHandoverNoSchoolDay)
	}},
	{Target: classday.ErrClassHandoverUnknownChild, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeClassdayClassHandoverUnknownChild)
	}},
	{Target: classday.ErrClassHandoverNotFound, Render: func(err error) render.Renderer {
		return common.ErrorNotFoundWithCode(err, common.CodeClassdayClassHandoverNotFound)
	}},
	{Target: classday.ErrClassHandoverInvalid, Render: common.ErrorInvalidRequest},
}, common.ErrorInternalServer)

// putClassHandover handles PUT /school/class-day/handover/{schoolClass}/{date}.
func (rs *Resource) putClassHandover(w http.ResponseWriter, r *http.Request) {
	req := &ClassHandoverRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	rs.changeClassHandover(w, r, classday.ClassHandoverChange{PresentStudentIDs: req.PresentStudentIDs, AbsentStudentIDs: req.AbsentStudentIDs})
}

// deleteClassHandover handles DELETE /school/class-day/handover/{schoolClass}/{date}.
func (rs *Resource) deleteClassHandover(w http.ResponseWriter, r *http.Request) {
	rs.changeClassHandover(w, r, classday.ClassHandoverChange{Withdraw: true})
}

// changeClassHandover resolves the assigned class, the date and the caller's
// staff row into the change and answers 204.
func (rs *Resource) changeClassHandover(w http.ResponseWriter, r *http.Request, change classday.ClassHandoverChange) {
	class, ok := rs.requireAssignedClass(w, r, chi.URLParam(r, "schoolClass"))
	if !ok {
		return
	}
	date, ok := parseArrivalExceptionDate(w, r, chi.URLParam(r, "date"))
	if !ok {
		return
	}
	staffID, ok := rs.requireCurrentStaffID(w, r)
	if !ok {
		return
	}
	change.SchoolClass, change.Date, change.HandedOverBy = class, classday.Date(date.String()), staffID
	if err := rs.ClassDay.ChangeClassHandover(r.Context(), change); err != nil {
		common.RenderError(w, r, classHandoverErrorRenderer(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
