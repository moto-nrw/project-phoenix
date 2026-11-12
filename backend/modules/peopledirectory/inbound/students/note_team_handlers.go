package students

import (
	context "context"
	errors "errors"
	http "net/http"
	strconv "strconv"
	time "time"

	common "github.com/moto-nrw/project-phoenix/api/common"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// StudentNoteTeamReach names the colleagues who may read a child's notes and
// tells them on their devices (#6854). The root binds it over the staff
// directory and Delivery's staff notifications; both are built after this
// owner, so they arrive here and not in its composition.
type StudentNoteTeamReach interface {
	// NoteReaderAccountIDs lists the active staff accounts of the request's
	// school that may read children's notes.
	NoteReaderAccountIDs(context.Context) ([]int64, error)
	// NotifyStudentNoteShared enqueues the device notification in the
	// caller's transaction. A school without notifications, or a time outside
	// its notification window, is no error: the unread marker still reaches
	// everyone.
	NotifyStudentNoteShared(context.Context, StudentNoteSharedPush) error
}

// StudentNoteSharedPush is the device notification of one shared note. It
// names no child: the text shows on a locked screen, the link opens the note.
type StudentNoteSharedPush struct {
	StudentID           int64
	NoteID              int64
	RecipientAccountIDs []int64
	Title               string
	Body                string
	DeepLink            string
}

const (
	studentNoteSharedTitle = "Notiz für das Team"
	studentNoteSharedBody  = "Jemand aus dem Team hat eine Notiz zu einem Kind mit Ihnen geteilt."
)

// studentNotesDeepLink opens the child's notes, where the marker clears.
func studentNotesDeepLink(studentID int64) string {
	return "/students/" + strconv.FormatInt(studentID, 10) + "?tab=notizen"
}

type studentNoteTeamInformedResponse struct {
	// Informed is how many colleagues were told. Zero means nobody else in
	// the team may read the child's notes.
	Informed int `json:"informed"`
}

type studentNoteTeamNoticeResponse struct {
	NoteID           string    `json:"note_id"`
	StudentID        string    `json:"student_id"`
	StudentFirstName string    `json:"student_first_name"`
	StudentLastName  string    `json:"student_last_name"`
	AuthorName       string    `json:"author_name,omitempty"`
	InformedAt       time.Time `json:"informed_at"`
}

var errStudentNoteTeamUnbound = errors.New("student note team capability is not configured")

// informStudentNoteTeam is "Speichern und Team informieren": after the note
// is saved, the author tells the colleagues. The markers and the device
// notification are stored together; a failure leaves neither, and the
// response says so.
func (rs *Resource) informStudentNoteTeam(w http.ResponseWriter, r *http.Request) {
	if rs.StudentNoteTeam == nil || rs.NoteTeamReach == nil {
		renderError(w, r, common.ErrorInternalServer(errStudentNoteTeamUnbound))
		return
	}
	student, _, _, ok := rs.resolveNoteReader(w, r)
	if !ok {
		return
	}
	noteID, ok := common.ParsePositiveInt64IDWithError(w, r, "noteId", "invalid note id")
	if !ok {
		return
	}
	recipients, err := rs.NoteTeamReach.NoteReaderAccountIDs(r.Context())
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	informed, err := rs.StudentNoteTeam.InformStudentNoteTeam(r.Context(), peopleModule.InformStudentNoteTeam{
		NoteID: noteID, StudentID: student.ID, ActorAccountID: callerAccountID(r),
		RecipientAccountIDs: recipients,
		Notify: func(ctx context.Context, told []int64) error {
			return rs.NoteTeamReach.NotifyStudentNoteShared(ctx, StudentNoteSharedPush{
				StudentID: student.ID, NoteID: noteID, RecipientAccountIDs: told,
				Title: studentNoteSharedTitle, Body: studentNoteSharedBody, DeepLink: studentNotesDeepLink(student.ID),
			})
		},
	})
	if err != nil {
		renderError(w, r, studentNoteErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, studentNoteTeamInformedResponse{Informed: informed}, "Student note team informed")
}

// listStudentNoteTeamNotices serves the reader's unread markers for the
// header. They reach only the reader they were stored for.
func (rs *Resource) listStudentNoteTeamNotices(w http.ResponseWriter, r *http.Request) {
	if rs.StudentNoteTeam == nil {
		renderError(w, r, common.ErrorInternalServer(errStudentNoteTeamUnbound))
		return
	}
	notices, err := rs.StudentNoteTeam.ListUnreadStudentNoteTeamNotices(r.Context(), callerAccountID(r))
	if err != nil {
		renderError(w, r, studentNoteErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, renderStudentNoteTeamNotices(notices), "Student note team notices retrieved")
}

// readStudentNoteTeamNotices clears the reader's markers for one child once
// the child's notes are open, and answers with the markers still unread.
func (rs *Resource) readStudentNoteTeamNotices(w http.ResponseWriter, r *http.Request) {
	if rs.StudentNoteTeam == nil {
		renderError(w, r, common.ErrorInternalServer(errStudentNoteTeamUnbound))
		return
	}
	studentID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid student id")
	if !ok {
		return
	}
	notices, err := rs.StudentNoteTeam.MarkStudentNoteTeamNoticesRead(r.Context(), callerAccountID(r), studentID)
	if err != nil {
		renderError(w, r, studentNoteErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, renderStudentNoteTeamNotices(notices), "Student note team notices read")
}

func renderStudentNoteTeamNotices(notices []peopleModule.StudentNoteTeamNotice) []studentNoteTeamNoticeResponse {
	out := make([]studentNoteTeamNoticeResponse, 0, len(notices))
	for _, notice := range notices {
		out = append(out, studentNoteTeamNoticeResponse{
			NoteID: strconv.FormatInt(notice.NoteID, 10), StudentID: strconv.FormatInt(notice.StudentID, 10),
			StudentFirstName: notice.StudentFirstName, StudentLastName: notice.StudentLastName,
			AuthorName: notice.AuthorName, InformedAt: notice.InformedAt,
		})
	}
	return out
}
