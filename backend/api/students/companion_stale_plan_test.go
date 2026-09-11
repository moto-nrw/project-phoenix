package students_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	studentsAPI "github.com/moto-nrw/project-phoenix/api/students"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// putStudentExpect performs the student PUT and returns the recorder, for the
// cases that assert on a refusal rather than on a success.
func putStudentExpect(t *testing.T, tc *testContext, studentID int64, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", studentID), body)
	return authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
}

// companionLinkDays reads the child's stored "läuft mit" links back through the
// API, per partner and weekday, so the assertions below judge what the school
// actually sees.
func companionLinkDays(t *testing.T, tc *testContext, studentID int64) map[int64][]string {
	t.Helper()
	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/%d/companions", studentID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var body struct {
		Data []struct {
			CompanionStudentID int64    `json:"companion_student_id"`
			Weekdays           []string `json:"weekdays"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	links := make(map[int64][]string, len(body.Data))
	for _, entry := range body.Data {
		links[entry.CompanionStudentID] = entry.Weekdays
	}
	return links
}

// TestUpdateStudent_PlanOnlyTrimNeedsBaseline pins the protection for the write
// that deletes links WITHOUT naming any: a departure plan alone trims every link
// on a weekday it no longer allows.
//
// The Stammdaten forms resubmit the whole plan on every save, so a form opened
// before someone else widened the plan and linked a child on the new day carries
// a plan that would delete that fresh edge — and because no list travels with it,
// the fingerprint that guards a submitted list has nothing to compare. The
// stranding check is no substitute: it only refuses removals that leave the far
// child with no "mit wem" answer at all, so a companion with a note (as here)
// loses the link silently.
func TestUpdateStudent_PlanOnlyTrimNeedsBaseline(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	student := testpkg.CreateTestStudent(t, tc.db, "StaleTrim", "Subject", "ST1")
	companion := testpkg.CreateTestStudent(t, tc.db, "StaleTrim", "Partner", "ST1")

	// Both children accompany on Monday AND Tuesday, and both keep their own
	// note — so dropping a link stands or falls on the baseline rule alone.
	widePlan := func(extra map[string]any) map[string]any {
		body := map[string]any{
			"allowed_departure_modes":  map[string]any{"mon": []string{"accompanied"}, "tue": []string{"accompanied"}},
			"departure_companion_note": "Nachbarskind",
			"departure_days":           map[string]any{"mon": "accompanied", "tue": "accompanied"},
		}
		for k, v := range extra {
			body[k] = v
		}
		return body
	}
	putStudent(t, tc, companion.ID, widePlan(nil))
	putStudent(t, tc, student.ID, widePlan(map[string]any{
		"companions": []map[string]any{
			{"companion_student_id": companion.ID, "weekdays": []string{"mon", "tue"}},
		},
		"companions_fingerprint": "",
	}))

	current := fmt.Sprintf("%d:mon,tue", companion.ID)
	// The narrowing an out-of-date form would submit: Tuesday gone from the plan,
	// no list, so the Tuesday edge is on the block.
	narrowed := func(extra map[string]any) map[string]any {
		body := map[string]any{
			"allowed_departure_modes":  map[string]any{"mon": []string{"accompanied"}},
			"departure_companion_note": "Nachbarskind",
			"departure_days":           map[string]any{"mon": "accompanied"},
		}
		for k, v := range extra {
			body[k] = v
		}
		return body
	}

	t.Run("without a baseline the removal is refused", func(t *testing.T) {
		rr := putStudentExpect(t, tc, student.ID, narrowed(nil))
		require.Equal(t, http.StatusConflict, rr.Code, "Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), studentsAPI.CodeCompanionsChanged,
			"the client has to be told to reload, not to confirm anything")
		assert.Equal(t, map[int64][]string{companion.ID: {"mon", "tue"}}, companionLinkDays(t, tc, student.ID),
			"a refused update must leave the links untouched")
	})

	t.Run("a stale baseline is refused too", func(t *testing.T) {
		stale := fmt.Sprintf("%d:mon", companion.ID)
		rr := putStudentExpect(t, tc, student.ID, narrowed(map[string]any{
			"companions_fingerprint": stale,
		}))
		require.Equal(t, http.StatusConflict, rr.Code, "Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), studentsAPI.CodeCompanionsChanged)
	})

	t.Run("the current baseline lets the intended narrowing through", func(t *testing.T) {
		putStudent(t, tc, student.ID, narrowed(map[string]any{
			"companions_fingerprint": current,
		}))
		// Monday survives, Tuesday is gone: the trim is not blocked, only proven.
		assert.Equal(t, map[int64][]string{companion.ID: {"mon"}}, companionLinkDays(t, tc, student.ID))
	})

	t.Run("a plan that removes nothing needs no baseline", func(t *testing.T) {
		// The overwhelmingly common save: the same plan, resubmitted with a name
		// edit. It trims nothing, so it must not be held to a claim the forms
		// cannot always make (a failed companion load has no fingerprint).
		putStudent(t, tc, student.ID, narrowed(map[string]any{
			"first_name": "StaleTrimRenamed",
		}))
		assert.Equal(t, map[int64][]string{companion.ID: {"mon"}}, companionLinkDays(t, tc, student.ID))
	})
}

// TestUpdateStudent_ReportsCompanionVerdict pins the companions_changed field on
// the PUT response.
//
// The client cannot derive it from its own payload — the forms resubmit the whole
// departure plan on every save — and reacting to a save as if it had changed the
// links costs an open Laufgemeinschaft editor its draft. So the write reports
// what it actually did, on exactly the condition that drives the
// student_companions_changed broadcast.
func TestUpdateStudent_ReportsCompanionVerdict(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	student := testpkg.CreateTestStudent(t, tc.db, "CompanionVerdict", "Subject", "CV1")
	companion := testpkg.CreateTestStudent(t, tc.db, "CompanionVerdict", "Partner", "CV1")

	putStudent(t, tc, companion.ID, accompaniedPlanBody(nil))
	putStudent(t, tc, student.ID, accompaniedPlanBody(nil))

	verdict := func(t *testing.T, body map[string]any) *bool {
		t.Helper()
		rr := putStudentExpect(t, tc, student.ID, body)
		require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
		var parsed struct {
			Data struct {
				CompanionsChanged *bool `json:"companions_changed"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &parsed))
		require.NotNil(t, parsed.Data.CompanionsChanged,
			"every update answers the question — an absent key means 'no verdict', not 'unchanged'")
		return parsed.Data.CompanionsChanged
	}

	t.Run("a new link reports a change", func(t *testing.T) {
		assert.True(t, *verdict(t, accompaniedPlanBody(map[string]any{
			"companions": []map[string]any{
				{"companion_student_id": companion.ID, "weekdays": []string{"mon"}},
			},
			"companions_fingerprint": mondayLinkFingerprint(),
		})))
	})

	t.Run("a name-only save on the same plan reports none", func(t *testing.T) {
		assert.False(t, *verdict(t, accompaniedPlanBody(map[string]any{
			"first_name": "CompanionVerdictRenamed",
		})))
	})
}

