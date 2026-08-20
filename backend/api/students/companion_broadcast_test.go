package students_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// accompaniedPlanBody is the departure plan the Stammdaten and personal-info
// forms submit on EVERY save: the complete allowed-modes map plus the coupled
// "mit wem" note. The forms resend it whether the user edited the plan or just
// a name, which is exactly the case the assertions below pin.
func accompaniedPlanBody(extra map[string]any) map[string]any {
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

// mondayLinkFingerprint is the fingerprint of the STORED list a companion
// submission replaces — every request that carries `companions` has to say what
// state it started from, or it gets no lost-update protection at all. All the
// links in this file are Monday links, so the stored state is fully described by
// the ids it holds (no ids = the empty list a child starts with).
func mondayLinkFingerprint(ids ...int64) string {
	links := make([]userModels.CompanionLink, 0, len(ids))
	for _, id := range ids {
		links = append(links, userModels.CompanionLink{CompanionStudentID: id, Weekdays: []string{"mon"}})
	}
	return userModels.CompanionLinksFingerprint(links)
}

func putStudent(t *testing.T, tc *testContext, studentID int64, body map[string]any) {
	t.Helper()
	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", studentID), body)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
}

// TestUpdateStudent_CompanionEventOnlyOnEffectiveChange pins that
// student_companions_changed announces a CHANGE, not the mere presence of plan
// fields in the request. Every open "läuft mit" editor in the school reacts to
// this event by marking itself stale and blocking its save, so a name-only save
// that resubmits the unchanged plan must stay silent — otherwise routine
// Stammdaten edits cost other people their unsaved work.
func TestUpdateStudent_CompanionEventOnlyOnEffectiveChange(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "CompanionEvent", "Subject", "CE1")
	companion := testpkg.CreateTestStudent(t, tc.db, "CompanionEvent", "Partner", "CE1")

	// Both children need an accompanied Monday before a link is legal.
	putStudent(t, tc, companion.ID, accompaniedPlanBody(nil))
	putStudent(t, tc, student.ID, accompaniedPlanBody(nil))

	t.Run("linking announces the change", func(t *testing.T) {
		tc.broadcaster.Reset()
		putStudent(t, tc, student.ID, accompaniedPlanBody(map[string]any{
			"companions": []map[string]any{
				{"companion_student_id": companion.ID, "weekdays": []string{"mon"}},
			},
			"companions_fingerprint": mondayLinkFingerprint(),
		}))
		assert.True(t, tc.broadcaster.HasEventType(realtime.EventStudentCompanionsChanged),
			"writing a new link must announce student_companions_changed")
	})

	t.Run("a name-only save resubmitting the same plan stays silent", func(t *testing.T) {
		tc.broadcaster.Reset()
		putStudent(t, tc, student.ID, accompaniedPlanBody(map[string]any{
			"first_name": "CompanionEventRenamed",
		}))
		assert.False(t, tc.broadcaster.HasEventType(realtime.EventStudentCompanionsChanged),
			"an unchanged plan and an untouched link list must not mark companion editors stale")
		assert.True(t, tc.broadcaster.HasEventType(realtime.EventStudentUpdated),
			"the ordinary student_updated invalidation still has to fire")
	})

	t.Run("resubmitting the identical link list stays silent", func(t *testing.T) {
		tc.broadcaster.Reset()
		putStudent(t, tc, student.ID, accompaniedPlanBody(map[string]any{
			"companions": []map[string]any{
				{"companion_student_id": companion.ID, "weekdays": []string{"mon"}},
			},
			"companions_fingerprint": mondayLinkFingerprint(companion.ID),
		}))
		assert.False(t, tc.broadcaster.HasEventType(realtime.EventStudentCompanionsChanged),
			"a replacement that writes the same links changed nothing to announce")
	})

	t.Run("clearing the links announces the change", func(t *testing.T) {
		tc.broadcaster.Reset()
		putStudent(t, tc, student.ID, accompaniedPlanBody(map[string]any{
			"companions":             []map[string]any{},
			"companions_fingerprint": mondayLinkFingerprint(companion.ID),
		}))
		assert.True(t, tc.broadcaster.HasEventType(realtime.EventStudentCompanionsChanged),
			"dropping a link must announce student_companions_changed")
	})
}

// TestDeleteStudent_AnnouncesCompanionChange pins the delete side of the same
// contract: ON DELETE CASCADE removes the child's edges from every SURVIVING
// child's list, so the other clients' companion cards and Kindersuche grouping
// are stale the moment the row is gone (#1694).
func TestDeleteStudent_AnnouncesCompanionChange(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "CompanionDelete", "Subject", "CD1")
	companion := testpkg.CreateTestStudent(t, tc.db, "CompanionDelete", "Partner", "CD1")

	putStudent(t, tc, companion.ID, accompaniedPlanBody(nil))
	putStudent(t, tc, student.ID, accompaniedPlanBody(map[string]any{
		"companions": []map[string]any{
			{"companion_student_id": companion.ID, "weekdays": []string{"mon"}},
		},
		"companions_fingerprint": mondayLinkFingerprint(),
	}))

	// The link is the only "mit wem" answer the partner would be left with, so
	// give them their own note back — otherwise the delete is refused for
	// stranding them, which is a different rule (and its own test).
	putStudent(t, tc, companion.ID, accompaniedPlanBody(nil))

	tc.broadcaster.Reset()
	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/%d", student.ID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	assert.True(t, tc.broadcaster.HasEventType(realtime.EventStudentCompanionsChanged),
		"deleting a linked child must announce student_companions_changed to the tenant")
}
