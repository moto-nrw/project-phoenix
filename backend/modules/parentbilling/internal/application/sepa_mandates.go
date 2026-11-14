package application

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/sharedkernel/sepa"
)

// The SEPA mandates of the families and the creditor of the Träger (#4695,
// ADR 0171). The IBAN is sealed before it reaches the store and opened only
// for the debit file; the trail of a mandate names what happened and which
// fields, never a value.

// Actions of the trail of a mandate.
const (
	mandateCreate = "create"
	mandateUpdate = "update"
	mandateRevoke = "revoke"
	mandateRead   = "read"
	mandateReveal = "reveal"
)

// mandateFromRun is the source of a read a run preview made.
const mandateFromRun = "run"

// Fields a change of a mandate names in its trail.
const (
	mandateFieldHolder   = "account_holder"
	mandateFieldIBAN     = "iban"
	mandateFieldBIC      = "bic"
	mandateFieldSequence = "sequence"
)

// MandateStore persists the creditor, the mandates and their trail. Every
// method runs in the Träger transaction of organizationID.
type MandateStore interface {
	// SEPACreditor reads the creditor of the Träger, nil without one.
	SEPACreditor(ctx context.Context, organizationID int64) (*parentbilling.SEPACreditor, error)
	SaveSEPACreditor(ctx context.Context, organizationID, accountID int64, creditor parentbilling.SEPACreditor) error
	// SEPAMandates reads the mandates of a family (familyEmail) or of every
	// family (empty), newest first.
	SEPAMandates(ctx context.Context, organizationID int64, familyEmail string) ([]parentbilling.SEPAMandate, error)
	// SEPAMandate reads one mandate, locked for a change when lock is set.
	SEPAMandate(ctx context.Context, organizationID, id int64, lock bool) (parentbilling.SEPAMandate, error)
	// InsertSEPAMandate stores a new mandate; the reference and the mandate
	// in force of the family are unique.
	InsertSEPAMandate(ctx context.Context, organizationID, accountID int64, mandate parentbilling.SealedSEPAMandate) (parentbilling.SEPAMandate, error)
	// UpdateSEPAMandate stores the account of a mandate; a nil IBAN or holder keeps
	// the sealed one.
	UpdateSEPAMandate(ctx context.Context, organizationID, accountID int64, mandate parentbilling.SealedSEPAMandate) (parentbilling.SEPAMandate, error)
	RevokeSEPAMandate(ctx context.Context, organizationID, accountID, id int64, revokedOn calendar.Date) (parentbilling.SEPAMandate, error)
	// InsertSEPAMandateEvents records one action for each mandate in one
	// statement.
	InsertSEPAMandateEvents(ctx context.Context, organizationID, accountID int64, action, source string, fields []string,
		mandates []parentbilling.SEPAMandate) error
	SEPAMandateEvents(ctx context.Context, organizationID, id int64) ([]parentbilling.SEPAMandateEvent, error)
	// SealedSEPAMandates reads mandates with their sealed IBAN.
	SealedSEPAMandates(ctx context.Context, organizationID int64, ids []int64) ([]parentbilling.SealedSEPAMandate, error)
	// SEPADebitsOfMandate reads the debits of a mandate, newest first,
	// without the account (#4697).
	SEPADebitsOfMandate(ctx context.Context, organizationID, mandateID int64) ([]parentbilling.SealedSEPADebit, error)
	// MandateImportStore records the imports of existing mandates (#6198).
	MandateImportStore
}

// SEPAMandates serves parentbilling.SEPAMandates. The family of a contract
// comes from the Betreuungsvertrag through the root-bound port; the cipher
// is the field cipher the root binds.
type SEPAMandates struct {
	store     MandateStore
	contracts parentbilling.CareContracts
	cipher    parentbilling.BankDataCipher
	formats   parentbilling.DebtorNumberFormats
	today     func() calendar.Date
}

var _ parentbilling.SEPAMandates = (*SEPAMandates)(nil)

