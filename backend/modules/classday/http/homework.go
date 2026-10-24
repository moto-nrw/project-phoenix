package classdayhttp

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/classday"
)

// Homework through "moto schule" (#4796): the Lehrkraft enters what an
// assigned class has to do on a day; subject optional, text required. Every
// write passes the school scope, the class_day:homework_write permission and
// the caller's education.class_teachers assignment here. Changes and deletes
// reach only the caller's own entries. No school setting gates the write:
// homework is the Lehrkraft's own entry and changes no OGS record.

// HomeworkRequest is the body of the homework writes. Date is required only
// when an entry is created.
type HomeworkRequest struct {
	Date        string  `json:"date,omitempty"`
	Subject     *string `json:"subject,omitempty"`
	Description string  `json:"description"`
}

// Bind implements render.Binder. The length limits are checked behind the
// projection and answer 400 like this check.
func (r *HomeworkRequest) Bind(_ *http.Request) error {
	if strings.TrimSpace(r.Description) == "" {
		return errors.New("description is required")
	}
	return nil
}

var homeworkErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: classday.ErrHomeworkNotFound, Render: func(err error) render.Renderer {
		return common.ErrorNotFoundWithCode(err, common.CodeClassdayHomeworkNotFound)
	}},
	{Target: classday.ErrHomeworkInvalid, Render: common.ErrorInvalidRequest},
}, common.ErrorInternalServer)

// homeworkWriteTarget runs the gates every homework write shares: the class
// assignment, the body and the caller's staff row.
func (rs *Resource) homeworkWriteTarget(w http.ResponseWriter, r *http.Request) (string, *HomeworkRequest, int64, bool) {
	class, ok := rs.requireAssignedClass(w, r, chi.URLParam(r, "schoolClass"))
	if !ok {
		return "", nil, 0, false
	}
	req := &HomeworkRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return "", nil, 0, false
	}
	staffID, ok := rs.requireCurrentStaffID(w, r)
	return class, req, staffID, ok
}

// postHomework handles POST /school/class-day/homework/{schoolClass}.
func (rs *Resource) postHomework(w http.ResponseWriter, r *http.Request) {
	class, req, staffID, ok := rs.homeworkWriteTarget(w, r)
	if !ok {
		return
	}
	date, ok := parseArrivalExceptionDate(w, r, req.Date)
	if !ok {
		return
	}
	entry, err := rs.ClassDay.AddHomework(r.Context(), classday.HomeworkWrite{
		SchoolClass: class, Date: classday.Date(date.String()), Subject: req.Subject, Description: req.Description, StaffID: staffID,
	})
	if err != nil {
		common.RenderError(w, r, homeworkErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusCreated, entry, "Homework saved successfully")
}

// putHomework handles PUT /school/class-day/homework/{schoolClass}/{homeworkId}.
func (rs *Resource) putHomework(w http.ResponseWriter, r *http.Request) {
	id, ok := parseHomeworkID(w, r)
	if !ok {
		return
	}
	class, req, staffID, ok := rs.homeworkWriteTarget(w, r)
	if !ok {
		return
	}
	entry, err := rs.ClassDay.ChangeHomework(r.Context(), classday.HomeworkChange{
		ID: id, SchoolClass: class, Subject: req.Subject, Description: req.Description, StaffID: staffID,
	})
	if err != nil {
		common.RenderError(w, r, homeworkErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, entry, "Homework saved successfully")
}

// deleteHomework handles DELETE /school/class-day/homework/{schoolClass}/{homeworkId}.
func (rs *Resource) deleteHomework(w http.ResponseWriter, r *http.Request) {
	id, ok := parseHomeworkID(w, r)
	if !ok {
		return
	}
	class, ok := rs.requireAssignedClass(w, r, chi.URLParam(r, "schoolClass"))
	if !ok {
		return
	}
	staffID, ok := rs.requireCurrentStaffID(w, r)
	if !ok {
		return
	}
	if err := rs.ClassDay.DeleteHomework(r.Context(), id, class, staffID); err != nil {
		common.RenderError(w, r, homeworkErrorRenderer(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseHomeworkID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return common.ParsePositiveInt64IDWithError(w, r, "homeworkId", "invalid homework id")
}
