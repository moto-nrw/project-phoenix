// Which pill becomes a parent push, and what it says (#1671). Pure: the
// delivery around these two functions is the notification service's business.
package parentmessaging

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsStaffDecisionPill(t *testing.T) {
	t.Parallel()

	decision := ChildEvent{
		EventType:     EventRequestStatus,
		ActorKind:     ActorStaff,
		RequestStatus: RequestStatusDone,
		RequestType:   "care_schedule",
	}

	t.Run("a staff decision qualifies", func(t *testing.T) {
		assert.True(t, isStaffDecisionPill(decision))

		rejected := decision
		rejected.RequestStatus = RequestStatusRejected
		assert.True(t, isStaffDecisionPill(rejected), "a rejection is news too")
	})

	t.Run("a parent's own actions do not push back at them", func(t *testing.T) {
		withdrawn := decision
		withdrawn.ActorKind = ActorGuardian
		withdrawn.RequestStatus = RequestStatusWithdrawn
		assert.False(t, isStaffDecisionPill(withdrawn))

		submitted := decision
		submitted.EventType = EventRequestCreated
		submitted.ActorKind = ActorGuardian
		assert.False(t, isStaffDecisionPill(submitted))
	})

	t.Run("an unresolved request is not a decision", func(t *testing.T) {
		open := decision
		open.RequestStatus = RequestStatusOpen
		assert.False(t, isStaffDecisionPill(open))
	})

	t.Run("a self-service mirror is not a decision", func(t *testing.T) {
		// Sick notes and one-day pickup changes are mirrored into the thread as
		// their own event types (see ChildEvent.EventType). They are the parent's
		// own action, so they must not push back at them.
		sickNote := decision
		sickNote.EventType = "sick_note"
		assert.False(t, isStaffDecisionPill(sickNote))
	})
}

func TestRequestDecisionCopy(t *testing.T) {
	t.Parallel()

	t.Run("names the subject area, never the child", func(t *testing.T) {
		cases := map[string]string{
			"care_schedule":   "Betreuungszeiten",
			"pickup_change":   "Abholzeit",
			"master_data":     "Stammdaten",
			"excused_absence": "Abmeldung",
			"sick_absence":    "Krankmeldung",
		}
		for requestType, expected := range cases {
			title, body := requestDecisionCopy("de", requestType, RequestStatusDone)
			assert.Equal(t, "Anfrage genehmigt", title)
			assert.Contains(t, body, expected)
		}
	})

	t.Run("an unknown request type still reads as a sentence", func(t *testing.T) {
		title, body := requestDecisionCopy("de", "something_new", RequestStatusDone)
		assert.Equal(t, "Anfrage genehmigt", title)
		assert.Equal(t, "Ihre Anfrage wurde genehmigt.", body,
			"a future request type must degrade to generic copy, not to an empty push")
	})

	t.Run("a rejection says so", func(t *testing.T) {
		title, body := requestDecisionCopy("de", "care_schedule", RequestStatusRejected)
		assert.Equal(t, "Anfrage abgelehnt", title)
		assert.Contains(t, body, "abgelehnt")
	})

	t.Run("uses the guardian locale", func(t *testing.T) {
		title, body := requestDecisionCopy("en", "care_schedule", RequestStatusDone)
		assert.Equal(t, "Request approved", title)
		assert.Equal(t, "Your care schedule request was approved.", body)
	})
}