// NewSEPAMandates builds the mandates; their import checks a debtor number
// in the format of the Träger (#6675).
func NewSEPAMandates(store MandateStore, contracts parentbilling.CareContracts, cipher parentbilling.BankDataCipher,
	formats parentbilling.DebtorNumberFormats, today func() calendar.Date,
) *SEPAMandates {
	if store == nil || contracts == nil || cipher == nil || formats == nil || today == nil {
		panic("parent billing: the store, the care contracts, the cipher, the debtor number format and the clock are required")
	}
	return &SEPAMandates{store: store, contracts: contracts, cipher: cipher, formats: formats, today: today}
}

// ibanContext binds a sealed IBAN to its Träger and mandate reference, so a
// value copied into another row does not open.
func ibanContext(organizationID int64, reference string) []byte {
	return []byte("parent_billing.sepa_mandates.iban|" + strconv.FormatInt(organizationID, 10) + "|" + reference)
}

// holderContext binds a sealed account holder like ibanContext.
func holderContext(organizationID int64, reference string) []byte {
	return []byte("parent_billing.sepa_mandates.account_holder|" + strconv.FormatInt(organizationID, 10) + "|" + reference)
}

// openHolders opens the sealed account holder of each mandate.
func (m *SEPAMandates) openHolders(organizationID int64, mandates []parentbilling.SEPAMandate) error {
	for i := range mandates {
		if mandates[i].HolderSealed == nil {
			continue
		}
		holder, err := m.cipher.Open(mandates[i].HolderSealed, holderContext(organizationID, mandates[i].Reference))
		if err != nil {
			return fmt.Errorf("parent billing: open the account holder of mandate %d: %w", mandates[i].ID, err)
		}
		mandates[i].AccountHolder, mandates[i].HolderSealed = string(holder), nil
	}
	return nil
}

// SEPACreditor answers the creditor of the Träger, the defaults without one.
func (m *SEPAMandates) SEPACreditor(ctx context.Context, organizationID int64) (parentbilling.SEPACreditor, error) {
	if organizationID <= 0 {
		return parentbilling.SEPACreditor{}, errTraegerRequired
	}
	return creditorOf(ctx, m.store, organizationID)
}

func creditorOf(ctx context.Context, store MandateStore, organizationID int64) (parentbilling.SEPACreditor, error) {
	stored, err := store.SEPACreditor(ctx, organizationID)
	if err != nil {
		return parentbilling.SEPACreditor{}, err
	}
	if stored == nil {
		return parentbilling.DefaultSEPACreditor(), nil
	}
	return *stored, nil
}

// SaveSEPACreditor validates and stores the creditor.
func (m *SEPAMandates) SaveSEPACreditor(ctx context.Context, reviewer parentbilling.HolidayReviewer, creditor parentbilling.SEPACreditor) (parentbilling.SEPACreditor, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.SEPACreditor{}, err
	}
	normalized := parentbilling.SEPACreditor{CreditorID: sepa.Compact(creditor.CreditorID),
		PrenotificationDays: creditor.PrenotificationDays, MissingMandate: creditor.MissingMandate}
	errs := validation.Errors{}
	if normalized.CreditorID != "" && !sepa.ValidCreditorID(normalized.CreditorID) {
		errs["creditor_id"] = errors.New("must be a valid SEPA creditor identifier")
	}
	if normalized.PrenotificationDays < parentbilling.MinPrenotificationDays || normalized.PrenotificationDays > parentbilling.MaxPrenotificationDays {
		errs["prenotification_days"] = fmt.Errorf("must be between %d and %d", parentbilling.MinPrenotificationDays, parentbilling.MaxPrenotificationDays)
	}
	if normalized.MissingMandate != parentbilling.MissingMandateTransfer && normalized.MissingMandate != parentbilling.MissingMandateBlock {
		errs["missing_mandate"] = errors.New("must be transfer or block")
	}
	if len(errs) > 0 {
		return parentbilling.SEPACreditor{}, errs
	}
	if err := m.store.SaveSEPACreditor(ctx, reviewer.OrganizationID, reviewer.AccountID, normalized); err != nil {
		return parentbilling.SEPACreditor{}, err
	}
	return creditorOf(ctx, m.store, reviewer.OrganizationID)
}

