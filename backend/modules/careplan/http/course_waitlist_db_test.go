package careplanhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type courseWaitlistWire struct {
	ActivityGroupID  string `json:"activity_group_id"`
	Name             string `json:"name"`
	ParticipantLimit *int   `json:"participant_limit"`
	Participants     int    `json:"participants"`
	FreeSlots        *int   `json:"free_slots"`
	SlotFreeForNext  bool   `json:"slot_free_for_next"`
	Entries          []struct {
		RequestID   string `json:"request_id"`
		StudentID   string `json:"student_id"`
		StudentName string `json:"student_name"`
		Rank        int    `json:"rank"`
	} `json:"entries"`
}

// courseWaitlistRequest is one family asking for the AG: its approved
// enrollment child, a course offering of the child's phase fed by the AG,
// and the open request adding that course to today's booking.
type courseWaitlistRequest struct {
	studentID int64
	courseID  int64
	request   *careplan.OfferingChangeRequest
}

func createCourseWaitlistRequest(t *testing.T, tc *testContext, groupID int64, firstName, lastName string) courseWaitlistRequest {
	t.Helper()
	student := testpkg.CreateTestStudent(t, tc.db, firstName, lastName, "3a")
	_, guardian := testpkg.CreateTestTeacherWithAccount(t, tc.db, firstName, "Elternteil")
	fixture := setupCorrectionFixture(t, tc, student.ID, testpkg.Tenant(t), lastName)
	course := testpkg.CreateTestCareOffering(t, tc.db, fixture.phase.ID, "Fussball-Kurs")
	_, err := tc.db.NewRaw(`UPDATE enrollment.care_offerings SET activity_group_id = ? WHERE id = ? AND tenant_id = ?`,
		groupID, course.ID, testpkg.Tenant(t)).Exec(t.Context())
	require.NoError(t, err)
	payload, err := json.Marshal(map[string]any{"offerings": []any{
		map[string]any{"offering_id": fixture.ganztag.ID},
		map[string]any{"offering_id": fixture.mittag.ID},
		map[string]any{"offering_id": course.ID},
	}})
	require.NoError(t, err)
	row := &careplan.OfferingChangeRequest{}
	err = tc.db.NewRaw(`INSERT INTO enrollment.offering_change_requests
		(tenant_id, student_id, request_child_id, submitted_by, payload, effective_from, status)
		VALUES (?, ?, ?, ?, ?::jsonb, ?, ?) RETURNING id, created_at, updated_at`,
		testpkg.Tenant(t), student.ID, fixture.child.ID, guardian.ID, string(payload),
		string(studentsTestToday.AddDays(30)), careplan.OfferingChangePending).Scan(t.Context(), row)
	require.NoError(t, err)
	return courseWaitlistRequest{studentID: student.ID, courseID: course.ID, request: row}
}

