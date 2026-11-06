// Package ports holds the persistence port of the Betreuungsvertrag and
// the values it carries (ADR 0153).
package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
)

// TemplateRow is a stored template with the offering it belongs to.
type TemplateRow struct {
	SchoolID   int64
	OfferingID int64
	Template   carecontract.Template
}

// ContractKey is the key of a contract: the application child at one
// school (ADR 0088 section 6).
type ContractKey struct {
	SchoolID       int64
	RequestChildID int64
}

// NewContract is a draft with its first version and its party.
type NewContract struct {
	Key       ContractKey
	StudentID *int64
	Version   carecontract.Version
	Party     NewParty
}

// NewParty is the party of a new draft.
type NewParty struct {
	FirstName string
	LastName  string
	Email     string
	AccountID *int64
}

// ContractRecord is a stored contract with its current version, its parties
// and its stored document.
type ContractRecord struct {
	Summary    carecontract.ContractSummary
	Version    carecontract.Version
	Parties    []carecontract.Party
	DocumentID *int64
}

// StoredMailing is a send of the contract as stored.
type StoredMailing struct {
	OutboxID  int64
	Recipient string
	SentAt    time.Time
}

// Event is one entry of the trail.
type Event struct {
	ContractID *int64
	SchoolID   *int64
	AccountID  int64
	Action     string
	Details    map[string]any
}

// The actions of the trail.
const (
	ActionRead     = "read"
	ActionCreate   = "create"
	ActionSend     = "send"
	ActionDiscard  = "discard"
	ActionTemplate = "template"
)

// Store persists the contracts of a Träger in its Träger transaction.
type Store interface {
	Templates(ctx context.Context, organizationID int64) ([]TemplateRow, error)
	UpsertTemplate(ctx context.Context, organizationID, accountID, schoolID, offeringID int64, input carecontract.TemplateInput) (carecontract.Template, error)
	ContractKeys(ctx context.Context, organizationID int64, schoolIDs []int64) (map[ContractKey]bool, error)
	InsertContract(ctx context.Context, organizationID, accountID int64, contract NewContract) (int64, error)
	ListContracts(ctx context.Context, organizationID int64, filter carecontract.ContractFilter) ([]carecontract.ContractSummary, error)
	CountContracts(ctx context.Context, organizationID int64) (map[string]int, error)
	Contract(ctx context.Context, organizationID, contractID int64, forUpdate bool) (ContractRecord, error)
	Mailings(ctx context.Context, organizationID, contractID int64) ([]StoredMailing, error)
	InsertDocument(ctx context.Context, organizationID, accountID int64, record ContractRecord, templateVersion int, file carecontract.File) (int64, error)
	Document(ctx context.Context, organizationID, contractID int64) (carecontract.File, error)
	InsertMailing(ctx context.Context, organizationID, accountID, contractID, documentID, outboxID int64, recipient string) error
	MarkSent(ctx context.Context, organizationID, contractID int64, at time.Time) error
	DeleteDraft(ctx context.Context, organizationID, contractID int64) (bool, error)
	InsertEvents(ctx context.Context, organizationID int64, events []Event) error
	InboxContracts(ctx context.Context, organizationID int64, reader carecontract.InboxReader) ([]carecontract.InboxContract, error)
	InboxDocument(ctx context.Context, organizationID, contractID int64, reader carecontract.InboxReader) (carecontract.File, error)
}