// ListSEPAMandates lists the mandates, of the family of a contract when
// contractID is set, names their contract parties and records the read.
func (m *SEPAMandates) ListSEPAMandates(ctx context.Context, reviewer parentbilling.HolidayReviewer, contractID int64) ([]parentbilling.SEPAMandate, error) {
	if err := validReviewer(reviewer); err != nil {
		return nil, err
	}
	family := ""
	if contractID > 0 {
		key, err := m.familyOf(ctx, reviewer.OrganizationID, contractID)
		if err != nil {
			return nil, err
		}
		family = key
	}
	mandates, err := m.store.SEPAMandates(ctx, reviewer.OrganizationID, family)
	if err != nil {
		return nil, err
	}
	if err := m.openHolders(reviewer.OrganizationID, mandates); err != nil {
		return nil, err
	}
	if err := m.nameParties(ctx, reviewer.OrganizationID, mandates); err != nil {
		return nil, err
	}
	if err := m.store.InsertSEPAMandateEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, mandateRead,
		parentbilling.MandateFromOffice, nil, mandates); err != nil {
		return nil, fmt.Errorf("parent billing: record the read of the SEPA mandates: %w", err)
	}
	return mandates, nil
}

// nameParties names the contract party of each mandate in one read.
func (m *SEPAMandates) nameParties(ctx context.Context, organizationID int64, mandates []parentbilling.SEPAMandate) error {
	if len(mandates) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(mandates))
	for _, mandate := range mandates {
		if !slices.Contains(ids, mandate.ContractID) {
			ids = append(ids, mandate.ContractID)
		}
	}
	parties, err := m.contracts.Recipients(ctx, organizationID, ids)
	if err != nil {
		return fmt.Errorf("parent billing: read the contract parties: %w", err)
	}
	for i := range mandates {
		party := parties[mandates[i].ContractID]
		mandates[i].PartyFirstName, mandates[i].PartyLastName = party.FirstName, party.LastName
	}
	return nil
}

// familyOf is the family of a contract of the Träger.
func (m *SEPAMandates) familyOf(ctx context.Context, organizationID, contractID int64) (string, error) {
	if contractID <= 0 {
		return "", parentbilling.ErrFamilyUnknown
	}
	recipients, err := m.contracts.Recipients(ctx, organizationID, []int64{contractID})
	if err != nil {
		return "", fmt.Errorf("parent billing: read the contract party: %w", err)
	}
	key := parentbilling.FamilyKey(recipients[contractID].Email)
	if key == "" {
		return "", parentbilling.ErrFamilyUnknown
	}
	return key, nil
}

// CreateSEPAMandate validates the mandate, seals its IBAN and stores it for
// the family of the contract.
func (m *SEPAMandates) CreateSEPAMandate(ctx context.Context, reviewer parentbilling.HolidayReviewer, input parentbilling.SEPAMandateInput) (parentbilling.SEPAMandate, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.SEPAMandate{}, err
	}
	mandate, iban, err := m.normalizeMandate(input)
	if err != nil {
		return parentbilling.SEPAMandate{}, err
	}
	family, err := m.familyOf(ctx, reviewer.OrganizationID, input.ContractID)
	if err != nil {
		return parentbilling.SEPAMandate{}, err
	}
	mandate.FamilyEmail, mandate.ContractID = family, input.ContractID
	// A mandate is given to the Träger's creditor identifier and stays bound
	// to it; a later change of the identifier needs a new mandate (ADR 0171).
	creditor, err := creditorOf(ctx, m.store, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.SEPAMandate{}, err
	}
	if creditor.CreditorID == "" {
		return parentbilling.SEPAMandate{}, parentbilling.ErrSEPACreditorMissing
	}
	if given := sepa.Compact(input.CreditorID); given != "" && given != creditor.CreditorID {
		return parentbilling.SEPAMandate{}, validation.Errors{"creditor_id": errors.New("must be the creditor identifier of the Träger")}
	}
	mandate.CreditorID = creditor.CreditorID
	sealed, err := m.cipher.Seal([]byte(iban), ibanContext(reviewer.OrganizationID, mandate.Reference))
	if err != nil {
		return parentbilling.SEPAMandate{}, fmt.Errorf("parent billing: seal the IBAN: %w", err)
	}
	holder, err := m.cipher.Seal([]byte(mandate.AccountHolder), holderContext(reviewer.OrganizationID, mandate.Reference))
	if err != nil {
		return parentbilling.SEPAMandate{}, fmt.Errorf("parent billing: seal the account holder: %w", err)
	}
	stored, err := m.store.InsertSEPAMandate(ctx, reviewer.OrganizationID, reviewer.AccountID,
		parentbilling.SealedSEPAMandate{Mandate: mandate, IBAN: sealed, Holder: holder})
	if err != nil {
		return parentbilling.SEPAMandate{}, err
	}
	if err := m.record(ctx, reviewer, mandateCreate, stored.Source, nil, stored); err != nil {
		return parentbilling.SEPAMandate{}, err
	}
	return m.named(ctx, reviewer.OrganizationID, stored)
}

