package students

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	securitycontract "github.com/moto-nrw/project-phoenix/modules/securityruntime/ports"

	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	t.Run("a future day is refused", func(t *testing.T) {
		t.Parallel()
		future := calendar.TodayDate().AddDays(1).String()
		_, err := parseNoteSubject(studentNoteRequestBody{SubjectDate: &future})
		require.ErrorContains(t, err, "must not be in the future")
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

func TestValidateNoteSubjectForChild(t *testing.T) {
	t.Parallel()

	groupID, activityID := int64(9), int64(7)
	child := securitycontract.StudentNoteChild{
		GroupID: &groupID, ActivityGroupIDs: []int64{activityID},
	}

	tests := []struct {
		name    string
		subject peopleModule.StudentNoteSubject
		wantErr string
	}{
		{name: "the child's education group", subject: peopleModule.StudentNoteSubject{EducationGroupID: &groupID}},
		{name: "the child's activity", subject: peopleModule.StudentNoteSubject{ActivityGroupID: &activityID}},
		{name: "another education group", subject: peopleModule.StudentNoteSubject{EducationGroupID: int64Pointer(10)}, wantErr: "education_group_id"},
		{name: "another activity", subject: peopleModule.StudentNoteSubject{ActivityGroupID: int64Pointer(8)}, wantErr: "activity_group_id"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := validateNoteSubjectForChild(test.subject, child)
			if test.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

func TestBuildNoteResponseDoesNotOfferEditingForOrphanedLeadershipNote(t *testing.T) {
	t.Parallel()

	author := int64(17)
	date := calendar.TodayDate()
	note := peopleModule.StudentNote{
		ID: 1, StudentID: 2, AuthorAccountID: &author, Origin: peopleModule.StudentNoteOriginStaff,
		Kind: peopleModule.StudentNoteKindJournal, Visibility: peopleModule.StudentNoteVisibilityGroupLeads,
		Body: "Der Bezug wurde gelöscht.", Subject: peopleModule.StudentNoteSubject{Date: &date},
	}

	groupID := int64(19)
	response := buildNoteResponse(note, securitycontract.StudentNoteAudience{
		LedEducationGroupIDs: []int64{groupID},
	}, &peopleModule.StudentState{GroupID: &groupID}, author)
	assert.False(t, response.CanEdit,
		"the response must not offer an edit that the owner rejects without a group reference")
	assert.False(t, response.CanDelete,
		"the response must not offer deletion of a retained leadership note to a current group leader")
}

// A day hint from moto schule (#4795) is withdrawn only there, so the child's
// record offers neither editing nor deletion, not even to an administrator.
func TestBuildNoteResponseKeepsSchoolDayHintReadOnly(t *testing.T) {
	t.Parallel()

	author := int64(17)
	date := calendar.NewDate(2099, 3, 2)
	note := peopleModule.StudentNote{
		ID: 1, StudentID: 2, AuthorAccountID: &author, Origin: peopleModule.StudentNoteOriginSchool,
		Kind: peopleModule.StudentNoteKindJournal, Visibility: peopleModule.StudentNoteVisibilityAllStaff,
		Body: "Heute ist Sportfest.", Subject: peopleModule.StudentNoteSubject{Date: &date},
	}
	groupID := int64(19)
	for _, audience := range []securitycontract.StudentNoteAudience{
		{Admin: true},
		{LedEducationGroupIDs: []int64{groupID}},
	} {
		response := buildNoteResponse(note, audience, &peopleModule.StudentState{GroupID: &groupID}, author)
		assert.False(t, response.CanEdit)
		assert.False(t, response.CanDelete)
	}
}

func int64Pointer(value int64) *int64 { return &value }

func TestBuildNoteSubjectsResponse(t *testing.T) {
	t.Parallel()

	response := buildNoteSubjectsResponse([]peopleModule.StudentActivityGroup{
		{ID: 7, Name: "Fußball-AG"}, {ID: 12, Name: "Schach"},
	})
	assert.Equal(t, []noteSubjectActivity{
		{ID: "7", Name: "Fußball-AG"}, {ID: "12", Name: "Schach"},
	}, response.Activities)

	// A child without enrollments still answers with an empty list, so the
	// picker renders "no activity" instead of failing on a null.
	empty := buildNoteSubjectsResponse(nil)
	require.NotNil(t, empty.Activities)
	assert.Empty(t, empty.Activities)
}
