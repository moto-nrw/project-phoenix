// Which pill becomes a parent push, and what it says (#1671). Pure: the
// delivery around these two functions is the notification service's business.
package parentmessaging

import (
	"testing"

	eventContract "github.com/moto-nrw/project-phoenix/modules/communication/eventcontract"

	"github.com/stretchr/testify/assert"
)

func TestIsStaffDecisionPill(t *testing.T) {
	t.Parallel()

	decision := eventContract.ChildEvent{
		EventType:     eventContract.EventRequestStatus,
		ActorKind:     eventContract.ActorStaff,
		RequestStatus: eventContract.RequestStatusDone,
		RequestType:   "care_schedule",
	}

	t.Run("a staff decision qualifies", func(t *testing.T) {
		assert.True(t, isStaffDecisionPill(decision))

		rejected := decision
		rejected.RequestStatus = eventContract.RequestStatusRejected
		assert.True(t, isStaffDecisionPill(rejected), "a rejection is news too")
	})

	t.Run("a parent's own actions do not push back at them", func(t *testing.T) {
		withdrawn := decision
		withdrawn.ActorKind = eventContract.ActorGuardian
		withdrawn.RequestStatus = eventContract.RequestStatusWithdrawn
		assert.False(t, isStaffDecisionPill(withdrawn))

		submitted := decision
		submitted.EventType = eventContract.EventRequestCreated
		submitted.ActorKind = eventContract.ActorGuardian
		assert.False(t, isStaffDecisionPill(submitted))
	})

	t.Run("an unresolved request is not a decision", func(t *testing.T) {
		open := decision
		open.RequestStatus = eventContract.RequestStatusOpen
		assert.False(t, isStaffDecisionPill(open))
	})

	t.Run("a self-service mirror is not a decision", func(t *testing.T) {
		// Sick notes and one-day pickup changes are mirrored into the thread as
		// their own event types (see eventContract.ChildEvent.EventType). They are the parent's
		// own action, so they must not push back at them.
		sickNote := decision
		sickNote.EventType = "sick_note"
		assert.False(t, isStaffDecisionPill(sickNote))
	})
}
