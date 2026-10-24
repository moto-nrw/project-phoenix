package timetablehttp

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Homework feedback from a Lernzeit block (#4796): the OGS, or a Lehrkraft
// running her own Lernzeit in moto schule, records per child "done",
// "not_done" or "difficult" with an optional short note.
//
// Authorization is the roster itself, as for the supervision sheet: Roster()
// re-checks the caller's access to the block, its verdict
// can_record_homework_feedback requires a Lernzeit block and the right to
// edit its attendance, and the child must be on the rows it returns. The
// roster rows are the same union requireRosterStudent bounds a per-child
// write to (planned children and walk-ins of the running session, minus
// graduates and ended care), applied here to every portal.

// HomeworkFeedbackRecorder stores a child's homework feedback; the Timetable
// owner's HomeworkBook is one.
type HomeworkFeedbackRecorder interface {
	RecordHomeworkFeedback(ctx context.Context, in timetable.HomeworkFeedbackInput) (timetable.HomeworkFeedback, error)
}

var (
	errHomeworkFeedbackNotAllowed  = errors.New("homework feedback is only possible in a Lernzeit block the caller may edit")
	errHomeworkFeedbackNotOnRoster = errors.New("this child is not part of this supervision")
)

// HomeworkFeedbackRequest is the body of PUT …/students/{student_id}/homework-feedback.
type HomeworkFeedbackRequest struct {
	Status string  `json:"status"`
	Note   *string `json:"note,omitempty"`
}

// Bind implements render.Binder; the service validates status and note.
func (r *HomeworkFeedbackRequest) Bind(_ *http.Request) error {
	if r.Status == "" {
		return errors.New("status is required")
	}
	return nil
}

// HomeworkFeedbackResponse is the stored feedback on the wire.
type HomeworkFeedbackResponse struct {
	StudentID int64   `json:"student_id"`
	Date      string  `json:"date"`
	Status    string  `json:"status"`
	Note      *string `json:"note,omitempty"`
}

var homeworkFeedbackErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: errHomeworkFeedbackNotAllowed, Render: func(err error) render.Renderer {
		return common.ErrorForbiddenWithCode(err, common.CodeTimetableHomeworkFeedbackNotAllowed)
	}},
	{Target: errHomeworkFeedbackNotOnRoster, Render: func(err error) render.Renderer {
		return common.ErrorForbiddenWithCode(err, common.CodeTimetableHomeworkFeedbackChildNotInSupervision)
	}},
	{Target: timetable.ErrHomeworkFeedbackWithoutHomework, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeTimetableHomeworkFeedbackWithoutHomework)
	}},
	{Target: timetable.ErrInvalidHomeworkFeedback, Render: common.ErrorInvalidRequest},
}, common.ErrorInternalServer)

// operationsPutHomeworkFeedback handles PUT …/instances/{id}/students/{student_id}/homework-feedback
// (tenant) and PUT /school/supervisions/{id}/students/{student_id}/homework-feedback.
func (rs *Resource) operationsPutHomeworkFeedback(w http.ResponseWriter, r *http.Request) {
	if rs.OperationsService == nil || rs.HomeworkFeedback == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("homework feedback is not wired")))
		return
	}
	instanceID, studentID, ok := parseOperationInstanceStudentIDs(w, r)
	if !ok {
		return
	}
	req := &HomeworkFeedbackRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	accountID, isAdmin := operationActor(r.Context())
	roster, err := rs.OperationsService.Roster(r.Context(), accountID, isAdmin, instanceID)
	if err != nil {
		rs.renderOperationsError(w, r, err)
		return
	}
	row, err := homeworkFeedbackRow(roster, studentID)
	if err != nil {
		common.RenderError(w, r, homeworkFeedbackErrorRenderer(err))
		return
	}
	saved, err := rs.HomeworkFeedback.RecordHomeworkFeedback(r.Context(), timetable.HomeworkFeedbackInput{
		StudentID: studentID, SchoolClass: row.SchoolClass, Date: calendar.Date(roster.Instance.Date),
		Status: req.Status, Note: req.Note,
	})
	if err != nil {
		common.RenderError(w, r, homeworkFeedbackErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, HomeworkFeedbackResponse{
		StudentID: saved.StudentID, Date: saved.Date.String(), Status: saved.Status, Note: saved.Note,
	}, "Homework feedback saved")
}

// homeworkFeedbackRow applies the roster's verdict and finds the child's row.
func homeworkFeedbackRow(roster *timetable.OperationRoster, studentID int64) (timetable.OperationRosterRow, error) {
	if roster == nil || !roster.CanRecordHomeworkFeedback {
		return timetable.OperationRosterRow{}, errHomeworkFeedbackNotAllowed
	}
	for _, row := range roster.Rows {
		if row.StudentID == studentID {
			return row, nil
		}
	}
	return timetable.OperationRosterRow{}, errHomeworkFeedbackNotOnRoster
}