// named opens the account holder of a mandate a write answered, names its
// contract party and tells whether it is bound to the creditor now.
func (m *SEPAMandates) named(ctx context.Context, organizationID int64, mandate parentbilling.SEPAMandate) (parentbilling.SEPAMandate, error) {
	creditor, err := creditorOf(ctx, m.store, organizationID)
	if err != nil {
		return parentbilling.SEPAMandate{}, err
	}
	mandate.CreditorCurrent = creditor.CreditorID != "" && mandate.CreditorID == creditor.CreditorID
	list := []parentbilling.SEPAMandate{mandate}
	if err := m.openHolders(organizationID, list); err != nil {
		return parentbilling.SEPAMandate{}, err
	}
	if err := m.nameParties(ctx, organizationID, list); err != nil {
		return parentbilling.SEPAMandate{}, err
	}
	return list[0], nil
}

// normalizeMandate trims and checks a new mandate and answers it with its
// compacted IBAN.
func (m *SEPAMandates) normalizeMandate(input parentbilling.SEPAMandateInput) (parentbilling.SEPAMandate, string, error) {
	mandate := parentbilling.SEPAMandate{Reference: strings.TrimSpace(input.Reference), SignedOn: input.SignedOn,
		Kind: input.Kind, Sequence: input.Sequence, Source: input.Source}
	if mandate.Kind == "" {
		mandate.Kind = parentbilling.MandateRecurring
	}
	if mandate.Sequence == "" {
		mandate.Sequence = parentbilling.SequenceFirst
	}
	if mandate.Source == "" {
		mandate.Source = parentbilling.MandateFromOffice
	}
	errs := validation.Errors{}
	switch {
	case input.SignedOn.IsZero():
		errs["signed_on"] = errors.New("is required")
	case input.SignedOn.After(m.today()):
		errs["signed_on"] = errors.New("must not be in the future")
	}
	if mandate.Reference == "" && !input.SignedOn.IsZero() {
		// Readable for the family, unique enough that a replacement after a
		// revocation on the same day gets its own; the unique constraint
		// refuses the rare collision (ErrSEPAMandateReferenceTaken).
		mandate.Reference = fmt.Sprintf("MOTO-%d-%s-%04X", input.ContractID,
			strings.ReplaceAll(input.SignedOn.String(), "-", ""), rand.IntN(0x10000)) //nolint:gosec // a reference, no secret
	}
	if !sepa.ValidMandateReference(mandate.Reference) {
		errs["reference"] = fmt.Errorf("must have 1 to %d of the SEPA characters and no double slash", sepa.MaxMandateReferenceLength)
	}
	if mandate.Kind != parentbilling.MandateRecurring && mandate.Kind != parentbilling.MandateOneOff {
		errs["kind"] = errors.New("must be recurring or one_off")
	}
	if !slices.Contains([]string{parentbilling.MandateFromOffice, parentbilling.MandateFromImport, parentbilling.MandateFromParent}, mandate.Source) {
		errs["source"] = errors.New("must be office, import or parent")
	}
	account, iban := normalizeAccount(errs, mandate.Kind, input.AccountHolder, input.IBAN, input.BIC, mandate.Sequence, true)
	mandate.AccountHolder, mandate.BIC, mandate.Sequence, mandate.IBANMasked = account.AccountHolder, account.BIC, account.Sequence, sepa.MaskIBAN(iban)
	if len(errs) > 0 {
		return mandate, "", errs
	}
	return mandate, iban, nil
}

