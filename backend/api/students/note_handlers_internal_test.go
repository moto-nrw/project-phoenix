package students

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/models/users"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// The note routes turn a request into an owner command and a decision into
// affordances. The owner's own rules are covered against the database in
// modules/peopledirectory/compose; what is checked here is the translation —
// the part where a wrong mapping would silently widen an audience or offer a
// button the next request refuses.

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

// The filter is where the policy's facts become the owner's audience. A missing
// branch here is a leak or a blank timeline, and neither shows up as an error.
func TestToModuleAudience(t *testing.T) {
	t.Parallel()

	t.Run("a colleague without the child gets the team notes only", func(t *testing.T) {
		t.Parallel()
		filter := toModuleAudience(authorize.StudentNoteAudience{}, 12)
		assert.Equal(t, []string{peopleModule.StudentNoteVisibilityAllStaff}, filter.Visibilities)
		assert.Equal(t, int64(12), filter.ReaderAccountID)
	})

	t.Run("the care team adds its own audience", func(t *testing.T) {
		t.Parallel()
		filter := toModuleAudience(authorize.StudentNoteAudience{CareTeam: true}, 12)
		assert.Equal(t, []string{
			peopleModule.StudentNoteVisibilityAllStaff,
			peopleModule.StudentNoteVisibilityCareTeam,
		}, filter.Visibilities)
		assert.Empty(t, filter.LedEducationGroupIDs,
			"belonging to the care team does not make anyone a group lead")
	})

	t.Run("an admin reaches every audience", func(t *testing.T) {
		t.Parallel()
		filter := toModuleAudience(authorize.StudentNoteAudience{Admin: true, CareTeam: true}, 12)
		assert.Equal(t, []string{
			peopleModule.StudentNoteVisibilityAllStaff,
			peopleModule.StudentNoteVisibilityCareTeam,
			peopleModule.StudentNoteVisibilityGroupLeads,
		}, filter.Visibilities)
	})

	t.Run("led groups travel to the store", func(t *testing.T) {
		t.Parallel()
		filter := toModuleAudience(authorize.StudentNoteAudience{
			LedEducationGroupIDs: []int64{3}, LedActivityGroupIDs: []int64{4},
		}, 12)
		assert.Equal(t, []int64{3}, filter.LedEducationGroupIDs)
		assert.Equal(t, []int64{4}, filter.LedActivityGroupIDs)
	})
}

// can_edit and can_delete are what the client renders buttons from. They must
// agree with what the next request would actually allow.
func TestBuildNoteResponseAffordances(t *testing.T) {
	t.Parallel()

	var author, colleague, groupID int64 = 5, 6, 3
	student := &users.Student{GroupID: &groupID}
	note := peopleModule.StudentNote{
		ID: 1, StudentID: 2, AuthorAccountID: &author,
		Origin:     peopleModule.StudentNoteOriginStaff,
		Kind:       peopleModule.StudentNoteKindJournal,
		Visibility: peopleModule.StudentNoteVisibilityAllStaff,
		Body:       "Eine Beobachtung.", AuthorName: "Sara Betreuerin",
	}

	t.Run("the author may correct, a colleague may not", func(t *testing.T) {
		t.Parallel()
		mine := buildNoteResponse(note, authorize.StudentNoteAudience{}, student, author)
		assert.True(t, mine.CanEdit)
		assert.Equal(t, "Sara Betreuerin", mine.AuthorName)
		assert.Equal(t, "1", mine.ID, "IDs travel as strings")

		theirs := buildNoteResponse(note, authorize.StudentNoteAudience{}, student, colleague)
		assert.False(t, theirs.CanEdit)
	})

	t.Run("a carried-over hint is nobody's to reword", func(t *testing.T) {
		t.Parallel()
		carried := note
		carried.Origin = peopleModule.StudentNoteOriginMasterData
		response := buildNoteResponse(carried, authorize.StudentNoteAudience{}, student, author)
		assert.False(t, response.CanEdit,
			"the author account on a carried-over hint does not make it editable")
	})

	t.Run("only the leadership of the child's group may remove a general note", func(t *testing.T) {
		t.Parallel()
		none := buildNoteResponse(note, authorize.StudentNoteAudience{}, student, author)
		assert.False(t, none.CanDelete, "its own author may not remove it")

		lead := buildNoteResponse(note, authorize.StudentNoteAudience{
			LedEducationGroupIDs: []int64{groupID},
		}, student, colleague)
		assert.True(t, lead.CanDelete)
	})
}
