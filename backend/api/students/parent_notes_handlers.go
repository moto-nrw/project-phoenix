package students

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// parentNotesStaffLimit caps how many of the newest parent notes the
// staff student view surfaces.
const parentNotesStaffLimit = 3

// ParentNoteResponse is the staff-facing wire shape for a parent note.
// IDs are stringified per the int64 → string frontend convention.
type ParentNoteResponse struct {
	ID        string    `json:"id"`
	StudentID string    `json:"student_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// getStudentParentNotes returns the newest notes a guardian left for the
// student via the parents portal. Read access mirrors status-days: full
// access to the student is required (these are guardian messages about a
// specific child).
func (rs *Resource) getStudentParentNotes(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}
	if !rs.checkStudentReadAccess(r, student) {
		renderError(w, r, ErrorForbidden(errors.New("full access required")))
		return
	}
	if rs.StudentParentNoteRepo == nil {
		common.Respond(w, r, http.StatusOK, []ParentNoteResponse{}, "Parent notes retrieved successfully")
		return
	}

	notes, err := rs.StudentParentNoteRepo.ListByStudent(r.Context(), student.ID, parentNotesStaffLimit)
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to fetch parent notes", err))
		return
	}

	out := make([]ParentNoteResponse, 0, len(notes))
	for _, n := range notes {
		out = append(out, ParentNoteResponse{
			ID:        strconv.FormatInt(n.ID, 10),
			StudentID: strconv.FormatInt(n.StudentID, 10),
			Body:      n.Body,
			CreatedAt: n.CreatedAt,
		})
	}
	common.Respond(w, r, http.StatusOK, out, "Parent notes retrieved successfully")
}