// normalizeAccount trims and checks the account of a mandate: the holder,
// the IBAN with its check digits (requireIBAN; an empty one keeps the
// stored IBAN), the BIC where the IBAN's country needs one and the
// sequence of the kind.
func normalizeAccount(errs validation.Errors, kind, holder, iban, bic, sequence string, requireIBAN bool) (parentbilling.SEPAMandateChange, string) {
	account := parentbilling.SEPAMandateChange{AccountHolder: strings.TrimSpace(holder), BIC: sepa.Compact(bic), Sequence: sequence}
	switch length := len([]rune(account.AccountHolder)); {
	case length == 0:
		errs["account_holder"] = errors.New("is required")
	case length > parentbilling.MaxAccountHolderLength:
		errs["account_holder"] = fmt.Errorf("must be at most %d characters", parentbilling.MaxAccountHolderLength)
	}
	compact := sepa.Compact(iban)
	switch {
	case compact == "" && !requireIBAN:
	case !sepa.ValidIBAN(compact):
		// The message never repeats the value: no bank data in errors.
		errs["iban"] = errors.New("must be a valid IBAN")
	case account.BIC == "" && sepa.NeedsBIC(compact):
		errs["bic"] = errors.New("is required for an account outside the EEA")
	}
	if account.BIC != "" && !sepa.ValidBIC(account.BIC) {
		errs["bic"] = errors.New("must be a valid BIC")
	}
	switch {
	case sequence != parentbilling.SequenceFirst && sequence != parentbilling.SequenceRecurring:
		errs["sequence"] = errors.New("must be first or recurring")
	case kind == parentbilling.MandateOneOff && sequence != parentbilling.SequenceFirst:
		errs["sequence"] = errors.New("a one-off mandate is collected once")
	}
	return account, compact
}

// ChangeSEPAMandate changes the account of a mandate in force.
func (m *SEPAMandates) ChangeSEPAMandate(ctx context.Context, reviewer parentbilling.HolidayReviewer, id int64, change parentbilling.SEPAMandateChange) (parentbilling.SEPAMandate, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.SEPAMandate{}, err
	}
	current, err := m.lockInForce(ctx, reviewer.OrganizationID, id)
	if err != nil {
		return parentbilling.SEPAMandate{}, err
	}
	errs := validation.Errors{}
	account, iban := normalizeAccount(errs, current.Kind, change.AccountHolder, change.IBAN, change.BIC, change.Sequence, false)
	if iban == "" && account.BIC == "" && sepa.NeedsBIC(current.IBANMasked) {
		errs["bic"] = errors.New("is required for an account outside the EEA")
	}
	if len(errs) > 0 {
		return parentbilling.SEPAMandate{}, errs
	}
	next, fields, err := m.changedAccount(reviewer.OrganizationID, current, account, iban)
	if err != nil {
		return parentbilling.SEPAMandate{}, err
	}
	if len(fields) == 0 {
		// Nothing changed, but the answer shows the account: a read.
		if err := m.record(ctx, reviewer, mandateRead, parentbilling.MandateFromOffice, nil, current); err != nil {
			return parentbilling.SEPAMandate{}, err
		}
		return m.named(ctx, reviewer.OrganizationID, current)
	}
	slices.Sort(fields)
	stored, err := m.store.UpdateSEPAMandate(ctx, reviewer.OrganizationID, reviewer.AccountID, next)
	if err != nil {
		return parentbilling.SEPAMandate{}, err
	}
	if err := m.record(ctx, reviewer, mandateUpdate, parentbilling.MandateFromOffice, fields, stored); err != nil {
		return parentbilling.SEPAMandate{}, err
	}
	return m.named(ctx, reviewer.OrganizationID, stored)
}

