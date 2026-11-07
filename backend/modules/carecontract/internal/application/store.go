// Package application holds the use cases of the Betreuungsvertrag (ADR
// 0088, ADR 0153, #4678).
package application

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/ports"
)

// The persistence port and its values live in the ports package.
type (
	Store          = ports.Store
	TemplateRow    = ports.TemplateRow
	ContractKey    = ports.ContractKey
	NewContract    = ports.NewContract
	NewParty       = ports.NewParty
	ContractRecord = ports.ContractRecord
	StoredMailing  = ports.StoredMailing
	Event          = ports.Event
)

// The actions of the trail.
const (
	ActionRead     = ports.ActionRead
	ActionCreate   = ports.ActionCreate
	ActionSend     = ports.ActionSend
	ActionDiscard  = ports.ActionDiscard
	ActionTemplate = ports.ActionTemplate
	ActionConclude = ports.ActionConclude
)

// LetterRenderer lays out a contract letter as a PDF.
type LetterRenderer interface {
	RenderContract(letter Letter, filenameBase string) (carecontract.File, error)
}

// Clock answers the instant.
type Clock struct {
	Now func() time.Time
}

func (c Clock) now() time.Time {
	if c.Now == nil {
		return time.Now()
	}
	return c.Now()
}
