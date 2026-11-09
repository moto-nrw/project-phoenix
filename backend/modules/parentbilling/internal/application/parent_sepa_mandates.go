package application

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/sharedkernel/sepa"
)

// The digital SEPA mandate of the parents (#4696). The parents portal
// checked the account, its relationship right and that the contract was
// sent to the account's address; this flow checks the Träger's switch and
// creditor, renders the mandate text, compares the confirmed checksum and
// stores the mandate with its proof in one Träger transaction.

// ParentMandateStore is the mandate store with the proofs of online
// mandates; every method runs in the Träger transaction of organizationID.
type ParentMandateStore interface {
	MandateStore
	// InsertSEPAMandateConsent stores the proof of an online mandate; a
	// proof is written once and never changed.
	InsertSEPAMandateConsent(ctx context.Context, organizationID int64, consent parentbilling.SEPAMandateConsent) error
	// SEPAMandateConsent reads the proof of a mandate, nil for a mandate
	// not given online.
	SEPAMandateConsent(ctx context.Context, organizationID, mandateID int64) (*parentbilling.SEPAMandateConsent, error)
}

// ParentMandates serves parentbilling.ParentSEPAMandates.
type ParentMandates struct {
	store     ParentMandateStore
	contracts parentbilling.ContractParties
	cipher    parentbilling.BankDataCipher
	terms     parentbilling.OnlineMandateSettings
	sums      parentbilling.ContentFingerprints
	today     func() calendar.Date
}

var _ parentbilling.ParentSEPAMandates = (*ParentMandates)(nil)

// NewParentMandates builds the parents' mandates.
func NewParentMandates(store ParentMandateStore, contracts parentbilling.ContractParties, cipher parentbilling.BankDataCipher,
	terms parentbilling.OnlineMandateSettings, sums parentbilling.ContentFingerprints, today func() calendar.Date,
) *ParentMandates {
	if store == nil || contracts == nil || cipher == nil || terms == nil || sums == nil || today == nil {
		panic("parent billing: the store, the care contracts, the cipher, the settings, the fingerprints and the clock are required")
	}
	return &ParentMandates{store: store, contracts: contracts, cipher: cipher, terms: terms, sums: sums, today: today}
}

// textChecksum is the lower-case hex SHA-256 of the text in UTF-8.
func (p *ParentMandates) textChecksum(text string) string { return p.sums.Content([]byte(text)) }

var errParentMandateReader = errors.New("parent billing: the mandate needs a Träger, an account, a contract and the family")

// parentMandateContext is what the Träger set and the family of the reader.
type parentMandateContext struct {
	terms    parentbilling.OnlineMandateTerms
	creditor parentbilling.SEPACreditor
	family   string
}

// ready tells whether the Träger can take a mandate: a creditor identifier
// and an address that fits the consent proof's database constraint.
func (c parentMandateContext) ready() bool {
	address := oneLine(c.terms.CreditorAddress)
	return c.creditor.CreditorID != "" && address != "" && len([]rune(address)) <= 1000
}

// open checks the reader and reads the terms; a Träger that does not take
// online mandates answers ErrOnlineMandateDisabled.
func (p *ParentMandates) open(ctx context.Context, organizationID int64, reader parentbilling.ParentMandateReader) (parentMandateContext, error) {
	family := parentbilling.FamilyKey(reader.Email)
	if organizationID <= 0 || reader.AccountID <= 0 || reader.ContractID <= 0 || family == "" {
		return parentMandateContext{}, errParentMandateReader
	}
	terms, err := p.terms.OnlineMandateTerms(ctx, organizationID)
	if err != nil {
		return parentMandateContext{}, fmt.Errorf("parent billing: read the online mandate setting: %w", err)
	}
	if !terms.Allowed {
		return parentMandateContext{}, parentbilling.ErrOnlineMandateDisabled
	}
	creditor, err := creditorOf(ctx, p.store, organizationID)
	if err != nil {
		return parentMandateContext{}, err
	}
	return parentMandateContext{terms: terms, creditor: creditor, family: family}, nil
}