// changedAccount applies a checked change of the account to a mandate in
// force: it seals a new IBAN or account holder and names the fields that
// changed, none when nothing does.
func (m *SEPAMandates) changedAccount(organizationID int64, current parentbilling.SEPAMandate, account parentbilling.SEPAMandateChange,
	iban string,
) (parentbilling.SealedSEPAMandate, []string, error) {
	next := parentbilling.SealedSEPAMandate{Mandate: current}
	next.Mandate.AccountHolder, next.Mandate.BIC, next.Mandate.Sequence = account.AccountHolder, account.BIC, account.Sequence
	var fields []string
	var err error
	if iban != "" {
		next.Mandate.IBANMasked = sepa.MaskIBAN(iban)
		if next.IBAN, err = m.cipher.Seal([]byte(iban), ibanContext(organizationID, current.Reference)); err != nil {
			return next, nil, fmt.Errorf("parent billing: seal the IBAN: %w", err)
		}
		fields = append(fields, mandateFieldIBAN)
	}
	if account.AccountHolder != current.AccountHolder {
		if next.Holder, err = m.cipher.Seal([]byte(account.AccountHolder), holderContext(organizationID, current.Reference)); err != nil {
			return next, nil, fmt.Errorf("parent billing: seal the account holder: %w", err)
		}
		fields = append(fields, mandateFieldHolder)
	}
	if account.BIC != current.BIC {
		fields = append(fields, mandateFieldBIC)
	}
	if account.Sequence != current.Sequence {
		fields = append(fields, mandateFieldSequence)
	}
	return next, fields, nil
}

// lockInForce reads a mandate in force for a change.
func (m *SEPAMandates) lockInForce(ctx context.Context, organizationID, id int64) (parentbilling.SEPAMandate, error) {
	if id <= 0 {
		return parentbilling.SEPAMandate{}, parentbilling.ErrSEPAMandateNotFound
	}
	mandate, err := m.store.SEPAMandate(ctx, organizationID, id, true)
	if err != nil {
		return parentbilling.SEPAMandate{}, err
	}
	if !mandate.InForce() {
		return parentbilling.SEPAMandate{}, parentbilling.ErrSEPAMandateRevoked
	}
	list := []parentbilling.SEPAMandate{mandate}
	if err := m.openHolders(organizationID, list); err != nil {
		return parentbilling.SEPAMandate{}, err
	}
	return list[0], nil
}

// RevokeSEPAMandate ends a mandate in force.
func (m *SEPAMandates) RevokeSEPAMandate(ctx context.Context, reviewer parentbilling.HolidayReviewer, id int64, revokedOn calendar.Date) (parentbilling.SEPAMandate, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.SEPAMandate{}, err
	}
	current, err := m.lockInForce(ctx, reviewer.OrganizationID, id)
	if err != nil {
		return parentbilling.SEPAMandate{}, err
	}
	switch {
	case revokedOn.IsZero():
		return parentbilling.SEPAMandate{}, validation.Errors{"revoked_on": errors.New("is required")}
	case revokedOn.After(m.today()):
		return parentbilling.SEPAMandate{}, validation.Errors{"revoked_on": errors.New("must not be in the future")}
	case revokedOn.Before(current.SignedOn):
		return parentbilling.SEPAMandate{}, validation.Errors{"revoked_on": errors.New("must not be before the signing date")}
	}
	stored, err := m.store.RevokeSEPAMandate(ctx, reviewer.OrganizationID, reviewer.AccountID, id, revokedOn)
	if err != nil {
		return parentbilling.SEPAMandate{}, err
	}
	if err := m.record(ctx, reviewer, mandateRevoke, parentbilling.MandateFromOffice, nil, stored); err != nil {
		return parentbilling.SEPAMandate{}, err
	}
	return m.named(ctx, reviewer.OrganizationID, stored)
}

// SEPAMandateHistory reads the trail of a mandate of the Träger.
func (m *SEPAMandates) SEPAMandateHistory(ctx context.Context, reviewer parentbilling.HolidayReviewer, id int64) ([]parentbilling.SEPAMandateEvent, error) {
	if err := validReviewer(reviewer); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, parentbilling.ErrSEPAMandateNotFound
	}
	mandate, err := m.store.SEPAMandate(ctx, reviewer.OrganizationID, id, false)
	if err != nil {
		return nil, err
	}
	// The trail is read about a mandate's bank data: it is a read too.
	if err := m.record(ctx, reviewer, mandateRead, parentbilling.MandateFromOffice, nil, mandate); err != nil {
		return nil, err
	}
	return m.store.SEPAMandateEvents(ctx, reviewer.OrganizationID, id)
}

