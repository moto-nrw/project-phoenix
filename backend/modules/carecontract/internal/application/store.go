// Package application holds the use cases of the Betreuungsvertrag (ADR
// 0088, ADR 0153, #4678).
package application

import (
	"time"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/modules/carecontract/internal/ports"
)

type (
	Store             = ports.Store
	TemplateRow       = ports.TemplateRow
	ContractKey       = ports.ContractKey
	NewContract       = ports.NewContract
	NewParty          = ports.NewParty
	ContractRecord    = ports.ContractRecord
	StoredMailing     = ports.StoredMailing
	Event             = ports.Event
	AmendableContract = ports.AmendableContract
	SentDocument      = ports.SentDocument
	VersionStep       = ports.VersionStep
)

const (
	ActionRead     = ports.ActionRead
	ActionCreate   = ports.ActionCreate
	ActionSend     = ports.ActionSend
	ActionDiscard  = ports.ActionDiscard
	ActionTemplate = ports.ActionTemplate
	ActionConclude = ports.ActionConclude
)

type LetterRenderer interface {
	RenderContract(letter Letter, filenameBase string) (carecontract.File, error)
}

type Clock struct {
	Now func() time.Time
}

func (c Clock) now() time.Time {
	if c.Now == nil {
		return time.Now()
	}
	return c.Now()
}