// ParentSEPAMandate answers the family's mandate in force with its proof,
// or the proposal for a new one, and records the read of a mandate.
func (p *ParentMandates) ParentSEPAMandate(ctx context.Context, organizationID int64, reader parentbilling.ParentMandateReader) (parentbilling.ParentMandate, error) {
	mc, err := p.open(ctx, organizationID, reader)
	if errors.Is(err, parentbilling.ErrOnlineMandateDisabled) {
		return parentbilling.ParentMandate{Enabled: false}, nil
	}
	if err != nil {
		return parentbilling.ParentMandate{}, err
	}
	inForce, err := p.mandateInForce(ctx, organizationID, mc.family)
	if err != nil {
		return parentbilling.ParentMandate{}, err
	}
	if inForce == nil {
		return p.withoutMandate(ctx, organizationID, reader, mc)
	}
	if err := p.store.InsertSEPAMandateEvents(ctx, organizationID, reader.AccountID, mandateRead, parentbilling.MandateFromParent, nil,
		[]parentbilling.SEPAMandate{*inForce}); err != nil {
		return parentbilling.ParentMandate{}, fmt.Errorf("parent billing: record the read of the SEPA mandate: %w", err)
	}
	return p.withMandate(ctx, organizationID, reader, mc, *inForce)
}

// mandateInForce reads the family's mandate in force, nil without one.
func (p *ParentMandates) mandateInForce(ctx context.Context, organizationID int64, family string) (*parentbilling.SEPAMandate, error) {
	mandates, err := p.store.SEPAMandates(ctx, organizationID, family)
	if err != nil {
		return nil, err
	}
	for i := range mandates {
		if mandates[i].InForce() && mandates[i].FamilyEmail == family {
			return &mandates[i], nil
		}
	}
	return nil, nil
}

// withoutMandate answers the creditor and the proposal of the contract
// party for a new mandate.
func (p *ParentMandates) withoutMandate(ctx context.Context, organizationID int64, reader parentbilling.ParentMandateReader,
	mc parentMandateContext,
) (parentbilling.ParentMandate, error) {
	view := parentbilling.ParentMandate{Enabled: true, CreditorMissing: !mc.ready(), Creditor: creditorView(reader, mc)}
	recipients, err := p.contracts.Recipients(ctx, organizationID, []int64{reader.ContractID})
	if err != nil {
		return parentbilling.ParentMandate{}, fmt.Errorf("parent billing: read the contract party: %w", err)
	}
	party := recipients[reader.ContractID]
	prefill := &parentbilling.ParentMandatePrefill{AccountHolder: strings.TrimSpace(party.FirstName + " " + party.LastName)}
	if party.Address != nil {
		prefill.Street, prefill.PostalCode, prefill.City = party.Address.Street, party.Address.PostalCode, party.Address.City
	}
	view.Prefill = prefill
	return view, nil
}

func creditorView(reader parentbilling.ParentMandateReader, mc parentMandateContext) *parentbilling.ParentMandateCreditor {
	return &parentbilling.ParentMandateCreditor{Name: strings.TrimSpace(reader.CreditorName),
		Address: oneLine(mc.terms.CreditorAddress), CreditorID: mc.creditor.CreditorID}
}

// withMandate answers a mandate with its opened holder and, for a mandate
// given online, the immutable text sealed at confirmation.
func (p *ParentMandates) withMandate(ctx context.Context, organizationID int64, reader parentbilling.ParentMandateReader, mc parentMandateContext,
	mandate parentbilling.SEPAMandate,
) (parentbilling.ParentMandate, error) {
	holder, err := p.cipher.Open(mandate.HolderSealed, holderContext(organizationID, mandate.Reference))
	if err != nil {
		return parentbilling.ParentMandate{}, fmt.Errorf("parent billing: open the account holder of mandate %d: %w", mandate.ID, err)
	}
	summary := &parentbilling.ParentMandateSummary{CollectionEligible: parentbilling.MandateCollectionGap(mc.creditor, &mandate, p.today()) == "", ID: mandate.ID, Reference: mandate.Reference, SignedOn: mandate.SignedOn,
		Kind: mandate.Kind, AccountHolder: string(holder), IBANMasked: mandate.IBANMasked, BIC: mandate.BIC, Source: mandate.Source,
		CreditorID: mandate.CreditorID, RevokedOn: mandate.RevokedOn}
	consent, err := p.store.SEPAMandateConsent(ctx, organizationID, mandate.ID)
	if err != nil {
		return parentbilling.ParentMandate{}, err
	}
	if consent != nil {
		sealed, err := p.cipher.Open(consent.TextSealed, consentTextContext(organizationID, mandate.Reference))
		if err != nil {
			return parentbilling.ParentMandate{}, fmt.Errorf("parent billing: open the consent text of mandate %d: %w", mandate.ID, err)
		}
		// The full IBAN stays in the owner; the parents read the text they
		// confirmed, with its mask.
		text, _, ok := confirmedText(string(sealed))
		if !ok || p.textChecksum(text) != consent.TextSHA256 {
			return parentbilling.ParentMandate{}, errors.New("parent billing: the consent text checksum does not match")
		}
		summary.Proof = &parentbilling.ParentMandateProof{Channel: consent.Channel, Version: consent.TextVersion,
			TextSHA256: consent.TextSHA256, Text: string(text), ConfirmedAt: consent.ConfirmedAt}
	}
	return parentbilling.ParentMandate{Enabled: true, CreditorMissing: !mc.ready(), Creditor: creditorView(reader, mc), Mandate: summary}, nil
}