// SEPAMandateDebits opens the IBAN of mandates of the Träger for the debit
// file and records the reveal of each. A mandate of another Träger or none
// is ErrSEPAMandateNotFound.
func (m *SEPAMandates) SEPAMandateDebits(ctx context.Context, reviewer parentbilling.HolidayReviewer, ids []int64) ([]parentbilling.SEPAMandateDebit, error) {
	if err := validReviewer(reviewer); err != nil {
		return nil, err
	}
	sealed, err := m.store.SealedSEPAMandates(ctx, reviewer.OrganizationID, ids)
	if err != nil {
		return nil, err
	}
	if len(sealed) != len(slices.Compact(slices.Sorted(slices.Values(ids)))) {
		return nil, parentbilling.ErrSEPAMandateNotFound
	}
	debits := make([]parentbilling.SEPAMandateDebit, len(sealed))
	mandates := make([]parentbilling.SEPAMandate, len(sealed))
	for i, entry := range sealed {
		iban, err := m.cipher.Open(entry.IBAN, ibanContext(reviewer.OrganizationID, entry.Mandate.Reference))
		if err != nil {
			return nil, fmt.Errorf("parent billing: open the IBAN of mandate %d: %w", entry.Mandate.ID, err)
		}
		mandate := entry.Mandate
		holder, err := m.cipher.Open(entry.Holder, holderContext(reviewer.OrganizationID, mandate.Reference))
		if err != nil {
			return nil, fmt.Errorf("parent billing: open the account holder of mandate %d: %w", mandate.ID, err)
		}
		mandate.AccountHolder, mandate.HolderSealed = string(holder), nil
		mandates[i] = mandate
		debits[i] = parentbilling.SEPAMandateDebit{MandateID: mandate.ID, Reference: mandate.Reference, SignedOn: mandate.SignedOn,
			SequenceType: mandate.SequenceType(), AccountHolder: mandate.AccountHolder, IBAN: string(iban), BIC: mandate.BIC}
	}
	if err := m.store.InsertSEPAMandateEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, mandateReveal,
		parentbilling.MandateFromOffice, nil, mandates); err != nil {
		return nil, fmt.Errorf("parent billing: record the reveal of the SEPA mandates: %w", err)
	}
	return debits, nil
}

// SEPAMandateCollections lists the debits of a mandate of the Träger: what
// was collected, what is pending and what stays out of a file and why. It
// names no bank data.
func (m *SEPAMandates) SEPAMandateCollections(ctx context.Context, reviewer parentbilling.HolidayReviewer, id int64,
) ([]parentbilling.SEPADebit, error) {
	if err := validReviewer(reviewer); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, parentbilling.ErrSEPAMandateNotFound
	}
	if _, err := m.store.SEPAMandate(ctx, reviewer.OrganizationID, id, false); err != nil {
		return nil, err
	}
	stored, err := m.store.SEPADebitsOfMandate(ctx, reviewer.OrganizationID, id)
	if err != nil {
		return nil, err
	}
	earliest := m.today().AddDays(1)
	debits := make([]parentbilling.SEPADebit, len(stored))
	for i, entry := range stored {
		debit := entry.Debit
		if debit.Status == parentbilling.DebitPending {
			debit.AmountCents += entry.CreditedCents
			debit.CollectOn = max(debit.DueOn, earliest)
			debit.Left = pendingDebitLeft(entry, debit.AmountCents, debit.CollectOn)
		}
		debits[i] = debit
	}
	return debits, nil
}

func (m *SEPAMandates) record(ctx context.Context, reviewer parentbilling.HolidayReviewer, action, source string, fields []string,
	mandate parentbilling.SEPAMandate,
) error {
	if err := m.store.InsertSEPAMandateEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, action, source, fields,
		[]parentbilling.SEPAMandate{mandate}); err != nil {
		return fmt.Errorf("parent billing: record the SEPA mandate: %w", err)
	}
	return nil
}
