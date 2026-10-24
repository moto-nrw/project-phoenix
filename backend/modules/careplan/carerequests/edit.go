package carerequests

import (
	"context"
	"encoding/json"
	"time"

	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// EditInput cannot change the kind of the locked request. Pickup edits check
// the cutoff for both the stored day and the proposed day.
type EditInput struct {
	RequestID         int64
	StudentID         int64
	GuardianAccountID int64
	ExpectedVersion   string
	ReasonRequired    bool
	Cutoff            SameDayCutoff
	Payload           json.RawMessage
	Date              calendar.Date
	PickupTime        time.Time
	DepartureMode     string
	PickupPerson      *PickupPerson
	// Signature replaces the stored one; nil leaves an edit unsigned.
	Signature *SignedStatement
	Reason    string
}

type Edits interface {
	EditRequest(context.Context, EditInput) (*Request, error)
}