// parentMandateDraft is a checked input with its compact IBAN and the
// fields of its text.
type parentMandateDraft struct {
	account parentbilling.SEPAMandateChange
	iban    string
	fields  mandateTextFields
}

// draft checks the input and fills the fields of the text; reference is
// the reference the text names.
func (p *ParentMandates) draft(reader parentbilling.ParentMandateReader, mc parentMandateContext, input parentbilling.ParentMandateInput,
	reference string,
) (parentMandateDraft, error) {
	errs := validation.Errors{}
	account, iban := normalizeAccount(errs, parentbilling.MandateRecurring, input.AccountHolder, input.IBAN, input.BIC,
		parentbilling.SequenceFirst, true)
	street, postalCode, city := strings.TrimSpace(input.Street), strings.TrimSpace(input.PostalCode), strings.TrimSpace(input.City)
	requireText(errs, "street", street, parentbilling.MaxDebtorStreetLength)
	requireText(errs, "city", city, parentbilling.MaxDebtorCityLength)
	if !postalCodePattern.MatchString(postalCode) {
		errs["postal_code"] = errors.New("must be five digits")
	}
	if len(errs) > 0 {
		return parentMandateDraft{}, errs
	}
	return parentMandateDraft{account: account, iban: iban, fields: mandateTextFields{
		CreditorName: strings.TrimSpace(reader.CreditorName), CreditorAddress: oneLine(mc.terms.CreditorAddress),
		CreditorID: mc.creditor.CreditorID, Reference: reference, Holder: account.AccountHolder, Street: street,
		PostalCode: postalCode, City: city, IBANMasked: sepa.MaskIBAN(iban), IBAN: iban, BIC: account.BIC, SignedOn: p.today(),
	}}, nil
}

func requireText(errs validation.Errors, field, value string, maxLength int) {
	switch length := len([]rune(value)); {
	case length == 0:
		errs[field] = errors.New("is required")
	case length > maxLength:
		errs[field] = fmt.Errorf("must be at most %d characters", maxLength)
	}
}

// PreviewParentSEPAMandate renders the text the parents confirm, with the
// reference of the preview they read before or a new one.
func (p *ParentMandates) PreviewParentSEPAMandate(ctx context.Context, organizationID int64, reader parentbilling.ParentMandateReader,
	input parentbilling.ParentMandateInput,
) (parentbilling.ParentMandateText, error) {
	mc, err := p.open(ctx, organizationID, reader)
	if err != nil {
		return parentbilling.ParentMandateText{}, err
	}
	if !mc.ready() {
		return parentbilling.ParentMandateText{}, parentbilling.ErrSEPACreditorMissing
	}
	reference := strings.TrimSpace(input.Reference)
	if !p.ownReference(reader.ContractID, reference) {
		reference = p.newReference(reader.ContractID)
	}
	draft, err := p.draft(reader, mc, input, reference)
	if err != nil {
		return parentbilling.ParentMandateText{}, err
	}
	text := renderMandateText(draft.fields)
	checksum := p.textChecksum(text)
	token, err := p.cipher.Seal([]byte(draft.iban), accountTokenContext(organizationID, reader.AccountID, reference, checksum))
	if err != nil {
		return parentbilling.ParentMandateText{}, fmt.Errorf("parent billing: seal the account of the preview: %w", err)
	}
	return parentbilling.ParentMandateText{Version: parentbilling.MandateTextVersion, Reference: reference, Text: text,
		TextSHA256: checksum, AccountToken: base64.RawURLEncoding.EncodeToString(token)}, nil
}

// accountTokenContext binds the sealed account of a preview to the Träger,
// the account that previewed, the reference and the checksum of the text.
func accountTokenContext(organizationID, accountID int64, reference, checksum string) []byte {
	return fmt.Appendf(nil, "parent_billing.sepa_mandate_preview.iban|%d|%d|%s|%s", organizationID, accountID, reference, checksum)
}

// previewedAccount tells whether the IBAN is the one the preview sealed for
// this text: the mask alone does not tell two accounts apart.
func (p *ParentMandates) previewedAccount(organizationID, accountID int64, reference, checksum, token, iban string) bool {
	sealed, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil || len(sealed) == 0 {
		return false
	}
	previewed, err := p.cipher.Open(sealed, accountTokenContext(organizationID, accountID, reference, checksum))
	return err == nil && string(previewed) == iban
}

