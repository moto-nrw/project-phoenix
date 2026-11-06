// Package ports holds the persistence port of the Betreuungsvertrag and
// the values it carries (ADR 0153).
package ports

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// AmendableContract is an active contract with its timeline (#4681): every
// effective version by effective date, the newest of them, the start of the
// first, which a fixed term counts from, and the number its next version
// gets.
type AmendableContract struct {
	ID         int64
	Key        ContractKey
	FirstFrom  calendar.Date
	NextNumber int
	Latest     carecontract.Version
	Versions   []carecontract.Version
}

// ErrVersionMoved reports that the version a new version should follow is
// no longer the newest effective one: another run changed the contract.
var ErrVersionMoved = errors.New("care contract: the newest version changed")

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
	AcceptedAt  *time.Time
	Acceptances []carecontract.AcceptedText
	Key         ContractKey
	StudentID   *int64
	Version     carecontract.Version
	Party       NewParty
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
	// Concluded is how and when the contract was concluded; nil before. The
	// conclusion at acceptance sets it before the PDF is rendered, so the
	// frozen PDF and the mail say that the contract is concluded.
	Concluded *carecontract.Concluded
	// SentDocument is the newest stored PDF of the contract with its version
	// (#4681). Once a later version holds, the current version has no PDF of
	// its own; a send again or an address correction delivers this one.
	SentDocument *SentDocument
}

// SentDocument is a stored PDF of a contract and the version it shows.
type SentDocument struct {
	ID        int64
	VersionID int64
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
	ActionConclude = "conclude"
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
	// MarkConcluded moves a sent contract with its version beantragt to
	// aktiv and the version to wirksam, with the method and the instant;
	// any other state is ErrContractNotConcludable.
	MarkConcluded(ctx context.Context, organizationID, contractID, versionID int64, conclusion carecontract.Concluded) error
	InsertEvents(ctx context.Context, organizationID int64, events []Event) error
	// Versions reads every version of a contract, oldest first, with the
	// last day each effective version holds.
	Versions(ctx context.Context, organizationID, contractID int64) ([]carecontract.Version, error)
	// AmendableContracts reads every active contract with its newest
	// effective version.
	AmendableContracts(ctx context.Context, organizationID int64) ([]AmendableContract, error)
	// InsertVersion stores version. With a previousID it is the newest: the
	// version previousID becomes abgeloest, and ErrVersionMoved when it is no
	// longer wirksam. Without one (0) it goes before a planned version and
	// supersedes nothing.
	InsertVersion(ctx context.Context, organizationID, contractID, previousID int64, version carecontract.Version) (int64, error)
	// VersionsInForce reads the effective versions holding between from and
	// to; VersionPeriod.From and Until stay for the caller to fill.
	VersionsInForce(ctx context.Context, organizationID int64, from, to calendar.Date, contractIDs []int64) ([]carecontract.VersionPeriod, error)
	// VersionDocument reads the stored PDF of a version; ok is false before
	// its first send.
	VersionDocument(ctx context.Context, organizationID, versionID int64) (carecontract.File, bool, error)
	InboxContracts(ctx context.Context, organizationID int64, reader carecontract.InboxReader) ([]carecontract.InboxContract, error)
	InboxDocument(ctx context.Context, organizationID, contractID int64, reader carecontract.InboxReader) (carecontract.File, error)
}
