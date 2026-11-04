package classdayhttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// Morning attendance through "moto schule" (#4790): the Lehrkraft marks each
// child of an assigned class "da" or "fehlt" for today's school morning.
// Every write passes the school scope, the class_day:morning_attendance_write
// permission and the caller's education.class_teachers assignment here; the
// projection then checks the school's setting, today, the school day and the
// class roster. The marks are stored apart from the OGS attendance.

// maxMorningAttendanceMarks bounds one request; a class is far smaller.
const maxMorningAttendanceMarks = 200

// MorningAttendanceMarkRequest is one child's mark. student_id names a child
// with an OGS record, list_entry_id (a JSON string, like the day row) a
// class-list-only entry; exactly one is set.
type MorningAttendanceMarkRequest struct {
	StudentID   int64  `json:"student_id,omitempty"`
	ListEntryID int64  `json:"list_entry_id,string,omitempty"`
	Status      string `json:"status"`
}

// MorningAttendanceRequest is the body of
// PUT /school/class-day/morning-attendance/{schoolClass}/{date}.
type MorningAttendanceRequest struct {
	Marks []MorningAttendanceMarkRequest `json:"marks"`
}

// Bind implements render.Binder.
func (r *MorningAttendanceRequest) Bind(_ *http.Request) error {
	if len(r.Marks) == 0 {
		return errors.New("marks are required")
	}
	if len(r.Marks) > maxMorningAttendanceMarks {
		return errors.New("too many marks")
	}
	for _, mark := range r.Marks {
		if (mark.StudentID > 0) == (mark.ListEntryID > 0) || mark.StudentID < 0 || mark.ListEntryID < 0 {
			return errors.New("each mark needs exactly one of student_id and list_entry_id")
		}
		if mark.Status != classday.MorningAttendancePresent && mark.Status != classday.MorningAttendanceAbsent {
			return errors.New("status must be present or absent")
		}
	}
	return nil
}

var morningAttendanceErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: classday.ErrMorningAttendanceDisabled, Render: func(err error) render.Renderer {
		return common.ErrorForbiddenWithCode(err, common.CodeClassdaySchoolWriteDisabled)
	}},
	{Target: classday.ErrMorningAttendanceNotToday, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeClassdayMorningAttendanceNotToday)
	}},
	{Target: classday.ErrMorningAttendanceNoSchoolDay, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeClassdayMorningAttendanceNoSchoolDay)
	}},
	{Target: classday.ErrMorningAttendanceUnknownChild, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeClassdayMorningAttendanceUnknownChild)
	}},
	{Target: classday.ErrMorningAttendanceInvalid, Render: common.ErrorInvalidRequest},
}, common.ErrorInternalServer)

// putMorningAttendance handles PUT /school/class-day/morning-attendance/{schoolClass}/{date}.
func (rs *Resource) putMorningAttendance(w http.ResponseWriter, r *http.Request) {
	class, ok := rs.requireAssignedClass(w, r, chi.URLParam(r, "schoolClass"))
	if !ok {
		return
	}
	date, ok := parseArrivalExceptionDate(w, r, chi.URLParam(r, "date"))
	if !ok {
		return
	}
	req := &MorningAttendanceRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	staffID, ok := rs.requireCurrentStaffID(w, r)
	if !ok {
		return
	}
	marks := make([]classday.MorningAttendanceMark, 0, len(req.Marks))
	for _, mark := range req.Marks {
		marks = append(marks, classday.MorningAttendanceMark{StudentID: mark.StudentID, ListEntryID: mark.ListEntryID, Status: mark.Status})
	}
	if err := rs.ClassDay.RecordMorningAttendance(r.Context(), classday.MorningAttendanceWrite{
		SchoolClass: class, Date: classday.Date(date.String()), RecordedBy: staffID, Marks: marks,
		ActorAccountID: int64(sessiontoken.ClaimsFromCtx(r.Context()).ID),
	}); err != nil {
		common.RenderError(w, r, morningAttendanceErrorRenderer(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
