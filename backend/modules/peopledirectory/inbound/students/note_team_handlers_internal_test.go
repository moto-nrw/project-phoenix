package students

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	peopleModule "github.com/moto-ogs/project-phoenix/modules/peopledirectory"
)

// The share route answers a refusal with the status the client can act on:
// a note narrower than the whole team is a request it should not have sent,
// somebody else's note is forbidden, and an unbound capability is a server
// error, never a silent success.
func TestStudentNoteTeamRefusalsKeepTheirStatus(t *testing.T) {
	t.Parallel()
	for err, status := range map[error]int{
		peopleModule.ErrStudentNoteTeamAudience: http.StatusBadRequest,
		peopleModule.ErrStudentNoteNotAuthor:    http.StatusForbidden,
		peopleModule.ErrStudentNoteNotFound:     http.StatusNotFound,
	} {
		w := httptest.NewRecorder()
		renderError(w, httptest.NewRequest(http.MethodPost, "/1/notes/2/team-notices", nil), studentNoteErrorRenderer(err))
		assert.Equal(t, status, w.Code, err.Error())
	}

	w := httptest.NewRecorder()
	(&Resource{}).informStudentNoteTeam(w, httptest.NewRequest(http.MethodPost, "/1/notes/2/team-notices", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