// newReference forms a reference like the office's (MOTO-contract-day-random).
func (p *ParentMandates) newReference(contractID int64) string {
	return fmt.Sprintf("%s%04X", p.referencePrefix(contractID), rand.IntN(0x10000)) //nolint:gosec // a reference, no secret
}

func (p *ParentMandates) referencePrefix(contractID int64) string {
	return fmt.Sprintf("MOTO-%d-%s-", contractID, strings.ReplaceAll(p.today().String(), "-", ""))
}

var referenceSuffix = regexp.MustCompile(`^[0-9A-F]{4}$`)

// ownReference tells whether a reference is one a preview of this contract
// formed today; the parents never choose a reference of their own.
func (p *ParentMandates) ownReference(contractID int64, reference string) bool {
	suffix, found := strings.CutPrefix(reference, p.referencePrefix(contractID))
	return found && referenceSuffix.MatchString(suffix)
}

// GrantParentSEPAMandate stores the mandate the parents confirmed, with its
// proof, after it rendered the text again and compared its checksum.
func (p *ParentMandates) GrantParentSEPAMandate(ctx context.Context, organizationID int64, reader parentbilling.ParentMandateReader,
	input parentbilling.ParentMandateInput,
) (parentbilling.ParentMandate, error) {
	mc, err := p.open(ctx, organizationID, reader)
	if err != nil {
		return parentbilling.ParentMandate{}, err
	}
	if !mc.ready() {
		return parentbilling.ParentMandate{}, parentbilling.ErrSEPACreditorMissing
	}
	if !input.HolderConfirmed {
		return parentbilling.ParentMandate{}, validation.Errors{"holder_confirmed": parentbilling.ErrAccountHolderNotConfirmed}
	}
	reference := strings.TrimSpace(input.Reference)
	if !p.ownReference(reader.ContractID, reference) {
		return parentbilling.ParentMandate{}, parentbilling.ErrMandateTextChanged
	}
	draft, err := p.draft(reader, mc, input, reference)
	if err != nil {
		return parentbilling.ParentMandate{}, err
	}
	// The checksum is no secret: it tells two texts apart.
	text := renderMandateText(draft.fields)
	checksum := p.textChecksum(text)
	if checksum != strings.ToLower(strings.TrimSpace(input.TextSHA256)) {
		return parentbilling.ParentMandate{}, parentbilling.ErrMandateTextChanged
	}
	// The confirmation holds only for the IBAN the parents previewed: another
	// account with the same mask renders the same text.
	if !p.previewedAccount(organizationID, reader.AccountID, reference, checksum, input.AccountToken, draft.iban) {
		return parentbilling.ParentMandate{}, parentbilling.ErrMandateTextChanged
	}
	sealedText, err := p.cipher.Seal([]byte(sealedMandateText(draft.fields)), consentTextContext(organizationID, reference))
	if err != nil {
		return parentbilling.ParentMandate{}, fmt.Errorf("parent billing: seal the consent text: %w", err)
	}
	stored, err := p.insertMandate(ctx, organizationID, reader, mc, draft)
	if err != nil {
		return parentbilling.ParentMandate{}, err
	}
	if err := p.store.InsertSEPAMandateConsent(ctx, organizationID, parentbilling.SEPAMandateConsent{
		MandateID: stored.ID, ContractID: reader.ContractID, AccountID: reader.AccountID, Channel: parentbilling.ChannelParentAccount,
		TextVersion: parentbilling.MandateTextVersion, TextSHA256: checksum, TextSealed: sealedText, HolderConfirmed: true,
		CreditorName: draft.fields.CreditorName, CreditorAddress: draft.fields.CreditorAddress,
		DebtorStreet: draft.fields.Street, DebtorPostalCode: draft.fields.PostalCode, DebtorCity: draft.fields.City,
	}); err != nil {
		return parentbilling.ParentMandate{}, err
	}
	if err := p.store.InsertSEPAMandateEvents(ctx, organizationID, reader.AccountID, mandateCreate, parentbilling.MandateFromParent, nil,
		[]parentbilling.SEPAMandate{stored}); err != nil {
		return parentbilling.ParentMandate{}, fmt.Errorf("parent billing: record the SEPA mandate: %w", err)
	}
	return p.withMandate(ctx, organizationID, reader, mc, stored)
}

