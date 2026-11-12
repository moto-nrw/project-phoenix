package parentbilling

import (
	"context"
	"errors"
	"time"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The digital SEPA mandate of the parents (#4696, ADR 0171 addendum
// 2026-11-09). A parent with an account gives the mandate of the family in
// the parents portal, on a care contract of the inbox, and revokes it there.
// The parents portal checks the relationship right and that the contract
// was sent to the account's address before it calls; this capability runs
// in the Träger transaction of the contract and checks the rest:
//
//   - The Träger allows the online mandate (setting
//     parent_billing.online_mandate, off by default): the form of the
//     mandate is agreed between the Träger and its bank.
//   - The Träger has a creditor identifier; the mandate is given to it.
//   - The family has no mandate in force.
//   - The parent confirms to be the account holder and confirms exactly the
//     mandate text moto showed: the text is rendered here, its SHA-256 goes
//     back with the confirmation and must match the text rendered again.
//
// The proof of the confirmation (text version and checksum, channel,
// account, time, the holder confirmation and what the text named) is
// stored beside the mandate and never changes. A revocation ends the
// mandate on the day it arrives: the next run no longer collects with it.

// ChannelParentAccount is the channel of a mandate a parent gave signed in
// to the parents portal.
const ChannelParentAccount = "parent_account"

// MandateTextVersion names the wording of the mandate text; a new wording
// gets a new version, and every proof keeps the version it was shown in.
const MandateTextVersion = "sepa-core-de-1"

// Limits of what the parents enter.
const (
	MaxDebtorStreetLength = 120
	MaxDebtorCityLength   = 80
)

var (
	// ErrOnlineMandateDisabled reports a Träger that does not take mandates
	// in the parents portal.
	ErrOnlineMandateDisabled = errors.New("the Träger does not take SEPA mandates in the parents portal")
	// ErrMandateTextChanged reports a confirmation of another text than the
	// one moto renders now: the parents have to read and confirm again.
	ErrMandateTextChanged = errors.New("the confirmed mandate text is not the current one")
	// ErrAccountHolderNotConfirmed reports a mandate without the parent's
	// confirmation to be the account holder.
	ErrAccountHolderNotConfirmed = errors.New("the parent did not confirm to be the account holder")
)

// OnlineMandateTerms is what the Träger set for the online mandate: whether
// it takes one and its postal address as the mandate text names it.
type OnlineMandateTerms struct {
	Allowed         bool
	CreditorAddress string
}

// OnlineMandateSettings reads the terms of a Träger in its Träger
// transaction. The root binds it to the Träger level of the settings.
type OnlineMandateSettings interface {
	OnlineMandateTerms(ctx context.Context, organizationID int64) (OnlineMandateTerms, error)
}

// ContentFingerprints is the SHA-256 content identity of the mandate text;
// the root binds Security Runtime's.
type ContentFingerprints interface {
	Content(content []byte) string
}

// ContractParties names the party of a contract, which proposes the account
// holder of a new mandate. CareContracts satisfies it.
type ContractParties interface {
	Recipients(ctx context.Context, organizationID int64, contractIDs []int64) (map[int64]CareRecipient, error)
}

// ParentMandateReader is the parents' account on a contract the parents
// portal checked. Email is the billing recipient of the contract, resolved
// after that check; CreditorName is the name of the Träger.
type ParentMandateReader struct {
	AccountID    int64
	Email        string
	ContractID   int64
	CreditorName string
}

// ParentMandateInput is the account the parents enter and their
// confirmation. Reference and TextSHA256 come from the preview the parents
// read; HolderConfirmed is their statement to hold the account.
type ParentMandateInput struct {
	AccountHolder   string `json:"account_holder"`
	IBAN            string `json:"iban"`
	BIC             string `json:"bic"`
	Street          string `json:"street"`
	PostalCode      string `json:"postal_code"`
	City            string `json:"city"`
	Reference       string `json:"reference"`
	TextSHA256      string `json:"text_sha256"`
	HolderConfirmed bool   `json:"holder_confirmed"`
	// AccountToken is the sealed account of the preview: the confirmation
	// holds only for the IBAN the parents previewed, not for another one
	// with the same mask.
	AccountToken string `json:"account_token"`
}

// ParentMandateText is the mandate text the parents read before they
// confirm it, with its version, checksum and the reference it names.
type ParentMandateText struct {
	Version    string `json:"version"`
	Reference  string `json:"reference"`
	Text       string `json:"text"`
	TextSHA256 string `json:"text_sha256"`
	// AccountToken seals the full IBAN of the preview, bound to the Träger,
	// the account, the reference and the checksum; only the owner opens it.
	AccountToken string `json:"account_token"`
}

// ParentMandateCreditor is the creditor the mandate text names.
type ParentMandateCreditor struct {
	Name       string `json:"name"`
	Address    string `json:"address"`
	CreditorID string `json:"creditor_id"`
}

// ParentMandatePrefill proposes the holder and address of the contract
// party, which the parents may change.
type ParentMandatePrefill struct {
	AccountHolder string `json:"account_holder"`
	Street        string `json:"street"`
	PostalCode    string `json:"postal_code"`
	City          string `json:"city"`
}

// ParentMandateProof is the proof of an online mandate as the parents read
// it: the immutable text they confirmed, and when.
type ParentMandateProof struct {
	Channel     string    `json:"channel"`
	Version     string    `json:"version"`
	TextSHA256  string    `json:"text_sha256"`
	Text        string    `json:"text"`
	ConfirmedAt time.Time `json:"confirmed_at"`
}

// ParentMandateSummary is the mandate of the family without bank data but
// the mask of its IBAN.
type ParentMandateSummary struct {
	CollectionEligible bool                `json:"collection_eligible"`
	ID                 int64               `json:"id"`
	Reference          string              `json:"reference"`
	SignedOn           calendar.Date       `json:"signed_on"`
	Kind               string              `json:"kind"`
	AccountHolder      string              `json:"account_holder"`
	IBANMasked         string              `json:"iban_masked"`
	BIC                string              `json:"bic"`
	Source             string              `json:"source"`
	CreditorID         string              `json:"creditor_id"`
	RevokedOn          *calendar.Date      `json:"revoked_on"`
	Proof              *ParentMandateProof `json:"proof"`
}

// ParentMandate is what the parents see on a contract: whether the Träger
// takes online mandates, its creditor, the family's mandate in force and
// the proposal for a new one. Enabled false answers nothing else.
type ParentMandate struct {
	Enabled         bool                   `json:"enabled"`
	CreditorMissing bool                   `json:"creditor_missing"`
	Creditor        *ParentMandateCreditor `json:"creditor"`
	Mandate         *ParentMandateSummary  `json:"mandate"`
	Prefill         *ParentMandatePrefill  `json:"prefill"`
}

// ParentMandateStatus is the inbox hint without bank data or a consent read.
type ParentMandateStatus struct {
	Enabled            bool `json:"enabled"`
	CollectionEligible bool `json:"collection_eligible"`
}

// ParentSEPAMandates serves the mandate of the family in the parents
// portal. Every operation runs in the Träger transaction of
// organizationID, which the caller opens after it checked the reader, and
// records every read and change in the trail of the mandate with the
// account and the source MandateFromParent.
type ParentSEPAMandates interface {
	// ParentSEPAMandateStatuses reads only status, without audit writes or decryption.
	// The caller supplies contract IDs already authorized for the inbox.
	ParentSEPAMandateStatuses(ctx context.Context, organizationID int64, reader ParentMandateReader, contractIDs []int64) (map[int64]ParentMandateStatus, error)
	ParentSEPAMandate(ctx context.Context, organizationID int64, reader ParentMandateReader) (ParentMandate, error)
	PreviewParentSEPAMandate(ctx context.Context, organizationID int64, reader ParentMandateReader, input ParentMandateInput) (ParentMandateText, error)
	GrantParentSEPAMandate(ctx context.Context, organizationID int64, reader ParentMandateReader, input ParentMandateInput) (ParentMandate, error)
	RevokeParentSEPAMandate(ctx context.Context, organizationID int64, reader ParentMandateReader) (ParentMandate, error)
}

// SEPAMandateConsent is the proof of a mandate the parents gave online as
// the store keeps it: never changed after it was written. It names what
// the confirmed text named and keeps that exact text sealed independently
// of later corrections to the live mandate.
type SEPAMandateConsent struct {
	MandateID        int64
	ContractID       int64
	AccountID        int64
	Channel          string
	TextVersion      string
	TextSHA256       string
	TextSealed       []byte
	HolderConfirmed  bool
	CreditorName     string
	CreditorAddress  string
	DebtorStreet     string
	DebtorPostalCode string
	DebtorCity       string
	ConfirmedAt      time.Time
}