func fetchCourseWaitlists(t *testing.T, tc *testContext, claims sessiontoken.AppClaims, query string) []courseWaitlistWire {
	t.Helper()
	rr := authExec(t, tc, testutil.NewRequest(http.MethodGet, "/offering-change-requests/course-waitlists"+query, nil),
		claims, []string{"users:update"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var envelope struct {
		Data []courseWaitlistWire `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
	return envelope.Data
}

func waitlistFor(waitlists []courseWaitlistWire, groupID int64) *courseWaitlistWire {
	for i := range waitlists {
		if waitlists[i].ActivityGroupID == strconv.FormatInt(groupID, 10) {
			return &waitlists[i]
		}
	}
	return nil
}

// TestCourseWaitlistsRankFreeSlotAndTenant drives the team's waiting list
// (#4767) through the production route: the oldest request is first, a free
// place names that child, moving up is the ordinary approval of the first
// request, and another school's queue never shows.
func TestCourseWaitlistsRankFreeSlotAndTenant(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	_, reviewer := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Warte", "Liste")
	claims := testutil.AdminTestClaims(int(reviewer.ID))
	group := testpkg.CreateTestActivityGroupWithLimit(t, tc.db, "Fußball-AG", 1)

	first := createCourseWaitlistRequest(t, tc, group.ID, "Mia", "Erstanfrage")
	second := createCourseWaitlistRequest(t, tc, group.ID, "Noah", "Zweitanfrage")

	var foreignGroupID int64
	t.Run("foreign school", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreign := setupStudentsRoute(t, fixedCalendarClock)
		foreignGroup := testpkg.CreateTestActivityGroupWithLimit(t, foreign.db, "Fremde AG", 1)
		createCourseWaitlistRequest(t, foreign, foreignGroup.ID, "Lea", "Fremdschule")
		foreignGroupID = foreignGroup.ID
	})

	t.Run("ranks by submission and names the next child", func(t *testing.T) {
		waitlists := fetchCourseWaitlists(t, tc, claims, "")
		waitlist := waitlistFor(waitlists, group.ID)
		require.NotNil(t, waitlist, "the AG with open requests is listed")
		assert.Equal(t, "Fußball-AG", waitlist.Name)
		require.NotNil(t, waitlist.ParticipantLimit)
		assert.Equal(t, 1, *waitlist.ParticipantLimit)
		assert.Equal(t, 0, waitlist.Participants)
		require.NotNil(t, waitlist.FreeSlots)
		assert.Equal(t, 1, *waitlist.FreeSlots)
		assert.True(t, waitlist.SlotFreeForNext, "a free place and a waiting child is the team's cue")
		require.Len(t, waitlist.Entries, 2)
		assert.Equal(t, strconv.FormatInt(first.request.ID, 10), waitlist.Entries[0].RequestID)
		assert.Equal(t, 1, waitlist.Entries[0].Rank)
		assert.Equal(t, "Mia Erstanfrage", waitlist.Entries[0].StudentName)
		assert.Equal(t, strconv.FormatInt(second.request.ID, 10), waitlist.Entries[1].RequestID)
		assert.Equal(t, 2, waitlist.Entries[1].Rank)
	})

	t.Run("another school's queue stays invisible", func(t *testing.T) {
		require.Positive(t, foreignGroupID)
		assert.Nil(t, waitlistFor(fetchCourseWaitlists(t, tc, claims, ""), foreignGroupID))
		assert.Empty(t, fetchCourseWaitlists(t, tc, claims, fmt.Sprintf("?activity_group_id=%d", foreignGroupID)),
			"a foreign AG id reads as no AG at all")
	})

	t.Run("the second child cannot overtake the first", func(t *testing.T) {
		rr := decideCourseRequest(t, tc, claims, second.request.ID)
		require.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
	})

	t.Run("approving the first moves the second up", func(t *testing.T) {
		rr := decideCourseRequest(t, tc, claims, first.request.ID)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

		waitlists := fetchCourseWaitlists(t, tc, claims, fmt.Sprintf("?activity_group_id=%d", group.ID))
		require.Len(t, waitlists, 1)
		waitlist := waitlists[0]
		require.Len(t, waitlist.Entries, 1)
		assert.Equal(t, strconv.FormatInt(second.request.ID, 10), waitlist.Entries[0].RequestID)
		assert.Equal(t, 1, waitlist.Entries[0].Rank, "the next child is first now")
		assert.Equal(t, 1, waitlist.Participants, "the approved child holds the only place")
		require.NotNil(t, waitlist.FreeSlots)
		assert.Equal(t, 0, *waitlist.FreeSlots)
		assert.False(t, waitlist.SlotFreeForNext, "the AG is full again")
	})
}

func TestCourseWaitlistsRespectReviewerGroupScope(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Warteliste", "Prüfung")
	visibleGroup := testpkg.CreateTestEducationGroup(t, tc.db, "Sichtbare Warteliste")
	hiddenGroup := testpkg.CreateTestEducationGroup(t, tc.db, "Verborgene Warteliste")
	testpkg.CreateTestGroupTeacher(t, tc.db, visibleGroup.ID, teacher.ID)
	courseGroup := testpkg.CreateTestActivityGroupWithLimit(t, tc.db, "Theater-AG", 2)
	hiddenCourseGroup := testpkg.CreateTestActivityGroupWithLimit(t, tc.db, "Verborgene Theater-AG", 2)

	hidden := createCourseWaitlistRequest(t, tc, hiddenCourseGroup.ID, "Verborgen", "Kind")
	visible := createCourseWaitlistRequest(t, tc, courseGroup.ID, "Sichtbar", "Kind")
	testpkg.AssignStudentToGroup(t, tc.db, hidden.studentID, hiddenGroup.ID)
	testpkg.AssignStudentToGroup(t, tc.db, visible.studentID, visibleGroup.ID)
	require.NoError(t, tc.settings().SetValue(
		testpkg.Ctx(t), settings.KeyParentRequestReviewScope, settings.ParentRequestReviewScopeGroupLeaders, nil, nil,
	))

	waitlist := waitlistFor(fetchCourseWaitlists(t, tc, testutil.TeacherTestClaims(int(account.ID)), ""), courseGroup.ID)
	require.NotNil(t, waitlist)
	require.Len(t, waitlist.Entries, 1)
	assert.Equal(t, strconv.FormatInt(visible.request.ID, 10), waitlist.Entries[0].RequestID)
	assert.Equal(t, "Sichtbar Kind", waitlist.Entries[0].StudentName)
	assert.Equal(t, 1, waitlist.Entries[0].Rank, "out-of-scope requests must not affect the visible queue position")
	assert.Empty(t, fetchCourseWaitlists(t, tc, testutil.TeacherTestClaims(int(account.ID)), fmt.Sprintf("?activity_group_id=%d", hiddenCourseGroup.ID)),
		"an explicit AG filter must not expose an AG without a visible request")
}

func TestCourseWaitlistsReadsOneAGWithoutQueue(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	_, reviewer := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Leere", "Liste")
	group := testpkg.CreateTestActivityGroupWithLimit(t, tc.db, "Theater-AG", 8)

	waitlists := fetchCourseWaitlists(t, tc, testutil.AdminTestClaims(int(reviewer.ID)), fmt.Sprintf("?activity_group_id=%d", group.ID))

	require.Len(t, waitlists, 1)
	assert.Equal(t, "Theater-AG", waitlists[0].Name)
	assert.Empty(t, waitlists[0].Entries)
	require.NotNil(t, waitlists[0].FreeSlots)
	assert.Equal(t, 8, *waitlists[0].FreeSlots)
	assert.False(t, waitlists[0].SlotFreeForNext, "nobody waits, so there is nothing to confirm")
}

func TestCourseWaitlistsRejectMalformedPendingRequest(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	_, reviewer := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Ungültige", "Anfrage")
	group := testpkg.CreateTestActivityGroupWithLimit(t, tc.db, "Theater-AG", 8)
	request := createCourseWaitlistRequest(t, tc, group.ID, "Mia", "Ungültig")
	_, err := tc.db.NewRaw(`UPDATE enrollment.offering_change_requests
		SET payload = ?::jsonb
		WHERE id = ? AND tenant_id = ?`, `{"not_offerings":[]}`, request.request.ID, testpkg.Tenant(t)).Exec(t.Context())
	require.NoError(t, err)

	rr := authExec(t, tc, testutil.NewRequest(http.MethodGet, "/offering-change-requests/course-waitlists", nil),
		testutil.AdminTestClaims(int(reviewer.ID)), []string{"users:update"})

	assert.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
}

func TestCourseWaitlistsReadOfferingCapacityForEmptyExplicitAG(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	_, reviewer := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Leere", "Angebotsgrenze")
	claims := testutil.AdminTestClaims(int(reviewer.ID))
	group := testpkg.CreateTestActivityGroup(t, tc.db, "Offene AG")
	_, err := tc.db.NewRaw(`UPDATE activities.groups SET max_participants = NULL WHERE id = ? AND tenant_id = ?`,
		group.ID, testpkg.Tenant(t)).Exec(t.Context())
	require.NoError(t, err)
	request := createCourseWaitlistRequest(t, tc, group.ID, "Mia", "Erledigt")
	_, err = tc.db.NewRaw(`UPDATE enrollment.care_offerings SET capacity = 3 WHERE id = ? AND tenant_id = ?`,
		request.courseID, testpkg.Tenant(t)).Exec(t.Context())
	require.NoError(t, err)
	_, err = tc.db.NewRaw(`UPDATE enrollment.offering_change_requests SET status = ? WHERE id = ? AND tenant_id = ?`,
		careplan.OfferingChangeRejected, request.request.ID, testpkg.Tenant(t)).Exec(t.Context())
	require.NoError(t, err)

	waitlist := waitlistFor(fetchCourseWaitlists(t, tc, claims, fmt.Sprintf("?activity_group_id=%d", group.ID)), group.ID)
	require.NotNil(t, waitlist)
	require.NotNil(t, waitlist.ParticipantLimit)
	assert.Equal(t, 3, *waitlist.ParticipantLimit)
	require.NotNil(t, waitlist.FreeSlots)
	assert.Equal(t, 3, *waitlist.FreeSlots)
	assert.Empty(t, waitlist.Entries)
}

func TestCourseWaitlistsExcludeUnlimitedGroupsFromQueue(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	_, reviewer := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Ohne", "Grenze")
	claims := testutil.AdminTestClaims(int(reviewer.ID))
	group := testpkg.CreateTestActivityGroup(t, tc.db, "Offene AG")
	_, err := tc.db.NewRaw(`UPDATE activities.groups SET max_participants = NULL WHERE id = ? AND tenant_id = ?`,
		group.ID, testpkg.Tenant(t)).Exec(t.Context())
	require.NoError(t, err)
	createCourseWaitlistRequest(t, tc, group.ID, "Mia", "Offen")

	assert.Nil(t, waitlistFor(fetchCourseWaitlists(t, tc, claims, ""), group.ID),
		"an unlimited AG does not have a capacity waitlist")

	waitlists := fetchCourseWaitlists(t, tc, claims, fmt.Sprintf("?activity_group_id=%d", group.ID))
	require.Len(t, waitlists, 1)
	assert.Empty(t, waitlists[0].Entries, "the explicit AG view keeps its data but no waitlist entries")
}

func TestCourseWaitlistsIncludeUnlimitedGroupsWithLimitedOfferings(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	_, reviewer := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Angebot", "Grenze")
	claims := testutil.AdminTestClaims(int(reviewer.ID))
	group := testpkg.CreateTestActivityGroup(t, tc.db, "Offene AG")
	_, err := tc.db.NewRaw(`UPDATE activities.groups SET max_participants = NULL WHERE id = ? AND tenant_id = ?`,
		group.ID, testpkg.Tenant(t)).Exec(t.Context())
	require.NoError(t, err)
	waiting := createCourseWaitlistRequest(t, tc, group.ID, "Mia", "Wartet")
	_, err = tc.db.NewRaw(`UPDATE enrollment.care_offerings SET capacity = 0 WHERE id = ? AND tenant_id = ?`,
		waiting.courseID, testpkg.Tenant(t)).Exec(t.Context())
	require.NoError(t, err)

	for _, query := range []string{"", fmt.Sprintf("?activity_group_id=%d", group.ID)} {
		waitlist := waitlistFor(fetchCourseWaitlists(t, tc, claims, query), group.ID)
		require.NotNil(t, waitlist, "the offering limit creates a waitlist even without an AG limit")
		require.NotNil(t, waitlist.ParticipantLimit)
		assert.Equal(t, 0, *waitlist.ParticipantLimit)
		require.NotNil(t, waitlist.FreeSlots)
		assert.Equal(t, 0, *waitlist.FreeSlots)
		assert.Len(t, waitlist.Entries, 1)
		assert.False(t, waitlist.SlotFreeForNext)
	}
}

func TestCourseWaitlistsUseNextRequestEffectiveDateForCapacity(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	_, reviewer := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Später", "Frei")
	claims := testutil.AdminTestClaims(int(reviewer.ID))
	group := testpkg.CreateTestActivityGroupWithLimit(t, tc.db, "Fußball-AG", 1)
	createCourseWaitlistRequest(t, tc, group.ID, "Mia", "Zukunft")

	occupied := testpkg.CreateTestStudent(t, tc.db, "Belegt", "BisDahin", "3a")
	_, err := tc.db.NewRaw(`INSERT INTO activities.student_enrollments
		(tenant_id, student_id, activity_group_id, valid_from, valid_until)
		VALUES (?, ?, ?, ?, ?)`,
		testpkg.Tenant(t), occupied.ID, group.ID, studentsTestToday, studentsTestToday.AddDays(30)).Exec(t.Context())
	require.NoError(t, err)

	waitlist := waitlistFor(fetchCourseWaitlists(t, tc, claims, fmt.Sprintf("?activity_group_id=%d", group.ID)), group.ID)
	require.NotNil(t, waitlist)
	assert.Equal(t, 0, waitlist.Participants, "the existing enrollment ends before the request becomes effective")
	require.NotNil(t, waitlist.FreeSlots)
	assert.Equal(t, 1, *waitlist.FreeSlots)
	assert.True(t, waitlist.SlotFreeForNext)
}

func TestCourseWaitlistsExcludeNextChildFromGroupOccupancy(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	_, reviewer := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Schon", "Dabei")
	claims := testutil.AdminTestClaims(int(reviewer.ID))
	group := testpkg.CreateTestActivityGroupWithLimit(t, tc.db, "Fußball-AG", 1)
	waiting := createCourseWaitlistRequest(t, tc, group.ID, "Mia", "Wartet")
	_, err := tc.db.NewRaw(`INSERT INTO activities.student_enrollments
		(tenant_id, student_id, activity_group_id, valid_from)
		VALUES (?, ?, ?, ?)`,
		testpkg.Tenant(t), waiting.studentID, group.ID, studentsTestToday).Exec(t.Context())
	require.NoError(t, err)

	waitlist := waitlistFor(fetchCourseWaitlists(t, tc, claims, fmt.Sprintf("?activity_group_id=%d", group.ID)), group.ID)
	require.NotNil(t, waitlist)
	assert.Equal(t, 0, waitlist.Participants, "the next child already holds this AG through another offering")
	require.NotNil(t, waitlist.FreeSlots)
	assert.Equal(t, 1, *waitlist.FreeSlots)
	assert.True(t, waitlist.SlotFreeForNext)
	assert.Equal(t, http.StatusOK, decideCourseRequest(t, tc, claims, waiting.request.ID).Code)
}

func TestCourseWaitlistsUseCourseOfferingCapacity(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	_, reviewer := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Angebot", "Voll")
	claims := testutil.AdminTestClaims(int(reviewer.ID))
	group := testpkg.CreateTestActivityGroupWithLimit(t, tc.db, "Fußball-AG", 2)
	waiting := createCourseWaitlistRequest(t, tc, group.ID, "Mia", "Wartet")
	_, err := tc.db.NewRaw(`UPDATE enrollment.care_offerings SET capacity = 0 WHERE id = ? AND tenant_id = ?`,
		waiting.courseID, testpkg.Tenant(t)).Exec(t.Context())
	require.NoError(t, err)

	waitlist := waitlistFor(fetchCourseWaitlists(t, tc, claims, fmt.Sprintf("?activity_group_id=%d", group.ID)), group.ID)
	require.NotNil(t, waitlist)
	require.NotNil(t, waitlist.ParticipantLimit)
	assert.Equal(t, 0, *waitlist.ParticipantLimit)
	require.NotNil(t, waitlist.FreeSlots)
	assert.Equal(t, 0, *waitlist.FreeSlots)
	assert.False(t, waitlist.SlotFreeForNext, "a full course offering cannot be freed")

	rr := decideCourseRequest(t, tc, claims, waiting.request.ID)
	assert.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
}

func TestCourseWaitlistsRejectsInvalidFilter(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	_, reviewer := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Falscher", "Filter")

	rr := authExec(t, tc, testutil.NewRequest(http.MethodGet, "/offering-change-requests/course-waitlists?activity_group_id=abc", nil),
		testutil.AdminTestClaims(int(reviewer.ID)), []string{"users:update"})

	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
}

func decideCourseRequest(t *testing.T, tc *testContext, claims sessiontoken.AppClaims, requestID int64) *httptest.ResponseRecorder {
	t.Helper()
	body := strings.NewReader(fmt.Sprintf(`{"approve":true,"effective_from":"%s","reason":"Platz frei"}`, studentsTestToday.AddDays(30)))
	return authExec(t, tc, testutil.NewRequest(http.MethodPost, fmt.Sprintf("/offering-change-requests/%d/decide", requestID), body),
		claims, []string{"users:update"})
}

// TestCourseWaitlistsQueryBudget guards GET /offering-change-requests/
// course-waitlists against one read per request or per AG (#4767): the
// statement count stays the same from one waiting child to four children
// over two AGs.
func TestCourseWaitlistsQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc := setupStudentsRoute(t, fixedCalendarClock)
	_, reviewer := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Budget", "Warteliste")
	claims := testutil.AdminTestClaims(int(reviewer.ID))
	football := testpkg.CreateTestActivityGroupWithLimit(t, tc.db, "Fußball-AG", 1)
	theater := testpkg.CreateTestActivityGroupWithLimit(t, tc.db, "Theater-AG", 1)

	counter := testpkg.CaptureQueries(t, tc.db)
	run := func() []string {
		counter.Reset()
		fetchCourseWaitlists(t, tc, claims, "")
		return counter.Queries()
	}
	setCourseCapacity := func(request courseWaitlistRequest) {
		t.Helper()
		_, err := tc.db.NewRaw(`UPDATE enrollment.care_offerings SET capacity = 8 WHERE id = ? AND tenant_id = ?`,
			request.courseID, testpkg.Tenant(t)).Exec(t.Context())
		require.NoError(t, err)
	}

	setCourseCapacity(createCourseWaitlistRequest(t, tc, football.ID, "Kind", "Eins"))
	small := run()

	setCourseCapacity(createCourseWaitlistRequest(t, tc, football.ID, "Kind", "Zwei"))
	setCourseCapacity(createCourseWaitlistRequest(t, tc, theater.ID, "Kind", "Drei"))
	setCourseCapacity(createCourseWaitlistRequest(t, tc, theater.ID, "Kind", "Vier"))
	large := run()

	t.Logf("1 request → %d statements, 4 requests over 2 AGs → %d statements", len(small), len(large))
	assert.Len(t, large, len(small), "the statement count must not grow with the queue")
	testpkg.AssertQueryBudget(t, "api.students.course_waitlists.list", large)
}