// insertMandate seals the account and stores the recurring mandate of the
// family, given today to the Träger's creditor identifier.
func (p *ParentMandates) insertMandate(ctx context.Context, organizationID int64, reader parentbilling.ParentMandateReader,
	mc parentMandateContext, draft parentMandateDraft,
) (parentbilling.SEPAMandate, error) {
	reference := draft.fields.Reference
	iban, err := p.cipher.Seal([]byte(draft.iban), ibanContext(organizationID, reference))
	if err != nil {
		return parentbilling.SEPAMandate{}, fmt.Errorf("parent billing: seal the IBAN: %w", err)
	}
	holder, err := p.cipher.Seal([]byte(draft.account.AccountHolder), holderContext(organizationID, reference))
	if err != nil {
		return parentbilling.SEPAMandate{}, fmt.Errorf("parent billing: seal the account holder: %w", err)
	}
	return p.store.InsertSEPAMandate(ctx, organizationID, reader.AccountID, parentbilling.SealedSEPAMandate{
		Mandate: parentbilling.SEPAMandate{ContractID: reader.ContractID, FamilyEmail: mc.family, Reference: reference,
			SignedOn: draft.fields.SignedOn, Kind: parentbilling.MandateRecurring, Sequence: parentbilling.SequenceFirst,
			CreditorID: mc.creditor.CreditorID, IBANMasked: draft.fields.IBANMasked, BIC: draft.account.BIC,
			Source: parentbilling.MandateFromParent},
		IBAN: iban, Holder: holder,
	})
}

// RevokeParentSEPAMandate ends the family's mandate in force today: the
// next run no longer collects with it.
func (p *ParentMandates) RevokeParentSEPAMandate(ctx context.Context, organizationID int64, reader parentbilling.ParentMandateReader) (parentbilling.ParentMandate, error) {
	mc, err := p.open(ctx, organizationID, reader)
	if err != nil {
		return parentbilling.ParentMandate{}, err
	}
	inForce, err := p.mandateInForce(ctx, organizationID, mc.family)
	if err != nil {
		return parentbilling.ParentMandate{}, err
	}
	if inForce == nil {
		return parentbilling.ParentMandate{}, parentbilling.ErrSEPAMandateNotFound
	}
	if _, err := p.store.SEPAMandate(ctx, organizationID, inForce.ID, true); err != nil {
		return parentbilling.ParentMandate{}, err
	}
	today := p.today()
	if today.Before(inForce.SignedOn) {
		today = inForce.SignedOn
	}
	revoked, err := p.store.RevokeSEPAMandate(ctx, organizationID, reader.AccountID, inForce.ID, today)
	if err != nil {
		return parentbilling.ParentMandate{}, err
	}
	if err := p.store.InsertSEPAMandateEvents(ctx, organizationID, reader.AccountID, mandateRevoke, parentbilling.MandateFromParent, nil,
		[]parentbilling.SEPAMandate{revoked}); err != nil {
		return parentbilling.ParentMandate{}, fmt.Errorf("parent billing: record the SEPA mandate: %w", err)
	}
	return p.withMandate(ctx, organizationID, reader, mc, revoked)
}

// consentTextContext binds the original confirmed text to its mandate.
func consentTextContext(organizationID int64, reference string) []byte {
	return fmt.Appendf(nil, "parent_billing.sepa_mandate_consents.text|%d|%s", organizationID, reference)
}

// ParentSEPAMandateStatuses batches inbox hints for the account's contracts.
// Only the current payer gets a hint; no holder or proof is opened or audited.
func (p *ParentMandates) ParentSEPAMandateStatuses(ctx context.Context, organizationID int64, reader parentbilling.ParentMandateReader,
	contractIDs []int64,
) (map[int64]parentbilling.ParentMandateStatus, error) {
	out := make(map[int64]parentbilling.ParentMandateStatus, len(contractIDs))
	if len(contractIDs) == 0 {
		return out, nil
	}
	reader.ContractID = contractIDs[0]
	mc, err := p.open(ctx, organizationID, reader)
	if errors.Is(err, parentbilling.ErrOnlineMandateDisabled) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	parties, err := p.contracts.Recipients(ctx, organizationID, contractIDs)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read the contract parties for mandate status: %w", err)
	}
	mandate, err := p.mandateInForce(ctx, organizationID, mc.family)
	if err != nil {
		return nil, err
	}
	status := parentbilling.ParentMandateStatus{Enabled: true,
		CollectionEligible: parentbilling.MandateCollectionGap(mc.creditor, mandate, p.today()) == ""}
	for _, id := range contractIDs {
		if parentbilling.FamilyKey(parties[id].Email) == mc.family {
			out[id] = status
		}
	}
	return out, nil
}
