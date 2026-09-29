package students

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// The note routes turn a request into an owner command. What is checked here
// is that translation: the defaults a missing field falls back to and the
// payloads the route refuses before the owner ever sees them.
//
// Who may read or remove a note is decided in auth/authorize (covered by
// auth/authorize/student_notes_test.go) and enforced by the owner against the
// real table (modules/peopledirectory/compose); this adapter test may not
// reach into either.

func noteBody(t *testing.T, payload string) (studentNoteRequestBody, int) {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/students/1/notes", strings.NewReader(payload))
	body, ok := decodeStudentNoteBody(w, req)
	if !ok {
		return studentNoteRequestBody{}, w.Code
	}
	return body, http.StatusOK
}

func TestDecodeStudentNoteBodyDefaultsAndRejects(t *testing.T) {
	t.Parallel()

	t.Run("an entry without kind or visibility is the narrowest sensible default", func(t *testing.T) {
		t.Parallel()
		body, status := noteBody(t, `{"body":"Kurze Notiz."}`)
		require.Equal(t, http.StatusOK, status)
		assert.Equal(t, peopleModule.StudentNoteKindJournal, body.Kind,
			"an unspecified note is an entry, not a durable hint")
		assert.Equal(t, peopleModule.StudentNoteVisibilityAllStaff, body.Visibility)
	})

	for _, payload := range []struct{ name, json string }{
		{"unknown visibility", `{"body":"x","visibility":"everyone"}`},
		{"unknown kind", `{"body":"x","kind":"forever"}`},
		{"unknown category", `{"body":"x","category":"gossip"}`},
		{"malformed json", `{"body":`},
	} {
		t.Run(payload.name+" is refused", func(t *testing.T) {
			t.Parallel()
			_, status := noteBody(t, payload.json)
			assert.Equal(t, http.StatusBadRequest, status)
		})
	}
}

func TestParseNoteSubject(t *testing.T) {
	t.Parallel()

	activity, group := "7", "9"

	t.Run("one occurrence is an activity and a day", func(t *testing.T) {
		t.Parallel()
		date := "2026-09-09"
		subject, err := parseNoteSubject(studentNoteRequestBody{
			SubjectDate: &date, ActivityGroupID: &activity,
		})
		require.NoError(t, err)
		require.NotNil(t, subject.Date)
		assert.Equal(t, "2026-09-09", subject.Date.String())
		require.NotNil(t, subject.ActivityGroupID)
		assert.Equal(t, int64(7), *subject.ActivityGroupID)
		assert.Nil(t, subject.EducationGroupID)
	})

	t.Run("two group references are refused before the database sees them", func(t *testing.T) {
		t.Parallel()
		_, err := parseNoteSubject(studentNoteRequestBody{
			ActivityGroupID: &activity, EducationGroupID: &group,
		})
		require.ErrorContains(t, err, "not to both")
	})

	t.Run("a day that is not a calendar date is refused", func(t *testing.T) {
		t.Parallel()
		broken := "09.09.2026"
		_, err := parseNoteSubject(studentNoteRequestBody{SubjectDate: &broken})
		require.ErrorContains(t, err, "YYYY-MM-DD")
	})

	t.Run("an empty reference is no reference", func(t *testing.T) {
		t.Parallel()
		empty := ""
		subject, err := parseNoteSubject(studentNoteRequestBody{
			SubjectDate: &empty, ActivityGroupID: &empty, EducationGroupID: &empty,
		})
		require.NoError(t, err)
		assert.Nil(t, subject.Date)
		assert.Nil(t, subject.ActivityGroupID)
		assert.Nil(t, subject.EducationGroupID)
	})

	t.Run("a non-numeric reference is refused", func(t *testing.T) {
		t.Parallel()
		broken := "abc"
		_, err := parseNoteSubject(studentNoteRequestBody{ActivityGroupID: &broken})
		require.ErrorContains(t, err, "activity_group_id")
	})
}
