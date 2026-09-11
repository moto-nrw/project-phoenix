package parentmessaging

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// #2267 story 47: the co-guardian notice must carry NO reason and NO author.
// Those belong to the guardian who filed the request; a co-guardian who was
// not made a recipient never sees them. Stripping happens in one place so no
// call site can leak them by forgetting to clear a field.
func TestNeutralChildEventDropsReasonAndAuthor(t *testing.T) {
	t.Parallel()

	refID := int64(99)
	full := ChildEvent{
		EventType:      "request_status",
		ActorKind:      "staff",
		ActorAccountID: 4711,
		Body:           "Betreuungsstand geändert: Krankmeldung 01.09.2026",
		RequestType:    "absence",
		RequestStatus:  "erledigt",
		DecisionReason: "Attest liegt vor",
		RefTable:       "active.excused_absence_requests",
		RefID:          &refID,
		Payload:        map[string]any{"note": "privat"},
	}

	neutral := NeutralChildEvent(full)

	assert.Empty(t, neutral.DecisionReason, "the reason belongs to the submitting guardian")
	// ActorAccountID survives: sender_account_id is NOT NULL with an FK, and
	// the parents API projects sender_kind/sender_name ("System"), never the
	// account, so keeping it names nobody to the reader.
	assert.Equal(t, full.ActorAccountID, neutral.ActorAccountID)
	assert.Empty(t, neutral.RequestStatus, "a co-guardian filed no request to have a status")
	assert.Empty(t, neutral.RefTable)
	assert.Nil(t, neutral.RefID, "no deep link into a request this guardian may not read")
	assert.Nil(t, neutral.Payload)

	// What survives is what makes the line useful at all.
	assert.Equal(t, full.EventType, neutral.EventType)
	assert.Equal(t, full.ActorKind, neutral.ActorKind)
	assert.Equal(t, full.Body, neutral.Body)
	assert.Equal(t, full.RequestType, neutral.RequestType)
}