// TestUpdateStudent_CompanionsWithoutAccompaniedDayRefused pins the refusal for
// the one companion payload the write cannot honour: a list submitted with a
// departure plan that allows "Anderes Kind" on no weekday at all.
//
// Such a request contradicts itself, and the write resolves the contradiction in
// favour of the plan — it drops every link. Answering 200 would tell the client
// its list was stored when nothing of it was even validated, so a stale form (or
// a direct API client) would keep showing a Laufgemeinschaft the school no longer
// has. The empty list is the opposite case: it says what the plan says, and is
// exactly what our own form submits when the last accompanied day is taken away.
func TestUpdateStudent_CompanionsWithoutAccompaniedDayRefused(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	student := testpkg.CreateTestStudent(t, tc.db, "NoAccompaniedDay", "Subject", "NA1")
	companion := testpkg.CreateTestStudent(t, tc.db, "NoAccompaniedDay", "Partner", "NA1")

	putStudent(t, tc, companion.ID, accompaniedPlanBody(nil))
	putStudent(t, tc, student.ID, accompaniedPlanBody(map[string]any{
		"companions": []map[string]any{
			{"companion_student_id": companion.ID, "weekdays": []string{"mon"}},
		},
		"companions_fingerprint": mondayLinkFingerprint(),
	}))

	// The plan every child starts from: home alone on Monday, no accompanied day
	// anywhere — so nothing in it can carry a link.
	alonePlan := func(extra map[string]any) map[string]any {
		body := map[string]any{
			"allowed_departure_modes": map[string]any{"mon": []string{"alone"}},
			"departure_days":          map[string]any{"mon": "alone"},
		}
		for k, v := range extra {
			body[k] = v
		}
		return body
	}

	t.Run("a non-empty list is refused", func(t *testing.T) {
		rr := putStudentExpect(t, tc, student.ID, alonePlan(map[string]any{
			"companions": []map[string]any{
				{"companion_student_id": companion.ID, "weekdays": []string{"mon"}},
			},
			"companions_fingerprint": mondayLinkFingerprint(companion.ID),
		}))
		require.Equal(t, http.StatusBadRequest, rr.Code, "Body: %s", rr.Body.String())
		assert.Equal(t, map[int64][]string{companion.ID: {"mon"}}, companionLinkDays(t, tc, student.ID),
			"a refused update must leave the links untouched")
	})

	t.Run("the empty list clears the Laufgemeinschaft", func(t *testing.T) {
		putStudent(t, tc, student.ID, alonePlan(map[string]any{
			"companions":             []map[string]any{},
			"companions_fingerprint": mondayLinkFingerprint(companion.ID),
		}))
		assert.Empty(t, companionLinkDays(t, tc, student.ID))
	})
}
