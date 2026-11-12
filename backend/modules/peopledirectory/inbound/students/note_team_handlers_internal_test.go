package students

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

type noteTeamReadCapture struct {
	peopleModule.StudentNoteTeamNotices
	observed []peopleModule.StudentNoteTeamNoticeRevision
	called   bool
}

func (c *noteTeamReadCapture) MarkStudentNoteTeamNoticesRead(_ context.Context, _, _ int64, observed []peopleModule.StudentNoteTeamNoticeRevision) ([]peopleModule.StudentNoteTeamNotice, error) {
	c.called, c.observed = true, observed
	return nil, nil
}

func TestStudentNoteTeamReadPreservesAnnouncementRevisions(t *testing.T) {
	t.Parallel()
	for _, payload := range []struct {
		name, body string
		status     int
	}{
		{"observed revision", `{"notices":[{"note_id":"9007199254740993","informed_at":"2026-09-09T10:00:00.123456Z"}]}`, http.StatusOK},
		{"malformed timestamp", `{"notices":[{"note_id":"42","informed_at":"not-a-time"}]}`, http.StatusBadRequest},
		{"malformed ID", `{"notices":[{"note_id":"not-an-id","informed_at":"2026-09-09T10:00:00Z"}]}`, http.StatusBadRequest},
	} {
		t.Run(payload.name, func(t *testing.T) {
			t.Parallel()
			capture := &noteTeamReadCapture{}
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/42/team-notices/read", strings.NewReader(payload.body))
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", "42")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
			(&Resource{StudentNoteTeam: capture}).readStudentNoteTeamNotices(w, req)
			require.Equal(t, payload.status, w.Code)
			assert.Equal(t, payload.status == http.StatusOK, capture.called)
			if capture.called {
				require.Len(t, capture.observed, 1)
				assert.EqualValues(t, 9007199254740993, capture.observed[0].NoteID)
				assert.Equal(t, time.Date(2026, 9, 9, 10, 0, 0, 123456000, time.UTC), capture.observed[0].InformedAt)
			}
		})
	}
}
